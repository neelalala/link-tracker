package main

import (
	"context"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/in/grpc"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/in/http"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/in/scheduler"
	grpcnotifier "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/grpc/notifier"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/http/github"
	httpnotifier "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/http/notifier"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/http/stackoverflow"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/logger"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/chat"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/link"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/subscription"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type ApiServer interface {
	Start(ctx context.Context) error
}

func main() {
	cfg, err := config.Load("application.conf")
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	scrapperCfg := cfg.ScrapperConfig

	var out io.Writer = os.Stdout

	if scrapperCfg.LogsFile != "" {
		file, err := os.OpenFile(scrapperCfg.LogsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		out = file
	}

	slogger := logger.NewLogger(scrapperCfg.LogLevel, out)

	chatRepo := chat.NewMemoryRepository()
	linkRepo := link.NewMemoryRepository()
	subRepo := subscription.NewMemoryRepository()

	githubClient := github.NewClient()
	stackoverflowClient := stackoverflow.NewClient()

	fetchers := []application.LinkFetcher{githubClient, stackoverflowClient}
	fetcher := application.NewFetcherService(fetchers)

	subsService := application.NewSubscriptionService(chatRepo, linkRepo, subRepo, fetcher, slogger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var apiServer ApiServer
	var botNotifier application.UpdateNotifier
	switch cfg.ApiProtocol {
	case config.HTTP:
		apiServer = http.NewServer(scrapperCfg.ApiPort, subsService, slogger)
		botNotifier = httpnotifier.NewBot(scrapperCfg.BotUrl)
	case config.GRPC:
		apiServer = grpc.NewServer(scrapperCfg.ApiPort, subsService, slogger)
		botNotifier, err = grpcnotifier.NewBot(scrapperCfg.BotUrl)
		if err != nil {
			slogger.Error("error creating grpc notifier",
				slog.String("context", "main"),
				slog.String("error", err.Error()),
			)
			os.Exit(1)
		}
	default:
		slogger.Error("unsupported protocol:", cfg.ApiProtocol)
		os.Exit(1)
	}

	cron, err := scheduler.NewCron(ctx)
	if err != nil {
		slogger.Error("failed to init cron",
			slog.String("context", "main"),
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	scrapperService := application.NewScrapperService(linkRepo, subRepo, fetcher, botNotifier, slogger)

	err = cron.Schedule(60*time.Second, 120*time.Second, func(jobCtx context.Context) {
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

	cron.Start()
	slogger.Info("scheduler started")

	slogger.Info("starting scrapper api server...")
	if err := apiServer.Start(ctx); err != nil {
		slogger.Error("api server stopped with error",
			slog.String("context", "main"),
			slog.String("error", err.Error()),
		)
	}

	slogger.Info("shutting down scheduler...")
	if err := cron.Shutdown(); err != nil {
		slogger.Error("failed to shutdown cron gracefully",
			slog.String("context", "main"),
			slog.String("error", err.Error()),
		)
	}

	slogger.Info("scrapper successfully stopped")
}
