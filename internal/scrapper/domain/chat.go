package domain

import "errors"

type Chat struct {
	ID int64
}

var (
	ErrChatAlreadyRegistered = errors.New("chat already registered")
	ErrChatNotRegistered     = errors.New("chat already exists")
)

type ChatRepository interface {
	Save(Chat) error
	GetById(int64) (Chat, error)
	Delete(Chat) error
}
