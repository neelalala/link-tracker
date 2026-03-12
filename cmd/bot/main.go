package main

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/http"
	telegramin "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/telegram"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/http/scrapper"
	telegramout "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/http/telegram"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/logger"
	"io"
	"log"
	"log/slog"
	"os"
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

	notifyService := application.NewNotifierService(slogger, tgClient)
	apiServer := http.NewServer(cfg.BotApiPort, notifyService, slogger)

	go func() {
		err := apiServer.Start()
		if err != nil {
			slogger.Error("Error on api server", slog.String("context", "apiServer.Start"), slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	scrapperApi := scrapper.NewClient(cfg.ScrapperUrl)
	poller, err := telegramin.NewPoller(tgClient, scrapperApi, slogger)
	if err != nil {
		slogger.Error("Failed to create bot", slog.String("context", "main"), slog.String("error", err.Error()))
		os.Exit(1)
	}

	poller.Start()
}
