package domain

import (
	"context"
	"time"
)

type Link struct {
	ID          int64
	URL         string
	LastUpdated time.Time
}

type LinkUpdate struct {
	ID          int64
	URL         string
	Description string
	TgChatIDs   []int64
}

type LinkRepository interface {
	Save(ctx context.Context, link Link) (Link, error)
	GetById(ctx context.Context, id int64) (Link, error)
	GetByUrl(ctx context.Context, url string) (Link, error)
	Delete(ctx context.Context, link Link) error
	GetBatch(ctx context.Context, limit int, offset int) ([]Link, error)
}
