package main

import (
	"context"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/http"
	telegramin "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/telegram"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/http/scrapper"
	telegramout "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/http/telegram"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/logger"
	"golang.org/x/sync/errgroup"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
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

	tgClient, err := telegramout.NewClient(cfg.TelegramToken)
	if err != nil {
		slogger.Error("Error creating telegram client", slog.String("context", "main"), slog.String("error", err.Error()))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	notifyService := application.NewNotifierService(slogger, tgClient)
	apiServer := http.NewServer(cfg.BotApiPort, notifyService, slogger)

	scrapperApi := scrapper.NewClient(cfg.ScrapperUrl)
	poller, err := telegramin.NewPoller(tgClient, scrapperApi, slogger)
	if err != nil {
		slogger.Error("Failed to create bot", slog.String("context", "main"), slog.String("error", err.Error()))
		os.Exit(1)
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		slogger.Info("Starting bot API server...")
		return apiServer.Start(gCtx)
	})

	g.Go(func() error {
		slogger.Info("Starting Telegram poller...")
		poller.Start(gCtx)
		return nil
	})

	if err := g.Wait(); err != nil {
		slogger.Error("Bot stopped with error", slog.String("error", err.Error()))
	} else {
		slogger.Info("Bot successfully stopped")
	}
}
