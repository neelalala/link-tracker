package domain

import "context"

type CommandInfo struct {
	Name        string
	Description string
}

type Command interface {
	Name() string
	Description() string
	Execute(ctx context.Context, msg Message) (string, error)
}
