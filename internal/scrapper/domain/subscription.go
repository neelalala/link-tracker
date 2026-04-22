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

//go:generate mockgen -source=subscription.go -destination=../mocks/mock_domain_subscription.go -package=mocks
type SubscriptionRepository interface {
	Save(ctx context.Context, sub Subscription) error
	GetByChatID(ctx context.Context, chatID int64) ([]Subscription, error)
	GetByLinkID(ctx context.Context, linkID int64) ([]Subscription, error)
	Exists(ctx context.Context, chatID int64, linkID int64) (bool, error)
	Delete(ctx context.Context, chatID int64, linkID int64) (Subscription, error)
	AddTags(ctx context.Context, linkID, chatID int64, tags []string) error
	GetTags(ctx context.Context, linkID, chatID int64) ([]string, error)
	DeleteTags(ctx context.Context, linkID, chatID int64, tags []string) error
}
