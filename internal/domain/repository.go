package domain

import "errors"

var ErrUserNotFound = errors.New("user not found")

type UserRepository interface {
	Create(user User) error
	GetById(userID int64) (User, error)
	Update(user User) error
	Delete(userID int64) error
}
