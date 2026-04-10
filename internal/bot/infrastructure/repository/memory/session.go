package memory

import (
	"context"
	"sync"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type SessionRepository struct {
	mu       sync.RWMutex
	sessions map[int64]domain.Session
}

func NewSessionRepository() *SessionRepository {
	return &SessionRepository{
		sessions: make(map[int64]domain.Session),
	}
}

func (sessionRepo *SessionRepository) GetOrCreate(ctx context.Context, chatID int64) (domain.Session, error) {
	sessionRepo.mu.RLock()
	defer sessionRepo.mu.RUnlock()
	session, ok := sessionRepo.sessions[chatID]
	if !ok {
		session = domain.NewSession(chatID)
		sessionRepo.sessions[chatID] = session
	}
	return session, nil
}

func (sessionRepo *SessionRepository) Save(ctx context.Context, session domain.Session) error {
	sessionRepo.mu.Lock()
	defer sessionRepo.mu.Unlock()
	sessionRepo.sessions[session.ChatID] = session
	return nil
}

func (sessionRepo *SessionRepository) Delete(ctx context.Context, chatID int64) (domain.Session, error) {
	sessionRepo.mu.Lock()
	defer sessionRepo.mu.Unlock()
	session, ok := sessionRepo.sessions[chatID]
	if !ok {
		session = domain.NewSession(chatID)
	} else {
		delete(sessionRepo.sessions, chatID)
	}
	return session, nil
}
