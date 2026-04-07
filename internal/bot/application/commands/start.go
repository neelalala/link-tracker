package commands

import (
	"context"
	"errors"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"log/slog"
)

const (
	startCommandName        = "start"
	startCommandDescription = "What this bot can do"

	startCommandUnexpectedError = "Something went wrong while registering your. Try again"
	startCommandMessageNewUser  = "Hi! This bot can track updates on your links, so you won't miss on news! /help for list my commands"
	startCommandMessageOldUser  = "Hi again! This bot can track updates on your links, so you won't miss on news! /help for list my commands"
)

type StartCommand struct {
	scrapper domain.Scrapper
	logger   *slog.Logger
}

func NewStartCommand(scrapper domain.Scrapper, logger *slog.Logger) *StartCommand {
	return &StartCommand{
		scrapper: scrapper,
		logger:   logger,
	}
}

func (c *StartCommand) Name() string {
	return startCommandName
}

func (c *StartCommand) Description() string {
	return startCommandDescription
}

func (c *StartCommand) Execute(ctx context.Context, msg domain.Message) (string, error) {
	err := c.scrapper.RegisterChat(ctx, msg.ChatID)
	if err != nil {
		if errors.Is(err, domain.ErrChatAlreadyRegistered) {
			return startCommandMessageOldUser, nil
		}
		c.logger.Error("error registering chat",
			slog.Int64("chat_id", msg.ChatID),
			slog.String("error", err.Error()),
		)
		return startCommandUnexpectedError, err
	}
	return startCommandMessageNewUser, nil
}
