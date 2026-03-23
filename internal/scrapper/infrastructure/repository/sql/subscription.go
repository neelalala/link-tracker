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

type SubscriptionRepository struct {
	pool *pgxpool.Pool
}

func NewSubscriptionRepository(pool *pgxpool.Pool) *SubscriptionRepository {
	return &SubscriptionRepository{
		pool: pool,
	}
}

func (subRepo *SubscriptionRepository) Save(ctx context.Context, sub domain.Subscription) error {
	tx, err := subRepo.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	subQuery := `
		INSERT INTO subscriptions (chat_id, link_id)
		VALUES ($1, $2)
	`

	_, err = tx.Exec(ctx, subQuery, sub.ChatID, sub.LinkID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				return fmt.Errorf("%w: chat id = %d, link id = %d", domain.ErrAlreadySubscribed, sub.ChatID, sub.LinkID)
			}
		}
		return fmt.Errorf("failed to insert subscription: %w", err)
	}

	if len(sub.Tags) > 0 {
		tagQuery := `
			INSERT INTO subscription_tags (chat_id, link_id, tag)
			VALUES ($1, $2, $3)
		`

		for _, tag := range sub.Tags {
			_, err = tx.Exec(ctx, tagQuery, sub.ChatID, sub.LinkID, tag)
			if err != nil {
				var pgErr *pgconn.PgError
				if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
					return fmt.Errorf("%w: tag '%subRepo' already exists for chat id = %d, link id = %d", domain.ErrAlreadySubscribed, tag, sub.ChatID, sub.LinkID)
				}
				return fmt.Errorf("failed to insert tag %subRepo: %w", tag, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (subRepo *SubscriptionRepository) GetByChatId(ctx context.Context, chatId int64) ([]domain.Subscription, error) {
	query := `
		SELECT 
			subRepo.chat_id, 
			l.id,
			st.tag
		FROM subscriptions subRepo
		JOIN links l ON subRepo.link_id = l.id
		LEFT JOIN subscription_tags st ON subRepo.chat_id = st.chat_id AND subRepo.link_id = st.link_id
		WHERE subRepo.chat_id = $1
	`

	rows, err := subRepo.pool.Query(ctx, query, chatId)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscriptions for chat %d: %w", chatId, err)
	}
	defer rows.Close()

	subsMap := make(map[int64]*domain.Subscription)

	for rows.Next() {
		var subChatID, linkID int64
		var tag *string

		err := rows.Scan(
			&subChatID,
			&linkID,
			&tag,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subscription row: %w", err)
		}

		sub, exists := subsMap[linkID]
		if !exists {
			sub = &domain.Subscription{
				ChatID: subChatID,
				LinkID: linkID,
				Tags:   make([]string, 0),
			}
			subsMap[linkID] = sub
		}

		if tag != nil {
			sub.Tags = append(sub.Tags, *tag)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	var result []domain.Subscription
	for _, sub := range subsMap {
		result = append(result, *sub)
	}

	return result, nil
}

func (subRepo *SubscriptionRepository) GetByLinkId(ctx context.Context, linkId int64) ([]domain.Subscription, error) {
	query := `
		SELECT 
			subRepo.chat_id, 
			l.id, 
			st.tag
		FROM subscriptions subRepo
		JOIN links l ON subRepo.link_id = l.id
		LEFT JOIN subscription_tags st ON subRepo.chat_id = st.chat_id AND subRepo.link_id = st.link_id
		WHERE subRepo.link_id = $1
	`

	rows, err := subRepo.pool.Query(ctx, query, linkId)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscriptions for link %d: %w", linkId, err)
	}
	defer rows.Close()

	subsMap := make(map[int64]*domain.Subscription)

	for rows.Next() {
		var subChatID, linkID int64
		var tag *string

		err := rows.Scan(
			&subChatID,
			&linkID,
			&tag,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subscription row: %w", err)
		}

		sub, exists := subsMap[subChatID]
		if !exists {
			sub = &domain.Subscription{
				ChatID: subChatID,
				LinkID: linkID,
				Tags:   make([]string, 0),
			}
			subsMap[subChatID] = sub
		}

		if tag != nil {
			sub.Tags = append(sub.Tags, *tag)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	var result []domain.Subscription
	for _, sub := range subsMap {
		result = append(result, *sub)
	}

	return result, nil
}

func (subRepo *SubscriptionRepository) Delete(ctx context.Context, sub domain.Subscription) (domain.Subscription, error) {
	tx, err := subRepo.pool.Begin(ctx)
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	deleteSubQuery := `
		DELETE FROM subscriptions 
		WHERE chat_id = $1 AND link_id = $2
		RETURNING chat_id
	`

	var deletedChatId int64
	err = subRepo.pool.QueryRow(ctx, deleteSubQuery, sub.ChatID, sub.LinkID).Scan(&deletedChatId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Subscription{}, fmt.Errorf("subscription not found for chat %d and link %d", sub.ChatID, sub.LinkID)
		}
		return domain.Subscription{}, fmt.Errorf("failed to delete subscription: %w", err)
	}

	deleteLinkQuery := `
		DELETE FROM links 
		WHERE id = $1 AND NOT EXISTS (
			SELECT 1 FROM subscriptions WHERE link_id = $1
		)
	`
	_, err = tx.Exec(ctx, deleteLinkQuery, sub.LinkID)
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("failed to cleanup orphaned link: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Subscription{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return sub, nil
}
