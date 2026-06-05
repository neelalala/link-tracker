package subscription

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

type Service struct {
	chatRepo domain.ChatRepository
	linkRepo domain.LinkRepository
	subRepo  domain.SubscriptionRepository

	transactor domain.Transactor

	linkValidator LinkValidator
	logger        *slog.Logger
}

func NewSubscriptionService(
	chatRepo domain.ChatRepository,
	linkRepo domain.LinkRepository,
	subRepo domain.SubscriptionRepository,
	transactor domain.Transactor,
	linkValidator LinkValidator,
	logger *slog.Logger,
) *Service {
	return &Service{
		chatRepo:      chatRepo,
		linkRepo:      linkRepo,
		subRepo:       subRepo,
		transactor:    transactor,
		linkValidator: linkValidator,
		logger:        logger,
	}
}

func (service *Service) RegisterChat(ctx context.Context, chatID int64) error {
	return service.chatRepo.Create(ctx, chatID)
}

func (service *Service) DeleteChat(ctx context.Context, chatID int64) error {
	return service.chatRepo.Delete(ctx, chatID)
}

func (service *Service) GetTrackedLinks(ctx context.Context, chatID int64) ([]domain.TrackedLink, error) {
	var trackedLinks []domain.TrackedLink
	err := service.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		_, err := service.chatRepo.GetByID(ctx, chatID)
		if err != nil {
			return err
		}

		subscriptions, err := service.subRepo.GetByChatID(ctx, chatID)
		if err != nil {
			return err
		}
		trackedLinks = make([]domain.TrackedLink, len(subscriptions))
		for i, sub := range subscriptions {
			link, err := service.linkRepo.GetByID(ctx, sub.LinkID)
			if err != nil {
				return err
			}

			trackedLinks[i] = domain.TrackedLink{
				ID:   link.ID,
				URL:  link.URL,
				Tags: sub.Tags,
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return trackedLinks, nil
}

func (service *Service) AddLink(ctx context.Context, chatID int64, url string, tags []string) (domain.TrackedLink, error) {
	if !service.linkValidator.CanHandle(url) {
		return domain.TrackedLink{}, fmt.Errorf("%w: %s", domain.ErrURLNotSupported, url)
	}

	var link domain.Link
	err := service.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		_, err := service.chatRepo.GetByID(ctx, chatID)
		if err != nil {
			return err
		}

		link, err = service.linkRepo.GetByURL(ctx, url)
		if err != nil {
			if !errors.Is(err, domain.ErrLinkNotFound) {
				return err
			}
			link, err = service.linkRepo.Save(ctx, domain.Link{
				URL:         url,
				LastUpdated: time.Now(),
			})
			if err != nil {
				return err
			}
		}
		if exists, _ := service.subRepo.Exists(ctx, chatID, link.ID); exists {
			return domain.ErrAlreadySubscribed
		}

		subscription := domain.Subscription{
			ChatID: chatID,
			LinkID: link.ID,
			Tags:   tags,
		}

		err = service.subRepo.Save(ctx, subscription)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return domain.TrackedLink{}, err
	}

	return domain.TrackedLink{
		ID:   link.ID,
		URL:  link.URL,
		Tags: tags,
	}, nil
}

func (service *Service) RemoveLink(ctx context.Context, chatID int64, url string) (domain.TrackedLink, error) {
	var trackedLink domain.TrackedLink
	err := service.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		_, err := service.chatRepo.GetByID(ctx, chatID)
		if err != nil {
			return err
		}

		link, err := service.linkRepo.GetByURL(ctx, url)
		if err != nil {
			return err
		}

		subscription, err := service.subRepo.Delete(ctx, chatID, link.ID)
		if err != nil {
			return err
		}

		trackedLink.ID = link.ID
		trackedLink.URL = link.URL
		trackedLink.Tags = subscription.Tags

		return nil
	})

	if err != nil {
		return domain.TrackedLink{}, err
	}

	return trackedLink, nil
}
