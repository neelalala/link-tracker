package domain

import "errors"

var ErrUserNotFound = errors.New("user not found")

type UserRepository interface {
	Save(user User) error
	GetById(userID int64) (User, error)
	Delete(userID int64) error
}

var (
	ErrLinkAlreadyTracked = errors.New("link already tracked")
	ErrLinkNotFound       = errors.New("link not found")
)

type LinkRepository interface {
	Save(link Link) error
	GetByUserIdChatId(userID, chatID int64) ([]Link, error)
	Delete(link Link) error
}
