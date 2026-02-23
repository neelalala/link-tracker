package main

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/logger"
	"log/slog"
	"os"
)

func main() {
	env := os.Getenv("ENV")
	logger := logger.NewLogger(env, os.Stdout)

	token := os.Getenv("APP_TELEGRAM_TOKEN")
	if token == "" {
		logger.Error("APP_TELEGRAM_TOKEN is not set", slog.String("context", "main"))
		os.Exit(1)
	}

	cmds := application.GetCommands()

	bot, err := application.NewBot(token, cmds, logger)
	if err != nil {
		logger.Error("Failed to create bot", slog.String("context", "main"), slog.String("token", token), slog.String("error", err.Error()))
		os.Exit(1)
	}

	bot.Start()
}
