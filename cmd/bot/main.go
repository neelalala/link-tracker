package main

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"

	"log"
	"os"
)

func main() {
	token := os.Getenv("APP_TELEGRAM_TOKEN")
	if token == "" {
		log.Panic("APP_TELEGRAM_TOKEN is not set")
	}

	cmds := []domain.Command{
		{
			Name:        "start",
			Description: "What this bot can do",
			Do:          domain.HandleStart,
		},
		{
			Name:        "help",
			Description: "List all available commands",
			Do:          domain.HandleHelp,
		},
	}

	router := application.NewRouter(cmds)

	bot, err := application.NewBot(token, router)
	if err != nil {
		log.Panic(err)
	}

	bot.Start()
}
