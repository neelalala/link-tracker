package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
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
		service.logger.Warn("no URL provided",
			slog.Int64("link-id", update.ID),
			slog.String("error", "no url provided in link update"),
		)
		return errors.New("no url provided")
	}
	if len(update.TgChatIDs) == 0 {
		service.logger.Warn("no telegram chat IDs provided",
			slog.Int64("link-id", update.ID),
			slog.String("error", "no telegram chat IDs provided"),
			slog.String("url", update.URL),
		)
		return errors.New("no telegram chat ids provided")
	}

	text := fmt.Sprintf("Update on %s:\n%s", update.URL, update.Description)
	if update.Preview != "" {
		text = fmt.Sprintf("%s\nPreview:\n%s", text, update.Preview)
	}

	var sendErr []error
	for _, chatID := range update.TgChatIDs {
		err := service.sender.SendMessage(ctx, chatID, text)
		if err != nil {
			service.logger.Error("failed to send notification",
				slog.String("context", "NotifyService.sender.SendMessage"),
				slog.Int64("chatID", chatID),
				slog.String("error", err.Error()),
			)
			sendErr = append(sendErr, fmt.Errorf("failed to send notification to chat=%d", chatID))
		}
	}

	return errors.Join(sendErr...)
}
