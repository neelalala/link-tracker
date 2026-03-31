package domain

import (
	"context"
)

//go:generate mockgen -source=scrapper.go -destination=../mocks/mock_domain_scrapper.go -package=mocks
type Scrapper interface {
	RegisterChat(ctx context.Context, chatId int64) error
	DeleteChat(ctx context.Context, chatId int64) error
	GetTrackedLinks(ctx context.Context, chatId int64) ([]TrackedLink, error)
	AddLink(ctx context.Context, chatId int64, url string, tags []string) (TrackedLink, error)
	RemoveLink(ctx context.Context, chatId int64, url string) (TrackedLink, error)
}
