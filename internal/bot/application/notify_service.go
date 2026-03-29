package application

import (
	"context"
	"errors"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"log/slog"
)

type MessageSender interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

type NotifierService struct {
	logger *slog.Logger
	sender MessageSender
}

func NewNotifierService(logger *slog.Logger, sender MessageSender) *NotifierService {
	return &NotifierService{
		logger: logger,
		sender: sender,
	}
}

func (service *NotifierService) HandleUpdate(ctx context.Context, update domain.LinkUpdate) error {
	if update.URL == "" {
		return errors.New("no url provided")
	}
	if len(update.TgChatIDs) == 0 {
		return errors.New("no telegram chat ids provided")
	}

	text := fmt.Sprintf("There is something new!\n\nLink: %s\nUpdate: %s", update.URL, update.Description)

	for _, chatID := range update.TgChatIDs {
		err := service.sender.SendMessage(ctx, chatID, text)
		if err != nil {
			service.logger.Error("Failed to send notification", slog.String("context", "NotifyService.sender.SendMessage"), slog.Int64("chatID", chatID), slog.String("error", err.Error()))
		}
	}

	return nil
}
