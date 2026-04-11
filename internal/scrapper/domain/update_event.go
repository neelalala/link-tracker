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

type LinkFetcher interface {
	CanHandle(url string) bool
	Fetch(ctx context.Context, url string, since time.Time) ([]UpdateEvent, error)
}
