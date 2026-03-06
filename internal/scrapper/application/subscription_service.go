package application

import (
	"errors"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"log/slog"
	"time"
)

type SubscriptionService interface {
	RegisterChat(chatID int64) error
	DeleteChat(chatID int64) error

	GetTrackedLinks(chatID int64) ([]domain.TrackedLink, error)
	AddLink(chatID int64, url string, tags, filters []string) (domain.TrackedLink, error)
	RemoveLink(chatID int64, url string) (domain.TrackedLink, error)
}

type Service struct {
	chatRepo domain.ChatRepository
	linkRepo domain.LinkRepository
	subRepo  domain.SubscriptionRepository

	logger *slog.Logger
}

func NewSubscriptionService(
	chatRepo domain.ChatRepository,
	linkRepo domain.LinkRepository,
	subRepo domain.SubscriptionRepository,
	logger *slog.Logger,
) *Service {
	return &Service{
		chatRepo: chatRepo,
		linkRepo: linkRepo,
		subRepo:  subRepo,
		logger:   logger,
	}
}

func (service *Service) RegisterChat(chatID int64) error {
	chat := domain.Chat{ID: chatID}
	return service.chatRepo.Create(chat)
}

func (service *Service) DeleteChat(chatID int64) error {
	chat := domain.Chat{ID: chatID}
	return service.chatRepo.Delete(chat)
}

func (service *Service) GetTrackedLinks(chatID int64) ([]domain.TrackedLink, error) {
	_, err := service.chatRepo.GetById(chatID)
	if err != nil {
		return nil, err
	}

	subs, err := service.subRepo.GetByChatId(chatID)
	if err != nil {
		return nil, err
	}
	trackedLinks := make([]domain.TrackedLink, len(subs))
	for i, sub := range subs {
		link, err := service.linkRepo.GetById(sub.LinkID)
		if err != nil {
			return nil, err
		}
		trackedLinks[i] = domain.TrackedLink{
			ID:      link.ID,
			URL:     link.URL,
			Tags:    sub.Tags,
			Filters: sub.Filters,
		}
	}
	return trackedLinks, nil
}

func (service *Service) AddLink(chatID int64, url string, tags, filters []string) (domain.TrackedLink, error) {
	_, err := service.chatRepo.GetById(chatID)
	if err != nil {
		return domain.TrackedLink{}, err
	}

	link, err := service.linkRepo.GetByUrl(url)
	if err != nil {
		if !errors.Is(err, domain.ErrLinkNotFound) {
			return domain.TrackedLink{}, err
		}
		link, err = service.linkRepo.Save(domain.Link{
			URL:         url,
			LastUpdated: time.Now(),
		})
		if err != nil {
			return domain.TrackedLink{}, err
		}
	} else {
		existingSubs, _ := service.subRepo.GetByChatId(chatID)
		for _, s := range existingSubs {
			if s.LinkID == link.ID {
				return domain.TrackedLink{}, domain.ErrAlreadySubscribed
			}
		}
	}

	sub := domain.Subscription{
		ChatID:  chatID,
		LinkID:  link.ID,
		Tags:    tags,
		Filters: filters,
	}

	err = service.subRepo.Save(sub)
	if err != nil {
		return domain.TrackedLink{}, err
	}

	return domain.TrackedLink{
		ID:      link.ID,
		URL:     link.URL,
		Tags:    sub.Tags,
		Filters: sub.Filters,
	}, nil
}

func (service *Service) RemoveLink(chatID int64, url string) (domain.TrackedLink, error) {
	_, err := service.chatRepo.GetById(chatID)
	if err != nil {
		return domain.TrackedLink{}, err
	}

	link, err := service.linkRepo.GetByUrl(url)
	if err != nil {
		return domain.TrackedLink{}, err
	}

	sub := domain.Subscription{
		ChatID: chatID,
		LinkID: link.ID,
	}

	sub, err = service.subRepo.Delete(sub)
	if err != nil {
		return domain.TrackedLink{}, err
	}

	if _, err := service.subRepo.GetByLinkId(link.ID); err != nil {
		if !errors.Is(err, domain.ErrLinkNotFound) {
			return domain.TrackedLink{}, err
		}
		_ = service.linkRepo.Delete(link)
	}

	return domain.TrackedLink{
		ID:      link.ID,
		URL:     link.URL,
		Tags:    sub.Tags,
		Filters: sub.Filters,
	}, nil
}
