package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"log/slog"
)

var ErrBadSessionState = fmt.Errorf("unknown session state")

type StateHandler func(context.Context, domain.Session, domain.Message) (string, error)

type DialogService struct {
	scrapper      domain.Scrapper
	sessionRepo   domain.SessionRepository
	logger        *slog.Logger
	stateRegistry map[domain.SessionState]StateHandler
}

func NewDialogService(
	scrapper domain.Scrapper,
	sessionRepo domain.SessionRepository,
	logger *slog.Logger,
) *DialogService {
	service := &DialogService{
		scrapper:    scrapper,
		sessionRepo: sessionRepo,
		logger:      logger,
	}

	service.stateRegistry = map[domain.SessionState]StateHandler{
		domain.StateIdle:                 service.handleIdle,
		domain.StateWaitingForURLTrack:   service.handleWaitingForURLTrack,
		domain.StateWaitingForTags:       service.handleWaitingForTags,
		domain.StateWaitingForURLUntrack: service.handleWaitingForURLUntrack,
	}

	return service
}

func (service *DialogService) HandleMessage(ctx context.Context, msg domain.Message) (string, error) {
	session, err := service.sessionRepo.GetOrCreate(ctx, msg.ChatID)
	if err != nil {
		service.logger.Error("failed to get session",
			slog.String("error", err.Error()),
			slog.Int64("chat_id", msg.ChatID),
		)
		return "Something went wrong while getting your session", err
	}

	handler, ok := service.stateRegistry[session.State]
	if !ok {
		session.Reset()
		_ = service.sessionRepo.Save(ctx, session)
		return "Unknown state. Process reset. Please send a command", ErrBadSessionState
	}

	return handler(ctx, session, msg)
}

func (service *DialogService) handleIdle(ctx context.Context, session domain.Session, msg domain.Message) (string, error) {
	return "I don't understand plain text right now. Please use commands from /help menu", nil
}

func (service *DialogService) handleWaitingForURLTrack(ctx context.Context, session domain.Session, msg domain.Message) (string, error) {
	url := strings.TrimSpace(msg.Text)
	if url == "" {
		return "Link cannot be empty. Please send a valid link or /cancel", nil
	}

	session.URL = url
	session.State = domain.StateWaitingForTags

	if err := service.sessionRepo.Save(ctx, session); err != nil {
		service.logger.Error("failed to save session",
			slog.String("error", err.Error()),
			slog.Int64("chat_id", msg.ChatID),
			slog.String("url", url),
			slog.Any("state", session.State),
		)
		return "Something went wrong. Please try again", err
	}

	return "Link saved! Now send tags separated by commas (e.g., work, bug). Or send 'skip' to add without tags.", nil
}

func (service *DialogService) handleWaitingForTags(ctx context.Context, session domain.Session, msg domain.Message) (string, error) {
	rawText := strings.TrimSpace(msg.Text)
	var tags []string

	if strings.ToLower(rawText) != "skip" {
		splitTags := strings.Split(rawText, ",")
		for _, tag := range splitTags {
			trimmed := strings.TrimSpace(tag)
			if trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
	}

	_, err := service.scrapper.AddLink(ctx, session.ChatID, session.URL, tags)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrAlreadySubscribed):
			return "You are already tracking this link", nil
		case errors.Is(err, domain.ErrChatNotRegistered):
			return "You are not registered yet. Use /start", nil
		case errors.Is(err, domain.ErrUrlNotSupported):
			return "This link is not supported yet", nil
		}

		service.logger.Error("failed to add link",
			slog.String("error", err.Error()),
			slog.Int64("chat_id", session.ChatID),
			slog.String("url", session.URL),
		)
		return "Failed to track link. Please try again", err
	}

	url := session.URL
	session.Reset()
	if err := service.sessionRepo.Save(ctx, session); err != nil {
		service.logger.Error("failed to reset session",
			slog.String("error", err.Error()),
			slog.Int64("chat_id", session.ChatID),
		)
		return "Something went wrong. Please try again", err
	}

	return fmt.Sprintf("Success! Now tracking link: %s", url), nil
}

func (service *DialogService) handleWaitingForURLUntrack(ctx context.Context, session domain.Session, msg domain.Message) (string, error) {
	url := strings.TrimSpace(msg.Text)
	if url == "" {
		return "Link cannot be empty. Please send a valid link or /cancel", nil
	}

	_, err := service.scrapper.RemoveLink(ctx, session.ChatID, url)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotSubscribed):
			return "You are not tracking this link", nil
		case errors.Is(err, domain.ErrChatNotRegistered):
			return "You are not registered yet. Use /start", nil
		}

		service.logger.Error("failed to remove link",
			slog.String("error", err.Error()),
			slog.Int64("chat_id", session.ChatID),
			slog.String("url", url),
		)
		return "Failed to untrack link", err
	}

	url = session.URL
	session.Reset()
	if err := service.sessionRepo.Save(ctx, session); err != nil {
		service.logger.Error("failed to save session",
			slog.String("error", err.Error()),
			slog.Int64("chat_id", session.ChatID),
		)
		return "Something went wrong. Please try again", err
	}

	return fmt.Sprintf("Link %s has been untracked", url), nil
}
