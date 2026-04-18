package domain

import (
	"context"
)

type Chat struct {
	ID int64
}

type ChatRepository interface {
	Create(context.Context, Chat) error
	GetById(context.Context, int64) (Chat, error)
	Delete(context.Context, Chat) error
}
