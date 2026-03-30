package commands

import (
	"context"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"log/slog"
)

const (
	untrackCommandName        = "untrack"
	untrackCommandDescription = "Stop tracking your link"

	untrackCommandUnexpectedError      = "Something went wrong while getting your session"
	untrackCommandTrackingSuccessfully = "Please send the link you want to untrack. Send /cancel to abort"
)

type UntrackCommand struct {
	sessionRepo domain.SessionRepository
	logger      *slog.Logger
}

func NewUntrackCommand(sessionRepo domain.SessionRepository, logger *slog.Logger) *UntrackCommand {
	return &UntrackCommand{
		sessionRepo: sessionRepo,
		logger:      logger,
	}
}

func (c *UntrackCommand) Name() string {
	return untrackCommandName
}

func (c *UntrackCommand) Description() string {
	return untrackCommandDescription
}

func (c *UntrackCommand) Execute(ctx context.Context, msg domain.Message) (string, error) {
	session, err := c.sessionRepo.GetOrCreate(ctx, msg.ChatID)
	if err != nil {
		c.logger.Error("failed to get or create session",
			slog.String("error", err.Error()),
			slog.Int64("chat_id", msg.ChatID),
		)
		return untrackCommandUnexpectedError, err
	}

	session.Reset()
	session.State = domain.StateWaitingForURLUntrack
	err = c.sessionRepo.Save(ctx, session)
	if err != nil {
		return untrackCommandUnexpectedError, err
	}

	return untrackCommandTrackingSuccessfully, nil
}
