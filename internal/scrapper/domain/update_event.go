package domain

import (
	"context"
	"time"
)

type UpdateEvent interface {
	UpdatedAt() time.Time
	Description() string
	Preview() string
}

//go:generate mockgen -source=update_event.go -destination=../mocks/mock_domain_link_fetcher.go -package=mocks
type LinkFetcher interface {
	CanHandle(url string) bool
	Fetch(ctx context.Context, url string, since time.Time) ([]UpdateEvent, error)
}
