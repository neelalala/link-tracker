package main

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/in/http"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/in/scheduler"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/http/github"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/http/notifier"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/http/stackoverflow"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/logger"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/chat"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/link"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/subscription"
	"io"
	"log"
	"log/slog"
	"os"
	"time"
)

func main() {
	cfg, err := config.Load("application.conf")
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	var out io.Writer = os.Stdout

	if cfg.LogsFile != "" {
		file, err := os.OpenFile(cfg.LogsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		out = file
	}

	slogger := logger.NewLogger(cfg.Environment, out)

	chatRepo := chat.NewMemoryRepository()
	linkRepo := link.NewMemoryRepository()
	subRepo := subscription.NewMemoryRepository()

	githubClient := github.NewClient()
	stackoverflowClient := stackoverflow.NewClient()

	fetchers := []application.LinkFetcher{githubClient, stackoverflowClient}
	fetcher := application.NewFetcherService(fetchers)

	subsService := application.NewSubscriptionService(chatRepo, linkRepo, subRepo, fetcher, slogger)

	apiServer := http.NewServer(cfg.ScrapperApiPort, subsService, slogger)

	cron, err := scheduler.NewCron()
	if err != nil {
		slogger.Error("failed to init cron", slog.String("error", err.Error()))
		os.Exit(1)
	}

	var botNotifier application.UpdateNotifier = notifier.NewBot(cfg.BotUrl)

	scrapperService := application.NewScrapperService(linkRepo, subRepo, fetcher, botNotifier, slogger)

	err = cron.Schedule(60*time.Second, func() {
		err := scrapperService.GetUpdates()
		if err != nil {
			slogger.Error("scrapper iteration failed", slog.String("error", err.Error()))
		}
	})
	if err != nil {
		slogger.Error("failed to schedule job", slog.String("error", err.Error()))
		os.Exit(1)
	}

	cron.Start()
	slogger.Info("scheduler started")

	apiServer.Start()
}
