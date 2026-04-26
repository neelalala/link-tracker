package domain

import (
	"context"
)

type Chat struct {
	ID int64
}

//go:generate mockgen -source=chat.go -destination=../mocks/mock_domain_chat_repository.go -package=mocks
type ChatRepository interface {
	Create(ctx context.Context, id int64) error
	GetByID(ctx context.Context, id int64) (Chat, error)
	Delete(ctx context.Context, id int64) error
}
