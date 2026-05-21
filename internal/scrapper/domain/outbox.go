package domain

import (
	"context"
	"time"
)

type OutboxStatus string

const (
	OutboxStatusPending OutboxStatus = "PENDING"
	OutboxStatusSent    OutboxStatus = "SENT"
	OutboxStatusFailed  OutboxStatus = "FAILED"
)

type Outbox struct {
	ID        int64
	Topic     string
	Payload   []byte
	Status    OutboxStatus
	Retries   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type OutboxRepository interface {
	Add(ctx context.Context, topic string, payload []byte) (Outbox, error)
	GetPending(ctx context.Context, limit int) ([]Outbox, error)
	GetFailed(ctx context.Context, limit int) ([]Outbox, error)
	UpdateStatus(ctx context.Context, id int64, status OutboxStatus) error
	IncrementRetries(ctx context.Context, id int64) (int, error)
}
