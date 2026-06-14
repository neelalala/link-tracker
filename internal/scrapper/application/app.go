package application

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"

	"github.com/jackc/pgx/v5/pgxpool"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/resilience"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application/subscription"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	cron "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/in/scheduler"
	servergrpc "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/in/server/grpc"
	serverhttp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/in/server/http"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/cache/valkey"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/http/github"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/http/stackoverflow"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/notifier/fallback"
	notifiergrpc "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/notifier/grpc"
	notifierhttp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/notifier/http"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/notifier/kafka"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/notifier/kafka/mapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/database"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/logger"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/sql"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/sqlbuilder"
)

const (
	rawLinkSchemaPath       = "./docs/raw_link_update.avsc"
	processedLinkSchemaPath = "./docs/processed_link_update.avsc"
)

type APIServer interface {
	Start() error
	Stop(ctx context.Context) error
}

type SubscriptionService interface {
	RegisterChat(ctx context.Context, chatID int64) error
	DeleteChat(ctx context.Context, chatID int64) error
	GetTrackedLinks(ctx context.Context, chatID int64) ([]domain.TrackedLink, error)
	AddLink(ctx context.Context, chatID int64, url string, tags []string) (domain.TrackedLink, error)
	RemoveLink(ctx context.Context, chatID int64, url string) (domain.TrackedLink, error)
}

type App struct {
	scheduler *cron.Scheduler
	server    APIServer
	log       *slog.Logger

	closers []func() error
}

func (a *App) onClose(f func() error) {
	a.closers = append(a.closers, f)
}

func NewApp(ctx context.Context, cfgPath string, out io.Writer) (*App, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}

	fmt.Printf("config: %+v\n", cfg)

	app := &App{}

	if cfg.Logger.File != "" {
		file, err := os.OpenFile(cfg.Logger.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("error opening log file: %v", err)
		}
		out = file
		app.onClose(file.Close)
	}

	log := logger.NewLogger(cfg.Logger.Level, out)

	log.Info("Info messages are enabled")
	log.Debug("Debug messages are enabled")

	app.log = log

	log.Debug("running migrations")
	err = database.RunMigrationsFromFile(cfg.Database.URL, cfg.Database.MigrationsDirURL, log)
	if err != nil {
		return nil, fmt.Errorf("error running migrations: %v", err)
	}

	log.Debug("Creating db pool")
	dbPool, err := pgxpool.New(context.Background(), cfg.Database.URL)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}
	app.onClose(func() error {
		dbPool.Close()
		return nil
	})

	log.Debug("Building transactor")
	transactor, err := buildTransactor(cfg.Database, dbPool)
	if err != nil {
		return nil, fmt.Errorf("error creating transactor: %v", err)
	}

	log.Debug("Building repositories")
	chatRepo, linkRepo, subRepo, err := buildRepos(cfg, dbPool)
	if err != nil {
		return nil, fmt.Errorf("error creating repository: %v", err)
	}

	log.Debug("Building fetchers")
	fetchers := buildFetchers(cfg.Fetchers, log)

	log.Debug("Building fetcher service")
	fetcher := NewFetcherService(fetchers)

	log.Debug("Building subscription service")
	subsService, err := buildSubscriptionService(cfg, chatRepo, linkRepo, subRepo, transactor, fetcher, log, app)
	if err != nil {
		return nil, fmt.Errorf("error creating subscription service: %v", err)
	}

	log.Debug("Building API server")
	server, err := buildAPIServer(cfg, subsService, log)
	if err != nil {
		return nil, fmt.Errorf("error creating API server: %v", err)
	}
	app.server = server

	log.Debug("Building notification service")
	notifier, err := buildNotifier(ctx, cfg, dbPool, app, log)
	if err != nil {
		return nil, fmt.Errorf("error creating notifier: %v", err)
	}

	log.Debug("Creating scheduler")
	scheduler, err := cron.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating scheduler: %v", err)
	}
	app.scheduler = scheduler

	log.Debug("Building scrapper service")
	scrapperService, err := NewScrapperService(
		linkRepo,
		subRepo,
		fetcher,
		transactor,
		notifier,
		cfg.Fetchers.Batch,
		cfg.Fetchers.Concurrency,
		log,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating scrapper: %v", err)
	}

	log.Debug("Creating scheduler job")
	err = scheduler.Schedule(
		cfg.Scheduler.JobInterval,
		cfg.Scheduler.JobTimeout,
		func(jobCtx context.Context) {
			log.Info("fetch job started")
			err := scrapperService.GetUpdates(jobCtx)
			if err != nil {
				log.Error("scrapper iteration failed",
					slog.String("context", "main"),
					slog.String("error", err.Error()),
				)
			}
			log.Info("fetch job finished")
		})
	if err != nil {
		return nil, fmt.Errorf("error scheduling job: %v", err)
	}
	app.onClose(scheduler.Shutdown)

	return app, nil
}

func (a *App) Start() error {
	a.log.Debug("Starting application")
	a.scheduler.Start()
	a.log.Info("scheduler started")

	a.log.Info("starting scrapper api server...")
	if err := a.server.Start(); err != nil {
		return fmt.Errorf("api server stopped with error: %w", err)
	}

	return nil
}

func (a *App) Shutdown(ctx context.Context) {
	a.log.Info("shutting down scrapper...")

	err := a.server.Stop(ctx)
	if err != nil {
		a.log.Error("error shutting down scrapper", slog.String("error", err.Error()))
	}

	for _, closer := range slices.Backward(a.closers) {
		if err := closer(); err != nil {
			a.log.Error("error during cleanup", slog.String("error", err.Error()))
		}
	}

	a.log.Info("scrapper successfully stopped")
}

func buildNotifier(
	ctx context.Context,
	cfg config.Config,
	dbPool *pgxpool.Pool,
	app *App,
	log *slog.Logger,
) (domain.UpdateNotifier, error) {
	log.Debug("Building kafka")
	kafkaNotifier, err := buildKafka(ctx, cfg, dbPool, app, log)
	if err != nil {
		return nil, fmt.Errorf("error creating kafka notifier: %v", err)
	}
	if cfg.Kafka.Enable {
		log.Debug("Kafka is primary notifier")
		return kafkaNotifier, nil
	}

	var primary domain.UpdateNotifier
	switch cfg.BotService.Protocol {
	case config.ProtocolHTTP:
		httpClientConfig := resilience.HTTPClientConfig{
			Timeout: cfg.BotService.Resilience.Timeout,
			Retry: resilience.RetryConfig{
				Enabled:           cfg.BotService.Resilience.Retry.Enabled,
				MaxRetries:        cfg.BotService.Resilience.Retry.MaxRetries,
				Delay:             cfg.BotService.Resilience.Retry.Delay,
				Backoff:           cfg.BotService.Resilience.Retry.Backoff,
				BackoffFactor:     cfg.BotService.Resilience.Retry.BackoffFactor,
				MaxDelay:          cfg.BotService.Resilience.Retry.MaxDelay,
				RetryableStatuses: cfg.BotService.Resilience.Retry.RetryableStatuses,
			},
			Breaker: resilience.CircuitBreakerConfig{
				Enabled:              cfg.BotService.Resilience.Breaker.Enabled,
				MaxRequests:          cfg.BotService.Resilience.Breaker.MaxRequests,
				SlidingWindow:        cfg.BotService.Resilience.Breaker.SlidingWindow,
				WaitInOpenState:      cfg.BotService.Resilience.Breaker.WaitInOpenState,
				MinimumNumberOfCalls: cfg.BotService.Resilience.Breaker.MinimumNumberOfCalls,
				FailureRateThreshold: cfg.BotService.Resilience.Breaker.FailureRateThreshold,
			},
		}
		httpClient := resilience.NewHTTPClient("bot-api", httpClientConfig, nil, log)
		primary = notifierhttp.NewBot(httpClient, cfg.BotService.URL, cfg.BotService.Resilience.Timeout, log)
	case config.ProtocolGRPC:
		grpcNotifier, err := notifiergrpc.NewBot(cfg.BotService.URL, cfg.BotService.Resilience.Timeout, log)
		if err != nil {
			return nil, err
		}
		app.onClose(grpcNotifier.Close)
		primary = grpcNotifier
	default:
		return nil, fmt.Errorf("unsupported notifier protocol: %s", cfg.BotService.Protocol)
	}

	notifier := fallback.New(primary, kafkaNotifier, log)
	return notifier, nil
}

func buildKafka(
	ctx context.Context,
	cfg config.Config,
	dbPool *pgxpool.Pool,
	app *App,
	log *slog.Logger,
) (domain.UpdateNotifier, error) {
	log.Debug("Building outbox repository")
	var outRepo domain.OutboxRepository
	switch cfg.Database.AccessType {
	case config.AccessTypeSQL:
		outRepo = sql.NewOutboxRepository(dbPool)
	case config.AccessTypeBUILDER:
		outRepo = sqlbuilder.NewOutboxRepository(dbPool)
	default:
		return nil, fmt.Errorf("unsupported database access type: %s", cfg.Database.AccessType)
	}

	configs, err := buildSchemaConfigs(cfg)
	if err != nil {
		return nil, fmt.Errorf("error building schemas configs: %v", err)
	}

	log.Debug("Building kafka producer")
	producer, err := kafka.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.SchemaRegistryURL, configs, outRepo, cfg.Kafka.Workers.EventLimit, cfg.Kafka.Workers.MaxRetries, log)
	if err != nil {
		return nil, fmt.Errorf("error creating kafka producer: %v", err)
	}
	app.onClose(producer.Close)

	log.Debug("Running workers")
	runWorkers(ctx, cfg.Kafka.Workers, producer, log)

	log.Debug("Building kafka notifier")
	notifier := kafka.NewNotifier(outRepo, cfg.Kafka.Topic, log)

	return notifier, nil
}

func buildSchemaConfigs(cfg config.Config) (map[string]kafka.TopicConfig, error) {
	rawUpdateSchemaFile, err := os.Open(rawLinkSchemaPath)
	if err != nil {
		return nil, fmt.Errorf("error opening kafka raw update schema file: %v", err)
	}
	defer rawUpdateSchemaFile.Close()

	rawUpdateSchemaBytes, err := io.ReadAll(rawUpdateSchemaFile)
	if err != nil {
		return nil, fmt.Errorf("error reading kafka raw update schema file: %v", err)
	}

	processedUpdateSchemaFile, err := os.Open(processedLinkSchemaPath)
	if err != nil {
		return nil, fmt.Errorf("error opening kafka processed update schema file: %v", err)
	}
	defer rawUpdateSchemaFile.Close()

	processedUpdateSchemaBytes, err := io.ReadAll(processedUpdateSchemaFile)
	if err != nil {
		return nil, fmt.Errorf("error reading kafka processed update schema file: %v", err)
	}

	return map[string]kafka.TopicConfig{
		cfg.Kafka.Topic: {
			SchemaString: string(rawUpdateSchemaBytes),
			ParseFunc:    mapper.RawLinkUpdateToNative,
		},
		"link-updates.processed": {
			SchemaString: string(processedUpdateSchemaBytes),
			ParseFunc:    mapper.ProcessedLinkUpdateToNative,
		},
	}, nil
}

func runWorkers(ctx context.Context, cfg config.KafkaWorkerConfig, producer *kafka.Producer, log *slog.Logger) {
	for range cfg.Count {
		worker := kafka.NewWorker(ctx, producer, cfg.Interval, log)
		go worker.Start()
	}
}

func buildTransactor(cfg config.DatabaseConfig, dbPool *pgxpool.Pool) (domain.Transactor, error) {
	switch cfg.AccessType {
	case config.AccessTypeSQL:
		transactor := sql.NewTransactor(dbPool)
		return transactor, nil
	case config.AccessTypeBUILDER:
		transactor := sqlbuilder.NewTransactor(dbPool)
		return transactor, nil
	default:
		return nil, fmt.Errorf("unsupported database access type: %s", cfg.AccessType)
	}
}

func buildRepos(cfg config.Config, dbPool *pgxpool.Pool) (domain.ChatRepository, domain.LinkRepository, domain.SubscriptionRepository, error) {
	switch cfg.Database.AccessType {
	case config.AccessTypeSQL:
		chatRepo := sql.NewChatRepository(dbPool)
		linkRepo := sql.NewLinkRepository(dbPool)
		subRepo := sql.NewSubscriptionRepository(dbPool)
		return chatRepo, linkRepo, subRepo, nil
	case config.AccessTypeBUILDER:
		chatRepo := sqlbuilder.NewChatRepository(dbPool)
		linkRepo := sqlbuilder.NewLinkRepository(dbPool)
		subRepo := sqlbuilder.NewSubscriptionRepository(dbPool)
		return chatRepo, linkRepo, subRepo, nil
	default:
		return nil, nil, nil, fmt.Errorf("unsupported database access type: %s", cfg.Database.AccessType)
	}
}

func buildFetchers(cfg config.FetchersConfig, log *slog.Logger) []domain.LinkFetcher {
	httpClientConfig := resilience.HTTPClientConfig{
		Timeout: cfg.Resilience.Timeout,
		Retry: resilience.RetryConfig{
			Enabled:           cfg.Resilience.Retry.Enabled,
			MaxRetries:        cfg.Resilience.Retry.MaxRetries,
			Delay:             cfg.Resilience.Retry.Delay,
			Backoff:           cfg.Resilience.Retry.Backoff,
			BackoffFactor:     cfg.Resilience.Retry.BackoffFactor,
			MaxDelay:          cfg.Resilience.Retry.MaxDelay,
			RetryableStatuses: cfg.Resilience.Retry.RetryableStatuses,
		},
		Breaker: resilience.CircuitBreakerConfig{
			Enabled:              cfg.Resilience.Breaker.Enabled,
			MaxRequests:          cfg.Resilience.Breaker.MaxRequests,
			SlidingWindow:        cfg.Resilience.Breaker.SlidingWindow,
			WaitInOpenState:      cfg.Resilience.Breaker.WaitInOpenState,
			MinimumNumberOfCalls: cfg.Resilience.Breaker.MinimumNumberOfCalls,
			FailureRateThreshold: cfg.Resilience.Breaker.FailureRateThreshold,
		},
	}
	githubClient := github.NewClient(
		resilience.NewHTTPClient("github-fetcher", httpClientConfig, nil, log),
		github.BaseURL,
		github.BaseApiURL,
		cfg.Resilience.Timeout,
		cfg.PreviewLimit,
	)
	stackoverflowClient := stackoverflow.NewClient(
		resilience.NewHTTPClient("stackoverflow-fetcher", httpClientConfig, nil, log),
		stackoverflow.BaseURL,
		stackoverflow.BaseApiURL,
		cfg.Resilience.Timeout,
		cfg.PreviewLimit,
		cfg.StackOverflowKey,
	)

	return []domain.LinkFetcher{githubClient, stackoverflowClient}
}

func buildAPIServer(cfg config.Config, subsService SubscriptionService, log *slog.Logger) (APIServer, error) {
	switch cfg.Server.Protocol {
	case config.ProtocolHTTP:
		rateLimitConfig := resilience.RateLimitConfig{
			Enabled: cfg.Server.RateLimit.Enabled,
			RPS:     cfg.Server.RateLimit.RPS,
			Burst:   cfg.Server.RateLimit.Burst,
			TTL:     cfg.Server.RateLimit.TTL,
		}
		server := serverhttp.NewServer(cfg.Server.Port, subsService, rateLimitConfig, log)
		return server, nil
	case config.ProtocolGRPC:
		server := servergrpc.NewServer(cfg.Server.Port, subsService, log)
		return server, nil
	default:
		return nil, fmt.Errorf("unsupported server protocol: %s", cfg.Server.Protocol)
	}
}

func buildSubscriptionService(
	cfg config.Config,
	chatRepo domain.ChatRepository,
	linkRepo domain.LinkRepository,
	subRepo domain.SubscriptionRepository,
	transactor domain.Transactor,
	fetcher domain.LinkFetcher,
	log *slog.Logger,
	app *App,
) (SubscriptionService, error) {
	service := subscription.NewService(chatRepo, linkRepo, subRepo, transactor, fetcher, log)

	if !cfg.Valkey.Enabled {
		return service, nil
	}

	cache, err := valkey.New(
		cfg.Valkey.Addresses,
		cfg.Valkey.TTL,
		cfg.Valkey.KeyPrefix,
		cfg.Valkey.ClientSideCache,
	)
	if err != nil {
		return nil, err
	}

	app.onClose(cache.Close)

	return subscription.NewCachingService(service, cache), nil
}
