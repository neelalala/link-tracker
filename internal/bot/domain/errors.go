package domain

import "errors"

var (
	ErrAlreadySubscribed     = errors.New("already subscribed")
	ErrNotSubscribed         = errors.New("not subscribed")
	ErrChatAlreadyRegistered = errors.New("chat already registered")
	ErrChatNotRegistered     = errors.New("chat already exists")
	ErrUrlNotSupported       = errors.New("url not supported")
	ErrLinkNotFound          = errors.New("link not found")
)
