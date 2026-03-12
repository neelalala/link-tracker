package application

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"log/slog"
	"time"
)

const batchSize = 100

type FetchResult struct {
	UpdatedAt   time.Time
	Description string
}

type LinkFetcher interface {
	CanHandle(url string) bool
	Fetch(url string) (FetchResult, error)
}

type UpdateNotifier interface {
	SendUpdate(update domain.LinkUpdate) error
}

type ScrapperService struct {
	linkRepo domain.LinkRepository
	subRepo  domain.SubscriptionRepository
	fetchers []LinkFetcher
	notifier UpdateNotifier
	logger   *slog.Logger
}

func NewScrapperService(
	linkRepo domain.LinkRepository,
	subRepo domain.SubscriptionRepository,
	fetchers []LinkFetcher,
	notifier UpdateNotifier,
	logger *slog.Logger,
) *ScrapperService {
	return &ScrapperService{
		linkRepo: linkRepo,
		subRepo:  subRepo,
		fetchers: fetchers,
		notifier: notifier,
		logger:   logger,
	}
}

func (s *ScrapperService) GetUpdates() error {
	s.logger.Info("started checking all links for updates")

	offset := 0

	for {
		links, err := s.linkRepo.GetBatch(batchSize, offset)
		if err != nil {
			s.logger.Error("failed to get batch of links", slog.String("error", err.Error()))
			return err
		}

		if len(links) == 0 {
			break
		}

		for _, link := range links {
			s.processLink(link)
		}

		offset += batchSize
	}

	s.logger.Info("finished checking updates")
	return nil
}

func (s *ScrapperService) processLink(link domain.Link) {
	var fetcher LinkFetcher
	for _, f := range s.fetchers {
		if f.CanHandle(link.URL) {
			fetcher = f
			break
		}
	}

	subs, err := s.subRepo.GetByLinkId(link.ID)
	if err != nil {
		s.logger.Error("failed to get subscriptions", slog.Int64("link_id", link.ID))
		return
	}

	chatIDs := make([]int64, len(subs))
	for i, sub := range subs {
		chatIDs[i] = sub.ChatID
	}

	if fetcher == nil {
		s.logger.Warn("no fetcher found for url", slog.String("url", link.URL))
		if len(chatIDs) > 0 {
			update := domain.LinkUpdate{
				URL:         link.URL,
				Description: "no fetcher for this link yet",
				TgChatIDs:   chatIDs,
			}
			s.notifier.SendUpdate(update)
		}
		return
	}

	result, err := fetcher.Fetch(link.URL)
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

			err = s.notifier.SendUpdate(update)
			if err != nil {
				s.logger.Error("failed to notify bot", slog.String("error", err.Error()))
				return
			}
		}

		link.LastUpdated = result.UpdatedAt
		_, err = s.linkRepo.Save(link)
		if err != nil {
			s.logger.Error("failed to update link in DB", slog.Int64("link_id", link.ID))
		}
	}
}
