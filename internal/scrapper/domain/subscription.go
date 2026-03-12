package domain

import (
	"context"
	"errors"
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

var (
	ErrAlreadySubscribed = errors.New("already subscribed")
	ErrNotSubscribed     = errors.New("not subscribed")
)

type SubscriptionRepository interface {
	Save(ctx context.Context, sub Subscription) error
	GetByChatId(ctx context.Context, chatId int64) ([]Subscription, error)
	GetByLinkId(ctx context.Context, linkId int64) ([]Subscription, error)
	Delete(ctx context.Context, sub Subscription) (Subscription, error)
}
