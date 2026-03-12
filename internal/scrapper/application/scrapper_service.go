package application

import (
	"context"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"log/slog"
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

func (s *ScrapperService) GetUpdates(ctx context.Context) error {
	s.logger.Info("started checking all links for updates")

	offset := 0

	for {
		links, err := s.linkRepo.GetBatch(ctx, batchSize, offset)
		if err != nil {
			s.logger.Error("failed to get batch of links", slog.String("error", err.Error()))
			return err
		}

		if len(links) == 0 {
			break
		}

		for _, link := range links {
			s.processLink(ctx, link)
		}

		offset += batchSize
	}

	s.logger.Info("finished checking updates")
	return nil
}

func (s *ScrapperService) processLink(ctx context.Context, link domain.Link) {
	subs, err := s.subRepo.GetByLinkId(ctx, link.ID)
	if err != nil {
		s.logger.Error("failed to get subscriptions", slog.Int64("link_id", link.ID))
		return
	}

	chatIDs := make([]int64, len(subs))
	for i, sub := range subs {
		chatIDs[i] = sub.ChatID
	}

	if !s.fetcher.CanHandle(link.URL) {
		s.logger.Warn("no fetcher found for url", slog.String("url", link.URL))
		if len(chatIDs) > 0 {
			update := domain.LinkUpdate{
				URL:         link.URL,
				Description: "no fetcher for this link yet",
				TgChatIDs:   chatIDs,
			}
			s.notifier.SendUpdate(ctx, update)
		} else {
			s.logger.Warn("link with no subscribers still in DB", slog.String("url", link.URL))
		}
		return
	}

	result, err := s.fetcher.Fetch(ctx, link.URL)
	if err != nil {
		s.logger.Error("failed to fetch link", slog.String("url", link.URL), slog.String("error", err.Error()))
		return
	}

	if result.UpdatedAt.After(link.LastUpdated) {
		s.logger.Info("found update for link", slog.String("url", link.URL))

		if len(chatIDs) > 0 {
			update := domain.LinkUpdate{
				ID:          link.ID,
				URL:         link.URL,
				Description: result.Description,
				TgChatIDs:   chatIDs,
			}

			err = s.notifier.SendUpdate(ctx, update)
			if err != nil {
				s.logger.Error("failed to notify bot", slog.String("error", err.Error()))
				return
			}
		}

		link.LastUpdated = result.UpdatedAt
		_, err = s.linkRepo.Save(ctx, link)
		if err != nil {
			s.logger.Error("failed to update link in DB", slog.Int64("link_id", link.ID))
		}
	}
}
