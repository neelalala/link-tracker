package application

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	cron "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/in/scheduler"
	servergrpc "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/in/server/grpc"
	serverhttp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/in/server/http"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/http/github"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/http/stackoverflow"
	notifiergrpc "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/notifier/grpc"
	notifierhttp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/notifier/http"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/notifier/kafka"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/database"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/logger"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/sql"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/sqlbuilder"
)

type APIServer interface {
	Start() error
	Stop(ctx context.Context) error
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

	fmt.Printf("Loaded Config: %+v\n", cfg)

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
	app.log = log

	err = database.RunMigrationsFromFile(cfg.Database.URL, cfg.Database.MigrationsDirUrl, log)
	if err != nil {
		return nil, fmt.Errorf("error running migrations: %v", err)
	}

	dbPool, err := pgxpool.New(context.Background(), cfg.Database.URL)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}
	app.onClose(func() error {
		dbPool.Close()
		return nil
	})

	transactor, err := buildTransactor(cfg, dbPool)
	if err != nil {
		return nil, fmt.Errorf("error creating transactor: %v", err)
	}

	chatRepo, linkRepo, subRepo, err := buildRepos(cfg, dbPool)
	if err != nil {
		return nil, fmt.Errorf("error creating repository: %v", err)
	}

	fetchers := buildFetchers(cfg)
	fetcher := NewFetcherService(fetchers)

	subsService := NewSubscriptionService(chatRepo, linkRepo, subRepo, transactor, fetcher, log)

	server, err := buildAPIServer(cfg, subsService, log)
	if err != nil {
		return nil, fmt.Errorf("error creating API server: %v", err)
	}
	app.server = server

	notifier, err := buildNotifier(cfg, log)
	if err != nil {
		return nil, fmt.Errorf("error creating notifier: %v", err)
	}
	app.onClose(notifier.Close)

	scheduler, err := cron.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating scheduler: %v", err)
	}
	app.scheduler = scheduler

	scrapperService, err := NewScrapperService(
		linkRepo,
		subRepo,
		fetcher,
		transactor,
		notifier,
		cfg.Fetchers.Batch,
		cfg.Fetchers.Concurrency,
		log)

	err = scheduler.Schedule(
		cfg.Scheduler.FetchInterval,
		cfg.Scheduler.FetchTimeout,
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

	for i := len(a.closers) - 1; i >= 0; i-- {
		if err := a.closers[i](); err != nil {
			a.log.Error("error during cleanup", slog.String("error", err.Error()))
		}
	}

	a.log.Info("scrapper successfully stopped")
}

func buildNotifier(cfg *config.Config, log *slog.Logger) (UpdateNotifier, error) {
	if cfg.UseQueue {
		notifier, err := kafka.NewNotifier(cfg.Kafka.Brokers, cfg.Kafka.Topic, log)
		if err != nil {
			return nil, err
		}
		return notifier, nil
	}
	switch cfg.BotService.Protocol {
	case config.ProtocolHTTP:
		notifier := notifierhttp.NewBot(cfg.BotService.URL, log)
		return notifier, nil
	case config.ProtocolGRPC:
		notifier, err := notifiergrpc.NewBot(cfg.BotService.URL)
		if err != nil {
			return nil, err
		}
		return notifier, nil
	default:
		return nil, fmt.Errorf("unsupported notifier protocol: %s", cfg.BotService.Protocol)
	}
}

func buildTransactor(cfg *config.Config, dbPool *pgxpool.Pool) (domain.Transactor, error) {
	switch cfg.Database.AccessType {
	case config.AccessTypeSQL:
		transactor := sql.NewTransactor(dbPool)
		return transactor, nil
	case config.AccessTypeBUILDER:
		transactor := sqlbuilder.NewTransactor(dbPool)
		return transactor, nil
	default:
		return nil, fmt.Errorf("unsupported database access type: %s", cfg.Database.AccessType)
	}
}

func buildRepos(cfg *config.Config, dbPool *pgxpool.Pool) (domain.ChatRepository, domain.LinkRepository, domain.SubscriptionRepository, error) {
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

func buildFetchers(cfg *config.Config) []domain.LinkFetcher {
	githubClient := github.NewClient(
		github.BaseURL,
		github.BaseApiURL,
		cfg.Fetchers.Timeout,
		cfg.Fetchers.PreviewLimit,
	)
	stackoverflowClient := stackoverflow.NewClient(
		stackoverflow.BaseURL,
		stackoverflow.BaseApiURL,
		cfg.Fetchers.Timeout,
		cfg.Fetchers.PreviewLimit,
		cfg.Fetchers.StackOverflowKey,
	)

	return []domain.LinkFetcher{githubClient, stackoverflowClient}
}

func buildAPIServer(cfg *config.Config, subsService *SubscriptionService, log *slog.Logger) (APIServer, error) {
	switch cfg.Server.Protocol {
	case config.ProtocolHTTP:
		server := serverhttp.NewServer(cfg.Server.Port, subsService, log)
		return server, nil
	case config.ProtocolGRPC:
		server := servergrpc.NewServer(cfg.Server.Port, subsService, log)
		return server, nil
	default:
		return nil, fmt.Errorf("unsupported server protocol: %s", cfg.Server.Protocol)
	}
}
