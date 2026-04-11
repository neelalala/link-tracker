package application

import (
	"context"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type FetcherService struct {
	linkFetchers []domain.LinkFetcher
}

func NewFetcherService(fetchers []domain.LinkFetcher) *FetcherService {
	return &FetcherService{
		linkFetchers: fetchers,
	}
}

func (service *FetcherService) CanHandle(url string) bool {
	for _, linkFetcher := range service.linkFetchers {
		if linkFetcher.CanHandle(url) {
			return true
		}
	}
	return false
}

func (service *FetcherService) Fetch(ctx context.Context, url string, since time.Time) ([]domain.UpdateEvent, error) {
	for _, linkFetcher := range service.linkFetchers {
		if linkFetcher.CanHandle(url) {
			return linkFetcher.Fetch(ctx, url, since)
		}
	}
	return nil, domain.ErrURLNotSupported
}
