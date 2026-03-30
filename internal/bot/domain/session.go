package domain

import "context"

type SessionState string

const (
	StateIdle                 SessionState = "idle"
	StateWaitingForURLTrack                = "waiting_for_url_track"
	StateWaitingForTags                    = "waiting_for_tags"
	StateWaitingForURLUntrack              = "waiting_for_url_untrack"
)

type Session struct {
	ChatID int64
	State  SessionState
	URL    string
}

func NewSession(chatID int64) Session {
	return Session{
		ChatID: chatID,
		State:  StateIdle,
	}
}

func (s *Session) Reset() {
	s.State = StateIdle
	s.URL = ""
}

type SessionRepository interface {
	GetOrCreate(ctx context.Context, chatID int64) (Session, error)
	Save(ctx context.Context, session Session) error
	Delete(ctx context.Context, chatID int64) (Session, error)
}
