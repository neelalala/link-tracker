package domain

import (
	"context"
)

type Subscription struct {
	ChatID int64
	LinkID int64
	Tags   []string
}

type TrackedLink struct {
	ID   int64
	URL  string
	Tags []string
}

type SubscriptionRepository interface {
	Save(ctx context.Context, sub Subscription) error
	GetByChatId(ctx context.Context, chatId int64) ([]Subscription, error)
	GetByLinkId(ctx context.Context, linkId int64) ([]Subscription, error)
	Exists(ctx context.Context, chatId int64, linkId int64) (bool, error)
	Delete(ctx context.Context, sub Subscription) (Subscription, error)
	AddTags(ctx context.Context, linkId, chatId int64, tags []string) error
	GetTags(ctx context.Context, linkId, chatId int64) ([]string, error)
	DeleteTags(ctx context.Context, linkId, chatId int64, tags []string) error
}
