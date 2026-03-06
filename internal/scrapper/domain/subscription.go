package domain

import "errors"

type Subscription struct {
	ChatID  int64
	LinkID  int64
	Tags    []string
	Filters []string
}

type TrackedLink struct {
	ID      int64
	URL     string
	Tags    []string
	Filters []string
}

var (
	ErrAlreadySubscribed = errors.New("already subscribed")
	ErrNotSubscribed     = errors.New("not subscribed")
)

type SubscriptionRepository interface {
	Save(Subscription) error
	GetByChatId(int64) ([]Subscription, error)
	GetByLinkId(int64) ([]Subscription, error)
	Delete(Subscription) (Subscription, error)
}
