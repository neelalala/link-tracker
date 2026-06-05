package subscription

import (
	"context"
	"log/slog"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type LinksCache interface {
	GetLinks(ctx context.Context, chatID int64) ([]domain.TrackedLink, bool, error)
	SetLinks(ctx context.Context, chatID int64, links []domain.TrackedLink) error
	Invalidate(ctx context.Context, chatID int64) error
}

type CachingService struct {
	service *Service
	cache   LinksCache

	log *slog.Logger
}

func NewCachingService(service *Service, cache LinksCache) *CachingService {
	return &CachingService{
		service: service,
		cache:   cache,
		log:     service.logger,
	}
}

func (service *CachingService) RegisterChat(ctx context.Context, chatID int64) error {
	return service.service.RegisterChat(ctx, chatID)
}

func (service *CachingService) DeleteChat(ctx context.Context, chatID int64) error {
	if err := service.service.DeleteChat(ctx, chatID); err != nil {
		return err
	}
	return service.cache.Invalidate(ctx, chatID)
}

func (service *CachingService) GetTrackedLinks(ctx context.Context, chatID int64) ([]domain.TrackedLink, error) {
	service.log.Debug("getting cached links",
		"context", "CachingService.GetTrackedLinks",
		"chatID", chatID,
	)
	links, hit, err := service.cache.GetLinks(ctx, chatID)
	if err != nil {
		service.log.Warn("failed to get cached links",
			"context", "CachingService.GetTrackedLinks",
			"chatID", chatID,
			"err", err,
		)
	}

	if hit {
		service.log.Info("got cached links",
			"context", "CachingService.GetTrackedLinks",
			"chatID", chatID,
			"count", len(links),
		)
		return links, nil
	}

	service.log.Info("get tracked links cache miss",
		"context", "CachingService.GetTrackedLinks",
		"chatID", chatID,
	)

	links, err = service.service.GetTrackedLinks(ctx, chatID)
	if err != nil {
		return nil, err
	}

	if err := service.cache.SetLinks(ctx, chatID, links); err != nil {
		service.log.Warn("failed to set cached links",
			"context", "CachingService.GetTrackedLinks",
			"chatID", chatID,
			"err", err,
		)
	}

	return links, nil
}

func (service *CachingService) AddLink(ctx context.Context, chatID int64, url string, tags []string) (domain.TrackedLink, error) {
	service.log.Debug("adding link",
		"context", "CachingService.AddLink",
		"chatID", chatID,
		"url", url,
		"tags", strings.Join(tags, ","),
	)
	link, err := service.service.AddLink(ctx, chatID, url, tags)
	if err != nil {
		return domain.TrackedLink{}, err
	}

	if err := service.cache.Invalidate(ctx, chatID); err != nil {
		service.log.Warn("failed to invalidate cache",
			"context", "CachingService.AddLink",
			"chatID", chatID,
			"err", err,
		)
	}

	return link, nil
}

func (service *CachingService) RemoveLink(ctx context.Context, chatID int64, url string) (domain.TrackedLink, error) {
	service.log.Debug("removing link",
		"context", "CachingService.RemoveLink",
		"chatID", chatID,
		"url", url,
	)
	link, err := service.service.RemoveLink(ctx, chatID, url)
	if err != nil {
		return domain.TrackedLink{}, err
	}

	if err := service.cache.Invalidate(ctx, chatID); err != nil {
		service.log.Warn("failed to invalidate cache",
			"context", "CachingService.RemoveLink",
			"chatID", chatID,
			"err", err,
			"url", url,
		)
	}

	return link, nil
}
