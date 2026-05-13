package sqlbuilder

import (
	"context"
	"fmt"

	"github.com/doug-martin/goqu/v9"
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
	query, args, err := psql.Insert(goqu.T("outbox")).
		Rows(
			goqu.Record{"topic": topic,
				"payload": string(payload),
			},
		).
		Returning("id", "topic", "payload", "status", "created_at", "updated_at").
		ToSQL()
	if err != nil {
		return domain.Outbox{}, fmt.Errorf("failed to build query: %w", err)
	}

	db := GetDB(ctx, outboxRepo.pool)

	var saved domain.Outbox
	err = db.QueryRow(ctx, query, args...).Scan(
		&saved.ID,
		&saved.Topic,
		&saved.Payload,
		&saved.Status,
		&saved.CreatedAt,
		&saved.UpdatedAt,
	)
	if err != nil {
		return domain.Outbox{}, fmt.Errorf("failed to save outbox: %w", err)
	}

	return saved, nil
}

func (outboxRepo *OutboxRepository) getByStatus(ctx context.Context, status domain.OutboxStatus, limit int) ([]domain.Outbox, error) {
	query, args, err := psql.Select("id", "topic", "payload", "status", "crated_at", "updated_at").
		From(goqu.T("outbox")).
		Where(goqu.C("status").Eq(status)).
		Order(goqu.C("created_at").Asc()).
		Limit(uint(limit)).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	db := GetDB(ctx, outboxRepo.pool)

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get outboxes with status %v: %w", status, err)
	}
	defer rows.Close()

	outboxes := make([]domain.Outbox, 0, limit)

	for rows.Next() {
		var outbox domain.Outbox
		if err := rows.Scan(
			&outbox.ID,
			&outbox.Topic,
			&outbox.Payload,
			&outbox.Status,
			&outbox.CreatedAt,
			&outbox.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan outbox: %w", err)
		}
		outboxes = append(outboxes, outbox)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over outboxes with status %v: %w", status, err)
	}

	return outboxes, nil
}

func (outboxRepo *OutboxRepository) GetPending(ctx context.Context, limit int) ([]domain.Outbox, error) {
	return outboxRepo.getByStatus(ctx, domain.OutboxStatusPending, limit)
}

func (outboxRepo *OutboxRepository) GetFailed(ctx context.Context, limit int) ([]domain.Outbox, error) {
	return outboxRepo.getByStatus(ctx, domain.OutboxStatusFailed, limit)
}

func (outboxRepo *OutboxRepository) UpdateStatus(ctx context.Context, id int64, status domain.OutboxStatus) error {
	query, args, err := psql.Update(goqu.T("outbox")).
		Set(goqu.Record{"status": status, "updated_at": "CURRENT_TIMESTAMP"}).
		Where(goqu.C("id").Eq(id)).
		ToSQL()
	if err != nil {
		return fmt.Errorf("failed to build query: %w", err)
	}

	db := GetDB(ctx, outboxRepo.pool)

	_, err = db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update outbox status: %w", err)
	}

	return nil
}
