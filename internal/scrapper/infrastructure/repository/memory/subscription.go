package memory

import (
	"context"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"sync"
)

type SubscriptionRepository struct {
	mu    sync.RWMutex
	links map[int64]map[int64]domain.Subscription // link id -> chat id -> subscription
}

func NewSubscriptionRepository() *SubscriptionRepository {
	return &SubscriptionRepository{
		links: make(map[int64]map[int64]domain.Subscription),
	}
}

func (subRepo *SubscriptionRepository) Save(ctx context.Context, subscription domain.Subscription) error {
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

func (subRepo *SubscriptionRepository) GetByChatId(ctx context.Context, id int64) ([]domain.Subscription, error) {
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

func (subRepo *SubscriptionRepository) GetByLinkId(ctx context.Context, id int64) ([]domain.Subscription, error) {
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

func (subRepo *SubscriptionRepository) Delete(ctx context.Context, subscription domain.Subscription) (domain.Subscription, error) {
	subRepo.mu.Lock()
	defer subRepo.mu.Unlock()
	if _, ok := subRepo.links[subscription.LinkID]; !ok {
		return domain.Subscription{}, domain.ErrLinkNotFound
	}

	subscriptions := subRepo.links[subscription.LinkID]
	if _, ok := subscriptions[subscription.ChatID]; !ok {
		return domain.Subscription{}, domain.ErrNotSubscribed
	}
	sub := subscriptions[subscription.ChatID]
	delete(subscriptions, subscription.ChatID)
	if len(subscriptions) == 0 {
		delete(subRepo.links, subscription.LinkID)
	}
	return sub, nil
}
