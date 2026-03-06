package subscription

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"sync"
)

type MemoryRepository struct {
	mu    sync.RWMutex
	links map[int64]map[int64]domain.Subscription // link id -> chat id -> subscription
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		links: make(map[int64]map[int64]domain.Subscription),
	}
}

func (subRepo *MemoryRepository) Save(subscription domain.Subscription) error {
	subRepo.mu.Lock()
	defer subRepo.mu.Unlock()

	if subRepo.links[subscription.LinkID] == nil {
		subRepo.links[subscription.LinkID] = make(map[int64]domain.Subscription)
	}

	if _, ok := subRepo.links[subscription.LinkID][subscription.ChatID]; ok {
		return domain.ErrAlreadySubscribed
	}

	subRepo.links[subscription.LinkID][subscription.ChatID] = subscription
	return nil
}

func (subRepo *MemoryRepository) GetByChatId(id int64) ([]domain.Subscription, error) {
	subRepo.mu.RLock()
	defer subRepo.mu.RUnlock()
	var subscriptions []domain.Subscription
	for _, subs := range subRepo.links {
		if sub, ok := subs[id]; ok {
			subscriptions = append(subscriptions, sub)
		}
	}
	return subscriptions, nil
}

func (subRepo *MemoryRepository) GetByLinkId(id int64) ([]domain.Subscription, error) {
	subRepo.mu.RLock()
	defer subRepo.mu.RUnlock()
	var subscriptions []domain.Subscription
	link, ok := subRepo.links[id]
	if !ok {
		return nil, domain.ErrLinkNotFound
	}

	for _, sub := range link {
		subscriptions = append(subscriptions, sub)
	}
	return subscriptions, nil
}

func (subRepo *MemoryRepository) Delete(subscription domain.Subscription) (domain.Subscription, error) {
	subRepo.mu.Lock()
	defer subRepo.mu.Unlock()
	if _, ok := subRepo.links[subscription.LinkID]; !ok {
		return domain.Subscription{}, domain.ErrLinkNotFound
	}

	subs := subRepo.links[subscription.LinkID]
	if _, ok := subs[subscription.ChatID]; !ok {
		return domain.Subscription{}, domain.ErrNotSubscribed
	}
	sub := subs[subscription.ChatID]
	delete(subs, subscription.ChatID)
	if len(subs) == 0 {
		delete(subRepo.links, subscription.LinkID)
	}
	return sub, nil
}
