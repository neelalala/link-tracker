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

func (repo *MemoryUserRepository) Create(user domain.User) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.users[user.UserID] = user
	return nil
}

func (repo *MemoryUserRepository) GetById(userID int64) (domain.User, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()

	user, exists := repo.users[userID]
	if !exists {
		return domain.User{}, domain.ErrUserNotFound
	}
	return user, nil
}

func (repo *MemoryUserRepository) Update(user domain.User) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	if _, exists := repo.users[user.UserID]; !exists {
		return domain.ErrUserNotFound
	}
	repo.users[user.UserID] = user
	return nil
}

func (repo *MemoryUserRepository) Delete(userID int64) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	delete(repo.users, userID)
	return nil
}
