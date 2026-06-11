package application

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/commands"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/listener/kafka"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/listener/server/grpc"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/listener/server/http"
	intelegram "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/telegram"
	scrappergrpc "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/scrapper/grpc"
	scrapperhttp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/scrapper/http"
	outtelegram "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/telegram"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/logger"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/repository/sql"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/repository/sqlbuilder"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/resilience"
)

type UpdateListener interface {
	Start() error
	Stop(ctx context.Context) error
}

type Poller interface {
	Start(ctx context.Context)
}

type ScrapperClient interface {
	domain.Scrapper
	Close() error
}

type App struct {
	listener UpdateListener
	poller   Poller
	log      *slog.Logger

	closers []func() error
}

func (app *App) onClose(f func() error) {
	app.closers = append(app.closers, f)
}

func NewApp(configPath string, out io.Writer) (*App, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("error loading config: %v", err)
	}

	app := &App{}

	if cfg.Logger.File != "" {
		file, err := os.OpenFile(cfg.Logger.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("error opening file: %v", err)
		}
		out = file
		app.onClose(file.Close)
	}

	log := logger.NewLogger(cfg.Logger.Level, out)
	app.log = log

	tgClient, err := buildTelegramClient(cfg.Telegram, log)
	if err != nil {
		return nil, fmt.Errorf("error creating telegram client: %v", err)
	}

	notifyService := NewNotifierService(log, tgClient)

	listener, err := buildListener(cfg, notifyService, log)
	if err != nil {
		return nil, fmt.Errorf("error creating update listener: %v", err)
	}
	app.listener = listener

	scrapper, err := buildScrapperClient(cfg.ScrapperService, log)
	if err != nil {
		return nil, fmt.Errorf("error creating scrapper client: %v", err)
	}
	app.onClose(scrapper.Close)

	dbPool, err := pgxpool.New(context.Background(), cfg.Database.URL)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %v", err)
	}
	app.onClose(func() error {
		dbPool.Close()
		return nil
	})

	sessionRepo, err := buildRepos(cfg.Database, dbPool, log)
	if err != nil {
		return nil, fmt.Errorf("error creating session repository: %v", err)
	}

	cmds := buildCommands(scrapper, sessionRepo, log)

	commandService := NewCommandService(scrapper, cmds)

	dialogService := NewDialogService(scrapper, sessionRepo, log)

	poller, err := intelegram.NewPoller(commandService, dialogService, tgClient, cfg.Telegram.Resilience.Timeout, log)
	if err != nil {
		return nil, fmt.Errorf("error creating telegram poller: %v", err)
	}
	app.poller = poller

	return app, nil
}

func (app *App) Start(ctx context.Context) error {
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		app.log.Info("Starting bot listener (API server, Kafka)...")
		return app.listener.Start()
	})

	g.Go(func() error {
		app.log.Info("Starting Telegram poller...")
		app.poller.Start(gCtx)
		return nil
	})

	return g.Wait()
}

func (app *App) Shutdown(ctx context.Context) {
	app.log.Info("shutting down bot...")

	err := app.listener.Stop(ctx)
	if err != nil {
		app.log.Error("failed to stop bot", slog.String("error", err.Error()))
	}

	for i := len(app.closers) - 1; i >= 0; i-- {
		if err := app.closers[i](); err != nil {
			app.log.Error("error during cleanup", slog.String("error", err.Error()))
		}
	}

	app.log.Info("bot successfully stopped")
}

func buildTelegramClient(cfg config.TelegramConfig, log *slog.Logger) (*outtelegram.Client, error) {
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
	httpClient := resilience.NewHTTPClient("telegram-in", httpClientConfig, nil, log)
	tgClient, err := outtelegram.NewClient(cfg.ApiURL, cfg.Token, cfg.Resilience.Timeout, httpClient)
	if err != nil {
		return nil, fmt.Errorf("error creating telegram client: %v", err)
	}
	return tgClient, nil
}

func buildListener(cfg config.Config, notifier domain.LinkUpdateHandler, log *slog.Logger) (UpdateListener, error) {
	if cfg.Kafka.Enable {
		log.Info("using queue as listener")
		kafka, err := kafka.NewListener(
			cfg.Kafka.Brokers,
			cfg.Kafka.ConsumerGroup,
			cfg.Kafka.Topic,
			cfg.Kafka.DLQTopic,
			cfg.Kafka.Retries.Delay,
			cfg.Kafka.Retries.MaxDelay,
			cfg.Kafka.Retries.BackoffFactor,
			cfg.Kafka.Retries.MaxRetries,
			cfg.Kafka.SchemaRegistryURL,
			notifier,
			log,
		)
		if err != nil {
			return nil, fmt.Errorf("error creating kafka listener: %v", err)
		}
		return kafka, nil
	}
	switch cfg.Server.Protocol {
	case config.ProtocolHTTP:
		log.Info("using http server as listener")
		server := http.NewServer(cfg.Server.Port, notifier, log)
		return server, nil
	case config.ProtocolGRPC:
		log.Info("using grpc server as listener")
		server := grpc.NewServer(cfg.Server.Port, notifier, log)
		return server, nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %v", cfg.Server.Protocol)
	}
}

func buildScrapperClient(cfg config.ScrapperServiceConfig, log *slog.Logger) (ScrapperClient, error) {
	switch cfg.Protocol {
	case config.ProtocolHTTP:
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
		httpClient := resilience.NewHTTPClient("scrapper", httpClientConfig, nil, log)
		log.Info("using http scrapper client")
		scrapper := scrapperhttp.NewClient(cfg.URL, httpClient, cfg.Resilience.Timeout, log)
		return scrapper, nil
	case config.ProtocolGRPC:
		log.Info("using grpc scrapper client")
		scrapper, err := scrappergrpc.NewClient(cfg.URL, cfg.Resilience.Timeout, log)
		if err != nil {
			return nil, fmt.Errorf("error creating scrapper: %v", err)
		}
		return scrapper, nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %v", cfg.Protocol)
	}
}

func buildRepos(cfg config.DatabaseConfig, dbPool *pgxpool.Pool, log *slog.Logger) (domain.SessionRepository, error) {
	switch cfg.AccessType {
	case config.AccessTypeSQL:
		log.Info("using raw sql database access type")
		sessionRepo := sql.NewSessionRepository(dbPool)
		return sessionRepo, nil
	case config.AccessTypeBUILDER:
		log.Info("using sql builder database access type")
		sessionRepo := sqlbuilder.NewSessionRepository(dbPool)
		return sessionRepo, nil
	default:
		return nil, fmt.Errorf("unsupported database access type: %v", cfg.AccessType)
	}
}

func buildCommands(scrapper domain.Scrapper, sessionRepo domain.SessionRepository, log *slog.Logger) []domain.Command {
	helpCommand := commands.NewHelpCommand()
	startCommand := commands.NewStartCommand(scrapper, log)
	listCommand := commands.NewListCommand(scrapper, log)
	trackCommand := commands.NewTrackCommand(sessionRepo, log)
	untrackCommand := commands.NewUntrackCommand(sessionRepo, log)
	cancelCommand := commands.NewCancelCommand(sessionRepo, log)

	cmds := []domain.Command{
		helpCommand,
		startCommand,
		listCommand,
		trackCommand,
		untrackCommand,
		cancelCommand,
	}

	helpCommand.SetCommands(cmds)

	return cmds
}
