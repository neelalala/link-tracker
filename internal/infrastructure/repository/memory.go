package repository

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"sync"
)

type MemoryUserRepository struct {
	mu    sync.RWMutex
	users map[int64]domain.User
}

func NewMemoryUserRepository() *MemoryUserRepository {
	return &MemoryUserRepository{
		mu:    sync.RWMutex{},
		users: make(map[int64]domain.User),
	}
}

func (r *MemoryUserRepository) Create(user domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[user.UserID] = user
	return nil
}

func (r *MemoryUserRepository) GetById(userID int64) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.users[userID]
	if !exists {
		return domain.User{}, domain.ErrUserNotFound
	}
	return user, nil
}

func (r *MemoryUserRepository) Update(user domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.users[user.UserID]; !exists {
		return domain.ErrUserNotFound
	}
	r.users[user.UserID] = user
	return nil
}

func (r *MemoryUserRepository) Delete(userID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.users, userID)
	return nil
}
