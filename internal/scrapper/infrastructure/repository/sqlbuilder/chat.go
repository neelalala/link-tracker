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

func (chatRepo *ChatRepository) Create(ctx context.Context, chat domain.Chat) error {
	query, args, err := psql.Insert("chats").
		Rows(goqu.Record{"id": chat.ID}).
		ToSQL()
	if err != nil {
		return fmt.Errorf("failed to build query: %w", err)
	}

	_, err = chatRepo.pool.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				return fmt.Errorf("%w: id = %d", domain.ErrChatAlreadyRegistered, chat.ID)
			}
		}

		return fmt.Errorf("failed to register chat: %w", err)
	}

	return nil
}

func (chatRepo *ChatRepository) GetById(ctx context.Context, chatId int64) (domain.Chat, error) {
	query, args, err := psql.From("chats").
		Select("id").
		Where(goqu.Ex{"id": chatId}).
		ToSQL()
	if err != nil {
		return domain.Chat{}, fmt.Errorf("failed to build query: %w", err)
	}

	var saved domain.Chat
	err = chatRepo.pool.QueryRow(ctx, query, args...).Scan(&saved.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Chat{}, fmt.Errorf("%w: chat with id %d not found", domain.ErrChatNotRegistered, chatId)
		}

		return domain.Chat{}, fmt.Errorf("%w: failed to retrieve chat with id %d", err, chatId)
	}

	return saved, nil
}

func (chatRepo *ChatRepository) Delete(ctx context.Context, chat domain.Chat) error {
	query, args, err := psql.Delete("chats").
		Where(goqu.Ex{"id": chat.ID}).
		ToSQL()
	if err != nil {
		return fmt.Errorf("failed to build query: %w", err)
	}

	cmdTag, err := chatRepo.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete chat with id %d: %w", chat.ID, err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("%w: chat with id %d not found", domain.ErrChatNotRegistered, chat.ID)
	}

	return nil
}
