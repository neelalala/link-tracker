package application

import (
	"context"
	"io"
	"log"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/in/grpc"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/in/http"
	cronin "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/in/scheduler"
	grpcnotifier "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/grpc/notifier"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/http/github"
	httpnotifier "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/http/notifier"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/http/stackoverflow"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/database"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/logger"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/sql"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/sqlbuilder"
)

type APIServer interface {
	Start(ctx context.Context) error
}

type App struct {
	scheduler *cronin.Scheduler
	server    APIServer
	slogger   *slog.Logger
}

func NewApp(ctx context.Context, cfgPath string, out io.Writer) (*App, func()) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	if cfg.Logger.File != "" {
		file, err := os.OpenFile(cfg.Logger.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		out = file
	}

	slogger := logger.NewLogger(cfg.Logger.Level, out)

	err = database.RunMigrationsFromFile(cfg.Database.URL, cfg.Database.MigrationsDirUrl, slogger)
	if err != nil {
		slogger.Error("migration failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	dbPool, err := pgxpool.New(context.Background(), cfg.Database.URL)
	if err != nil {
		slogger.Error("unable to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	cleanup := func() { dbPool.Close() }

	var chatRepo domain.ChatRepository
	var linkRepo domain.LinkRepository
	var subRepo domain.SubscriptionRepository

	switch cfg.Database.AccessType {
	case config.AccessTypeSQL:
		chatRepo = sql.NewChatRepository(dbPool)
		linkRepo = sql.NewLinkRepository(dbPool)
		subRepo = sql.NewSubscriptionRepository(dbPool)
	case config.AccessTypeBUILDER:
		chatRepo = sqlbuilder.NewChatRepository(dbPool)
		linkRepo = sqlbuilder.NewLinkRepository(dbPool)
		subRepo = sqlbuilder.NewSubscriptionRepository(dbPool)
	}

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

	fetchers := []domain.LinkFetcher{githubClient, stackoverflowClient}
	fetcher := NewFetcherService(fetchers)

	subsService := NewSubscriptionService(chatRepo, linkRepo, subRepo, fetcher, slogger)

	var server APIServer
	switch cfg.Server.Protocol {
	case config.ProtocolHTTP:
		server = http.NewServer(cfg.Server.Port, subsService, slogger)
	case config.ProtocolGRPC:
		server = grpc.NewServer(cfg.Server.Port, subsService, slogger)
	default:
		slogger.Error("unsupported protocol", "protocol", cfg.Server.Protocol)
		os.Exit(1)
	}

	var botNotifier UpdateNotifier
	switch cfg.BotService.Protocol {
	case config.ProtocolHTTP:
		botNotifier = httpnotifier.NewBot(cfg.BotService.URL, slogger)
	case config.ProtocolGRPC:
		botNotifier, err = grpcnotifier.NewBot(cfg.BotService.URL)
		if err != nil {
			slogger.Error("error creating grpc notifier",
				slog.String("context", "main"),
				slog.String("error", err.Error()),
			)
			os.Exit(1)
		}
	default:
		slogger.Error("unsupported protocol", "protocol", cfg.BotService.Protocol)
		os.Exit(1)
	}

	scheduler, err := cronin.New(ctx)
	if err != nil {
		slogger.Error("failed to init cron",
			slog.String("context", "main"),
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	scrapperService, err := NewScrapperService(
		linkRepo,
		subRepo,
		fetcher,
		botNotifier,
		cfg.Fetchers.Batch,
		cfg.Fetchers.Concurrency,
		slogger)

	err = scheduler.Schedule(
		cfg.Scheduler.FetchInterval,
		cfg.Scheduler.FetchTimeout,
		func(jobCtx context.Context) {
			err := scrapperService.GetUpdates(jobCtx)
			if err != nil {
				slogger.Error("scrapper iteration failed",
					slog.String("context", "main"),
					slog.String("error", err.Error()),
				)
			}
		})
	if err != nil {
		slogger.Error("failed to schedule job",
			slog.String("context", "main"),
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	return &App{
		scheduler: scheduler,
		server:    server,
		slogger:   slogger,
	}, cleanup
}

func (a *App) Start(ctx context.Context) {
	a.scheduler.Start()
	a.slogger.Info("scheduler started")

	a.slogger.Info("starting scrapper api server...")
	if err := a.server.Start(ctx); err != nil {
		a.slogger.Error("api server stopped with error",
			slog.String("context", "main"),
			slog.String("error", err.Error()),
		)
	}

	a.slogger.Info("shutting down scheduler...")
	if err := a.scheduler.Shutdown(); err != nil {
		a.slogger.Error("failed to shutdown cron gracefully",
			slog.String("context", "main"),
			slog.String("error", err.Error()),
		)
	}

	a.slogger.Info("scrapper successfully stopped")
}
