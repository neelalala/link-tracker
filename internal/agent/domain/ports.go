package domain

import "context"

type UpdateSender interface {
	SendUpdate(ctx context.Context, update ProcessedLinkUpdate) error
}

type Filter interface {
	Check(update LinkUpdate) bool
}

type Transformer interface {
	Transform(ctx context.Context, raw LinkUpdate) (ProcessedLinkUpdate, error)
}
