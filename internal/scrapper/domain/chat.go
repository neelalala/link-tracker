package domain

import (
	"context"
)

type Chat struct {
	ID int64
}

//go:generate mockgen -source=chat.go -destination=../mocks/mock_domain_chat_repository.go -package=mocks
type ChatRepository interface {
	Create(context.Context, Chat) error
	GetById(context.Context, int64) (Chat, error)
	Delete(context.Context, Chat) error
}
