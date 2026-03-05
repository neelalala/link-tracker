package application

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/telegram"
	"log/slog"
	"strings"
)

type Bot struct {
	api    *telegram.BotApi
	router *Router
	logger *slog.Logger
}

func NewBot(token string, cmds []domain.Command, logger *slog.Logger) (*Bot, error) {
	api, err := telegram.NewBot(token)
	if err != nil {
		return nil, err
	}

	err = api.SetMyCommands(cmds)
	if err != nil {
		return nil, err
	}
	router := NewRouter(cmds)

	return &Bot{
		api:    api,
		router: router,
		logger: logger,
	}, nil
}

func (bot *Bot) Start() {
	bot.logger.Info("Bot started listening for updates", slog.String("context", "bot.Start"))

	for {
		updates, err := bot.api.GetUpdates()
		if err != nil {
			bot.logger.Error("Failed to get updated", slog.String("error", err.Error()), slog.String("context", "api.GetUpdates"))
			continue
		}
		for _, update := range updates {
			err := bot.handleMessage(update)
			if err != nil {
				bot.logger.Error("Failed to handle update", slog.String("error", err.Error()), slog.String("context", "bot.handleMessage"))
			}
		}
	}
}

func (bot *Bot) handleMessage(msg domain.Message) error {
	if strings.HasPrefix(msg.Text, "/") {
		ss := strings.Split(msg.Text, " ")
		resp := bot.router.Handle(ss[0], ss[1:], msg.From, msg.ChatID)
		return bot.api.SendMessage(msg.ChatID, resp)
	}
	return nil
}
