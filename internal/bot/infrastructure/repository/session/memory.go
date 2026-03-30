package session

import (
	"context"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"sync"
)

type MemoryRepository struct {
	mu       sync.RWMutex
	sessions map[int64]domain.Session
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		sessions: make(map[int64]domain.Session),
	}
}

func (sessionRepo *MemoryRepository) GetOrCreate(ctx context.Context, chatID int64) (domain.Session, error) {
	sessionRepo.mu.RLock()
	defer sessionRepo.mu.RUnlock()
	session, ok := sessionRepo.sessions[chatID]
	if !ok {
		session = domain.Session{
			ChatID: chatID,
			State:  domain.StateIdle,
		}
		sessionRepo.sessions[chatID] = session
	}
	return session, nil
}

func (sessionRepo *MemoryRepository) Save(ctx context.Context, session domain.Session) error {
	sessionRepo.mu.Lock()
	defer sessionRepo.mu.Unlock()
	sessionRepo.sessions[session.ChatID] = session
	return nil
}

func (sessionRepo *MemoryRepository) Delete(ctx context.Context, chatID int64) (domain.Session, error) {
	sessionRepo.mu.Lock()
	defer sessionRepo.mu.Unlock()
	session, ok := sessionRepo.sessions[chatID]
	if !ok {
		delete(sessionRepo.sessions, chatID)
	}
	return session, nil
}
