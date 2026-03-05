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

func (repo *MemoryUserRepository) Save(user domain.User) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.users[user.ID] = user
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

func (repo *MemoryUserRepository) Delete(userID int64) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	delete(repo.users, userID)
	return nil
}

type MemoryLinkRepository struct {
	mu    sync.RWMutex
	links map[int64][]domain.Link // chatID -> []link
}

func NewMemoryLinkRepository() *MemoryLinkRepository {
	return &MemoryLinkRepository{
		links: make(map[int64][]domain.Link),
	}
}

func (linkRepo *MemoryLinkRepository) Save(link domain.Link) error {
	linkRepo.mu.Lock()
	defer linkRepo.mu.Unlock()
	chatLinks := linkRepo.links[link.ChatID]

	for _, existingLink := range chatLinks {
		if existingLink.UserID == link.UserID && existingLink.URL == link.URL {
			return domain.ErrLinkAlreadyTracked
		}
	}

	linkRepo.links[link.ChatID] = append(chatLinks, link)

	return nil
}

func (linkRepo *MemoryLinkRepository) GetByUserIdChatId(userID, chatID int64) ([]domain.Link, error) {
	chatLinks, ok := linkRepo.links[chatID]
	if !ok {
		return []domain.Link{}, nil
	}
	var links []domain.Link
	for _, link := range chatLinks {
		if link.UserID == userID {
			links = append(links, link)
		}
	}
	return links, nil
}

func (linkRepo *MemoryLinkRepository) Delete(link domain.Link) error {
	linkRepo.mu.Lock()
	defer linkRepo.mu.Unlock()
	chatLinks, ok := linkRepo.links[link.ChatID]
	if !ok {
		return domain.ErrLinkNotFound
	}
	for i, chatLink := range chatLinks {
		if chatLink.UserID == link.UserID && chatLink.URL == link.URL {
			chatLinks = append(chatLinks[:i], chatLinks[i+1:]...)
			linkRepo.links[link.ChatID] = chatLinks
			return nil
		}
	}

	return domain.ErrLinkNotFound
}
