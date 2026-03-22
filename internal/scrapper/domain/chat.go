package domain

import (
	"context"
	"errors"
)

type Chat struct {
	ID int64
}

var (
	ErrChatAlreadyRegistered = errors.New("chat already registered")
	ErrChatNotRegistered     = errors.New("chat already exists")
)

type ChatRepository interface {
	Create(context.Context, Chat) error
	GetById(context.Context, int64) (Chat, error)
	Delete(context.Context, Chat) error
}
