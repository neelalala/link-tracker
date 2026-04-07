package commands

import (
	"context"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"log/slog"
)

const (
	trackCommandName        = "track"
	trackCommandDescription = "Track your link"

	trackCommandUnexpectedError      = "Something went wrong while getting your session"
	trackCommandTrackingSuccessfully = "Please send the link you want to track. Send /cancel to abort"
)

type TrackCommand struct {
	sessionRepo domain.SessionRepository
	logger      *slog.Logger
}

func NewTrackCommand(sessionRepo domain.SessionRepository, logger *slog.Logger) *TrackCommand {
	return &TrackCommand{
		sessionRepo: sessionRepo,
		logger:      logger,
	}
}

func (c *TrackCommand) Name() string {
	return trackCommandName
}

func (c *TrackCommand) Description() string {
	return trackCommandDescription
}

func (c *TrackCommand) Execute(ctx context.Context, msg domain.Message) (string, error) {
	session, err := c.sessionRepo.GetOrCreate(ctx, msg.ChatID)
	if err != nil {
		return trackCommandUnexpectedError, err
	}

	session.Reset()
	session.State = domain.StateWaitingForURLTrack
	err = c.sessionRepo.Save(ctx, session)
	if err != nil {
		c.logger.Error("failed to save session",
			slog.String("error", err.Error()),
			slog.Int64("chat_id", msg.ChatID),
		)
		return trackCommandUnexpectedError, err
	}

	return trackCommandTrackingSuccessfully, nil
}
