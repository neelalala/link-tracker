package domain

import (
	"errors"
	"fmt"
)

var (
	ErrAlreadySubscribed               = errors.New("already subscribed")
	ErrNotSubscribed                   = errors.New("not subscribed")
	ErrChatAlreadyRegistered           = errors.New("chat already registered")
	ErrChatNotRegistered               = errors.New("chat not registered")
	ErrURLNotSupported                 = errors.New("url not supported")
	ErrChatNotRegisteredOrLinkNotFound = errors.New("chat not registered or link not found")
	ErrBadSessionState                 = fmt.Errorf("unknown session state")
	ErrSessionNotFound                 = errors.New("session not found")
)
