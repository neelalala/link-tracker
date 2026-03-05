package main

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/logger"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/repository"
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

	userRepo := repository.NewMemoryUserRepository()
	linkRepo := repository.NewMemoryLinkRepository()

	commandService := application.NewCommandService(userRepo, linkRepo, slogger)
	cmds := commandService.GetCommands()

	bot, err := application.NewBot(cfg.TelegramToken, cmds, slogger)
	if err != nil {
		slogger.Error("Failed to create bot", slog.String("context", "main"), slog.String("error", err.Error()))
		os.Exit(1)
	}

	bot.Start()
}
