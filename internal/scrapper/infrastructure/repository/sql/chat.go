package sql

import (
	"context"
	"errors"
	"fmt"

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
	query := `
		INSERT INTO chats (id)
		VALUES ($1)
	`

	_, err := chatRepo.pool.Exec(ctx, query, id)
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
	query := `SELECT id FROM chats WHERE id = $1`

	var saved domain.Chat
	err := chatRepo.pool.QueryRow(ctx, query, id).Scan(&saved.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Chat{}, fmt.Errorf("%w: chat with id %d not found", domain.ErrChatNotRegistered, id)
		}

		return domain.Chat{}, fmt.Errorf("%w: failed to retrieve chat with id %d", err, id)
	}

	return saved, nil
}

func (chatRepo *ChatRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM chats WHERE id = $1`

	cmdTag, err := chatRepo.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete chat with id %d: %w", id, err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("%w: chat with id %d not found", domain.ErrChatNotRegistered, id)
	}

	return nil
}
