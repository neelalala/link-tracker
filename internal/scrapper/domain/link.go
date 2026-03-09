package domain

import (
	"errors"
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

var (
	ErrLinkNotFound = errors.New("link not found")
)

type LinkRepository interface {
	Save(link Link) (Link, error)
	GetById(id int64) (Link, error)
	GetByUrl(url string) (Link, error)
	Delete(link Link) error
	GetBatch(limit int, offset int) ([]Link, error)
}
