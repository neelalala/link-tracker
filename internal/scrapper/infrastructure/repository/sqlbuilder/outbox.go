package sqlbuilder

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type OutboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{
		pool: pool,
	}
}

func (outboxRepo *OutboxRepository) Add(ctx context.Context, topic string, payload []byte) (domain.Outbox, error) {
	panic("implement me")
}

func (outboxRepo *OutboxRepository) getByStatus(ctx context.Context, status domain.OutboxStatus, limit int) ([]domain.Outbox, error) {
	panic("implement me")
}

func (outboxRepo *OutboxRepository) GetPending(ctx context.Context, limit int) ([]domain.Outbox, error) {
	return outboxRepo.getByStatus(ctx, domain.OutboxStatusPending, limit)
}

func (outboxRepo *OutboxRepository) GetFailed(ctx context.Context, limit int) ([]domain.Outbox, error) {
	return outboxRepo.getByStatus(ctx, domain.OutboxStatusFailed, limit)
}

func (outboxRepo *OutboxRepository) UpdateStatus(ctx context.Context, id int64, status domain.OutboxStatus) error {
	panic("implement me")
}
