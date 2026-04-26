package domain

import "errors"

var (
	ErrChatAlreadyRegistered = errors.New("chat already registered")
	ErrChatNotRegistered     = errors.New("chat already exists")
	ErrLinkNotFound          = errors.New("link not found")
	ErrAlreadySubscribed     = errors.New("already subscribed")
	ErrNotSubscribed         = errors.New("not subscribed")
	ErrURLNotSupported       = errors.New("url not supported")
)
