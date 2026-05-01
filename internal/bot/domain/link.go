package domain

import (
	"context"
)

type TrackedLink struct {
	ID   int64
	URL  string
	Tags []string
}

type LinkUpdate struct {
	ID          int64
	URL         string
	Description string
	Preview     string
	TgChatIDs   []int64
}

type LinkUpdateHandler interface {
	HandleUpdate(ctx context.Context, update LinkUpdate) error
}
