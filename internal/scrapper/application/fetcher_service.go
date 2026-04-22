package application

import (
	"context"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"time"
)

type FetchResult struct {
	UpdatedAt   time.Time
	Description string
}

type LinkFetcher interface {
	CanHandle(url string) bool
	Fetch(ctx context.Context, url string) (FetchResult, error)
}

type FetcherService struct {
	linkFetchers []LinkFetcher
}

func NewFetcherService(fetchers []LinkFetcher) *FetcherService {
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

func (service *FetcherService) Fetch(ctx context.Context, url string) (FetchResult, error) {
	for _, linkFetcher := range service.linkFetchers {
		if linkFetcher.CanHandle(url) {
			return linkFetcher.Fetch(ctx, url)
		}
	}
	return FetchResult{}, domain.ErrURLNotSupported
}
