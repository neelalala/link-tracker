package domain

import (
	"context"
)

type Chat struct {
	ID int64
}

type ChatRepository interface {
	Create(ctx context.Context, id int64) error
	GetByID(ctx context.Context, id int64) (Chat, error)
	Delete(ctx context.Context, id int64) error
}
