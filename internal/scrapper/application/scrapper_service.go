package application

import (
	"context"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

const batchSize = 100

type UpdateNotifier interface {
	SendUpdate(ctx context.Context, update domain.LinkUpdate) error
}

type ScrapperService struct {
	linkRepo domain.LinkRepository
	subRepo  domain.SubscriptionRepository
	fetcher  *FetcherService
	notifier UpdateNotifier
	logger   *slog.Logger
}

func NewScrapperService(
	linkRepo domain.LinkRepository,
	subRepo domain.SubscriptionRepository,
	fetcher *FetcherService,
	notifier UpdateNotifier,
	logger *slog.Logger,
) *ScrapperService {
	return &ScrapperService{
		linkRepo: linkRepo,
		subRepo:  subRepo,
		fetcher:  fetcher,
		notifier: notifier,
		logger:   logger,
	}
}

func (service *ScrapperService) GetUpdates(ctx context.Context) error {
	service.logger.Info("started checking all links for updates")

	offset := 0

	for {
		links, err := service.linkRepo.GetBatch(ctx, batchSize, offset)
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
			service.processLink(ctx, link)
		}

		offset += batchSize
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

	result, err := service.fetcher.Fetch(ctx, link.URL)
	if err != nil {
		service.logger.Error("failed to fetch link",
			slog.String("url", link.URL),
			slog.String("error", err.Error()),
			slog.String("context", "scrapperService.fetcher.Fetch"),
		)
		return
	}

	if result.UpdatedAt.After(link.LastUpdated) {
		service.logger.Info("found update for link", slog.String("url", link.URL))

		if len(chatIDs) > 0 {
			update := domain.LinkUpdate{
				ID:          link.ID,
				URL:         link.URL,
				Description: result.Description,
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

		link.LastUpdated = result.UpdatedAt
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
