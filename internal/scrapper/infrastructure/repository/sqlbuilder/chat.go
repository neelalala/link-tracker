package sqlbuilder

import (
	"context"
	"errors"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type ChatRepository struct {
	pool *pgxpool.Pool
}

func NewChatRepository(pool *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{
		pool: pool,
	}
}

func (chatRepo *ChatRepository) Create(ctx context.Context, id int64) error {
	query, args, err := psql.Insert("chats").
		Rows(goqu.Record{"id": id}).
		ToSQL()
	if err != nil {
		return fmt.Errorf("failed to build query: %w", err)
	}

	db := GetDB(ctx, chatRepo.pool)

	_, err = db.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				return fmt.Errorf("%w: id = %d", domain.ErrChatAlreadyRegistered, id)
			}
		}

		return fmt.Errorf("failed to register chat: %w", err)
	}

	return nil
}

func (chatRepo *ChatRepository) GetByID(ctx context.Context, id int64) (domain.Chat, error) {
	query, args, err := psql.From("chats").
		Select("id").
		Where(goqu.Ex{"id": id}).
		ToSQL()
	if err != nil {
		return domain.Chat{}, fmt.Errorf("failed to build query: %w", err)
	}

	db := GetDB(ctx, chatRepo.pool)

	var saved domain.Chat
	err = db.QueryRow(ctx, query, args...).Scan(&saved.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Chat{}, fmt.Errorf("%w: chat with id %d not found", domain.ErrChatNotRegistered, id)
		}

		return domain.Chat{}, fmt.Errorf("%w: failed to retrieve chat with id %d", err, id)
	}

	return saved, nil
}

func (chatRepo *ChatRepository) Delete(ctx context.Context, id int64) error {
	query, args, err := psql.Delete("chats").
		Where(goqu.Ex{"id": id}).
		ToSQL()
	if err != nil {
		return fmt.Errorf("failed to build query: %w", err)
	}

	db := GetDB(ctx, chatRepo.pool)

	cmdTag, err := db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete chat with id %d: %w", id, err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("%w: chat with id %d not found", domain.ErrChatNotRegistered, id)
	}

	return nil
}
