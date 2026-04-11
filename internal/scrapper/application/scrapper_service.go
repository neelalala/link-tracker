package application

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type UpdateNotifier interface {
	SendUpdate(ctx context.Context, update domain.LinkUpdate) error
}

type ScrapperService struct {
	linkRepo domain.LinkRepository
	subRepo  domain.SubscriptionRepository
	fetcher  *FetcherService
	notifier UpdateNotifier

	batchSize     int
	fetchersCount int

	logger *slog.Logger
}

func NewScrapperService(
	linkRepo domain.LinkRepository,
	subRepo domain.SubscriptionRepository,
	fetcher *FetcherService,
	notifier UpdateNotifier,
	batchSize int,
	fetchersCount int,
	logger *slog.Logger,
) (*ScrapperService, error) {
	if batchSize <= 0 {
		return nil, fmt.Errorf("batchSize must be positive, got %d", batchSize)
	}

	if fetchersCount <= 0 {
		return nil, fmt.Errorf("fetchersCount must be positive, got %d", fetchersCount)
	}

	return &ScrapperService{
		linkRepo:      linkRepo,
		subRepo:       subRepo,
		fetcher:       fetcher,
		notifier:      notifier,
		batchSize:     batchSize,
		fetchersCount: fetchersCount,
		logger:        logger,
	}, nil
}

func (service *ScrapperService) GetUpdates(ctx context.Context) error {
	service.logger.Info("started checking all links for updates")

	jobs := make(chan domain.Link, service.batchSize)
	defer close(jobs)

	var wg sync.WaitGroup
	defer wg.Wait()

	for range service.fetchersCount {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for link := range jobs {
				service.processLink(ctx, link)
			}
		}()
	}

	offset := 0

	for {
		links, err := service.linkRepo.GetBatch(ctx, service.batchSize, offset)
		if err != nil {
			service.logger.Error("failed to get batch of links",
				slog.String("error", err.Error()),
				slog.String("context", "scrapperService.linkRepo.GetBatch"),
			)
			return err
		}

		if len(links) == 0 {
			break
		}

		for _, link := range links {
			select {
			case <-ctx.Done():
				service.logger.Warn("context cancelled, stopping updates")
				return ctx.Err()
			case jobs <- link:
			}
		}

		offset += service.batchSize
	}

	service.logger.Info("finished checking updates")
	return nil
}

func (service *ScrapperService) processLink(ctx context.Context, link domain.Link) {
	subscriptions, err := service.subRepo.GetByLinkId(ctx, link.ID)
	if err != nil {
		service.logger.Error("failed to get subscriptions",
			slog.String("context", "scrapperService.subRepo.GetByLinkId"),
			slog.Int64("link_id", link.ID),
			slog.String("error", err.Error()))
		return
	}

	chatIDs := make([]int64, len(subscriptions))
	for i, sub := range subscriptions {
		chatIDs[i] = sub.ChatID
	}

	if !service.fetcher.CanHandle(link.URL) {
		service.logger.Warn("no fetcher found for url", slog.String("url", link.URL))
		if len(chatIDs) > 0 {
			update := domain.LinkUpdate{
				URL:         link.URL,
				Description: "no fetcher for this link yet",
				TgChatIDs:   chatIDs,
			}
			err := service.notifier.SendUpdate(ctx, update)
			if err != nil {
				service.logger.Error("failed to send update",
					slog.String("error", err.Error()),
					slog.Int64("link_id", link.ID),
					slog.Any("chat_ids", update.TgChatIDs),
					slog.String("context", "scrapperService.notifier.SendUpdate"),
				)
			}
		} else {
			service.logger.Warn("link with no subscribers still in DB", slog.String("url", link.URL))
		}
		return
	}

	events, err := service.fetcher.Fetch(ctx, link.URL, link.LastUpdated)
	if err != nil {
		service.logger.Error("failed to fetch link",
			slog.String("url", link.URL),
			slog.String("error", err.Error()),
			slog.String("context", "scrapperService.fetcher.Fetch"),
		)
		return
	}

	service.logger.Info("found update for link",
		slog.String("url", link.URL),
		slog.Int("count", len(events)),
	)

	for _, event := range events {
		if len(chatIDs) > 0 {
			update := domain.LinkUpdate{
				ID:          link.ID,
				URL:         link.URL,
				Description: event.Description(),
				TgChatIDs:   chatIDs,
			}

			err = service.notifier.SendUpdate(ctx, update)
			if err != nil {
				service.logger.Error("failed to notify bot",
					slog.String("error", err.Error()),
					slog.String("context", "scrapperService.notifier.SendUpdate"),
				)
				return
			}
		}

		link.LastUpdated = event.UpdatedAt()
		_, err = service.linkRepo.Save(ctx, link)
		if err != nil {
			service.logger.Error("failed to update link in DB",
				slog.Int64("link_id", link.ID),
				slog.String("error", err.Error()),
				slog.String("context", "scrapperService.linkRepo.Save"),
			)
		}
	}
}
