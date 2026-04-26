package commands

import (
	"context"
	"errors"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

const (
	cancelCommandName        = "cancel"
	cancelCommandDescription = "Cancel adding link process"

	cancelCommandSessionDeleteError = "Couldn't delete your session. Try again"
	cancelCommandCanceled           = "Process canceled"
	cancelCommandNothingToCancel    = "Nothing to cancel"
)

type CancelCommand struct {
	sessionRepo domain.SessionRepository
	logger      *slog.Logger
}

func NewCancelCommand(sessionRepo domain.SessionRepository, logger *slog.Logger) *CancelCommand {
	return &CancelCommand{
		sessionRepo: sessionRepo,
		logger:      logger,
	}
}

func (c *CancelCommand) Name() string {
	return cancelCommandName
}

func (c *CancelCommand) Description() string {
	return cancelCommandDescription
}

func (c *CancelCommand) Execute(ctx context.Context, msg domain.Message) (string, error) {
	session, err := c.sessionRepo.Delete(ctx, msg.ChatID)
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			return cancelCommandNothingToCancel, nil
		}
		c.logger.Error("failed to delete session",
			slog.String("error", err.Error()),
			slog.Int64("chat_id", msg.ChatID),
		)
		return cancelCommandSessionDeleteError, err
	}
	if session.State == domain.StateIdle {
		return cancelCommandNothingToCancel, nil
	}

	return cancelCommandCanceled, nil
}
