package domain

import (
	"context"
)

type Scrapper interface {
	RegisterChat(ctx context.Context, chatId int64) error
	DeleteChat(ctx context.Context, chatId int64) error
	GetTrackedLinks(ctx context.Context, chatId int64) ([]TrackedLink, error)
	AddLink(ctx context.Context, chatId int64, url string, tags []string) (TrackedLink, error)
	RemoveLink(ctx context.Context, chatId int64, url string) (TrackedLink, error)
}
