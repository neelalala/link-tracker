package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type LinkValidator interface {
	CanHandle(url string) bool
}

type SubscriptionService struct {
	chatRepo      domain.ChatRepository
	linkRepo      domain.LinkRepository
	subRepo       domain.SubscriptionRepository
	linkValidator LinkValidator
	logger        *slog.Logger
}

func NewSubscriptionService(
	chatRepo domain.ChatRepository,
	linkRepo domain.LinkRepository,
	subRepo domain.SubscriptionRepository,
	linkValidator LinkValidator,
	logger *slog.Logger,
) *SubscriptionService {
	return &SubscriptionService{
		chatRepo:      chatRepo,
		linkRepo:      linkRepo,
		subRepo:       subRepo,
		linkValidator: linkValidator,
		logger:        logger,
	}
}

func (service *SubscriptionService) RegisterChat(ctx context.Context, chatID int64) error {
	return service.chatRepo.Create(ctx, chatID)
}

func (service *SubscriptionService) DeleteChat(ctx context.Context, chatID int64) error {
	return service.chatRepo.Delete(ctx, chatID)
}

func (service *SubscriptionService) GetTrackedLinks(ctx context.Context, chatID int64) ([]domain.TrackedLink, error) {
	_, err := service.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return nil, err
	}

	subscriptions, err := service.subRepo.GetByChatID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	trackedLinks := make([]domain.TrackedLink, len(subscriptions))
	for i, sub := range subscriptions {
		link, err := service.linkRepo.GetById(ctx, sub.LinkID)
		if err != nil {
			return nil, err
		}
		trackedLinks[i] = domain.TrackedLink{
			ID:   link.ID,
			URL:  link.URL,
			Tags: sub.Tags,
		}
	}
	return trackedLinks, nil
}

func (service *SubscriptionService) AddLink(ctx context.Context, chatID int64, url string, tags []string) (domain.TrackedLink, error) {
	_, err := service.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return domain.TrackedLink{}, err
	}

	if !service.linkValidator.CanHandle(url) {
		return domain.TrackedLink{}, fmt.Errorf("%w: %s", ErrUrlNotSupported, url)
	}

	link, err := service.linkRepo.GetByUrl(ctx, url)
	if err != nil {
		if !errors.Is(err, domain.ErrLinkNotFound) {
			return domain.TrackedLink{}, err
		}
		link, err = service.linkRepo.Save(ctx, domain.Link{
			URL:         url,
			LastUpdated: time.Now(),
		})
		if err != nil {
			return domain.TrackedLink{}, err
		}
	}
	if exists, _ := service.subRepo.Exists(ctx, chatID, link.ID); exists {
		return domain.TrackedLink{}, domain.ErrAlreadySubscribed
	}

	subscription := domain.Subscription{
		ChatID: chatID,
		LinkID: link.ID,
		Tags:   tags,
	}

	err = service.subRepo.Save(ctx, subscription)
	if err != nil {
		return domain.TrackedLink{}, err
	}

	return domain.TrackedLink{
		ID:   link.ID,
		URL:  link.URL,
		Tags: subscription.Tags,
	}, nil
}

func (service *SubscriptionService) RemoveLink(ctx context.Context, chatID int64, url string) (domain.TrackedLink, error) {
	_, err := service.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return domain.TrackedLink{}, err
	}

	link, err := service.linkRepo.GetByUrl(ctx, url)
	if err != nil {
		return domain.TrackedLink{}, err
	}

	subscription, err := service.subRepo.Delete(ctx, chatID, link.ID)
	if err != nil {
		return domain.TrackedLink{}, err
	}

	return domain.TrackedLink{
		ID:   link.ID,
		URL:  link.URL,
		Tags: subscription.Tags,
	}, nil
}
