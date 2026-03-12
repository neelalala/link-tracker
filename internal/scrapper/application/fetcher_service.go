package application

import (
	"errors"
	"time"
)

var (
	ErrUrlNotSupported = errors.New("url not supported")
)

type FetchResult struct {
	UpdatedAt   time.Time
	Description string
}

type LinkFetcher interface {
	CanHandle(url string) bool
	Fetch(url string) (FetchResult, error)
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

func (service *FetcherService) Fetch(url string) (FetchResult, error) {
	for _, linkFetcher := range service.linkFetchers {
		if linkFetcher.CanHandle(url) {
			return linkFetcher.Fetch(url)
		}
	}
	return FetchResult{}, ErrUrlNotSupported
}
