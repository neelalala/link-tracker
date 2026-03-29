package main

import (
	"context"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/grpc"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/http"
	telegramin "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/telegram"
	grpcscrapper "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/grpc/scrapper"
	httpscrapper "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/http/scrapper"
	telegramout "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/http/telegram"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/logger"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"golang.org/x/sync/errgroup"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

type ApiServer interface {
	Start(ctx context.Context) error
}

func main() {
	cfg, err := config.Load("application.conf")
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}
	botCfg := cfg.BotConfig

	var out io.Writer = os.Stdout

	if botCfg.LogsFile != "" {
		file, err := os.OpenFile(botCfg.LogsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		out = file
	}

	slogger := logger.NewLogger(botCfg.LogLevel, out)

	tgClient, err := telegramout.NewClient(botCfg.TelegramToken)
	if err != nil {
		slogger.Error("Error creating telegram client", slog.String("context", "main"), slog.String("error", err.Error()))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	notifyService := application.NewNotifierService(slogger, tgClient)
	var apiServer ApiServer
	var scrapperApi application.Scrapper
	switch cfg.ApiProtocol {
	case config.HTTP:
		apiServer = http.NewServer(botCfg.ApiPort, notifyService, slogger)
		scrapperApi = httpscrapper.NewClient(botCfg.ScrapperUrl, botCfg.ScrapperTimeout)
	case config.GRPC:
		apiServer = grpc.NewServer(botCfg.ApiPort, notifyService, slogger)
		scrapperApi, err = grpcscrapper.NewClient(botCfg.ScrapperUrl)
		if err != nil {
			slogger.Error("error creating grpc scrapper: %v", err)
			os.Exit(1)
		}
	default:
		slogger.Error("unsupported protocol:", cfg.ApiProtocol)
		os.Exit(1)
	}

	commandSerice := application.NewCommandService(scrapperApi, slogger)
	poller, err := telegramin.NewPoller(commandSerice, tgClient, slogger)
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
