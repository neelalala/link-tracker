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
	defer tx.Rollback(context.Background())

	subQuery := `
		INSERT INTO subscriptions (chat_id, link_id)
		VALUES ($1, $2);
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

	batch := &pgx.Batch{}

	if len(sub.Tags) > 0 {
		tagQuery := `
			INSERT INTO subscription_tags (chat_id, link_id, tag)
			VALUES ($1, $2, $3);
		`

		for _, tag := range sub.Tags {
			batch.Queue(tagQuery, sub.ChatID, sub.LinkID, tag)
		}

		br := tx.SendBatch(ctx, batch)

		for i := 0; i < len(sub.Tags); i++ {
			_, err := br.Exec()
			if err != nil {
				br.Close()
				return fmt.Errorf("failed to insert tag %s: %w", sub.Tags[i], err)
			}
		}

		err = br.Close()
		if err != nil {
			return fmt.Errorf("failed to close batch: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (subRepo *SubscriptionRepository) GetByChatID(ctx context.Context, chatID int64) ([]domain.Subscription, error) {
	query := `
		SELECT 
			s.chat_id, 
			s.link_id,
			st.tag
		FROM subscriptions s
		LEFT JOIN subscription_tags st ON s.chat_id = st.chat_id AND s.link_id = st.link_id
		WHERE s.chat_id = $1
		ORDER BY s.link_id ASC;
	`

	rows, err := subRepo.pool.Query(ctx, query, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscriptions for chat %d: %w", chatID, err)
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

func (subRepo *SubscriptionRepository) GetByLinkID(ctx context.Context, linkID int64) ([]domain.Subscription, error) {
	query := `
		SELECT 
			s.chat_id, 
			s.link_id, 
			st.tag
		FROM subscriptions s
		LEFT JOIN subscription_tags st ON s.chat_id = st.chat_id AND s.link_id = st.link_id
		WHERE s.link_id = $1
		ORDER BY s.chat_id ASC;
	`

	rows, err := subRepo.pool.Query(ctx, query, linkID)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscriptions for link %d: %w", linkID, err)
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

func (subRepo *SubscriptionRepository) Exists(ctx context.Context, chatID int64, linkID int64) (bool, error) {
	query := `
       SELECT EXISTS (
           SELECT 1
           FROM subscriptions
           WHERE chat_id = $1 AND link_id = $2
       );
    `

	var exists bool
	err := subRepo.pool.QueryRow(ctx, query, chatID, linkID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check subscription for link %d in chat %d: %w", linkID, chatID, err)
	}

	return exists, nil
}

func (subRepo *SubscriptionRepository) Delete(ctx context.Context, chatID int64, linkID int64) (domain.Subscription, error) {
	query := `
		DELETE FROM subscriptions 
		WHERE chat_id = $1 AND link_id = $2;
	`

	ct, err := subRepo.pool.Exec(ctx, query, chatID, linkID)
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("failed to delete subscription: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return domain.Subscription{}, fmt.Errorf(
			"%w: link id = %d, chat id = %d",
			domain.ErrNotSubscribed,
			linkID,
			chatID)
	}

	sub := domain.Subscription{
		ChatID: chatID,
		LinkID: linkID,
	}

	return sub, nil
}

func (subRepo *SubscriptionRepository) AddTags(ctx context.Context, linkID, chatID int64, tags []string) error {
	if len(tags) == 0 {
		return nil
	}

	tx, err := subRepo.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(context.Background())

	batch := &pgx.Batch{}

	query := `
		INSERT INTO subscription_tags (chat_id, link_id, tag)
		VALUES ($1, $2, $3)
		ON CONFLICT (chat_id, link_id, tag) DO NOTHING;
	`
	for _, tag := range tags {
		batch.Queue(query, chatID, linkID, tag)
	}

	br := tx.SendBatch(ctx, batch)

	for i := 0; i < len(tags); i++ {
		_, err := br.Exec()
		if err != nil {
			br.Close()
			return fmt.Errorf("failed to insert tag %s: %w", tags[i], err)
		}
	}

	err = br.Close()
	if err != nil {
		return fmt.Errorf("failed to close batch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (subRepo *SubscriptionRepository) GetTags(ctx context.Context, linkID, chatID int64) ([]string, error) {
	query := `
		SELECT tag
		FROM subscription_tags
		WHERE chat_id = $1 AND link_id = $2;
	`
	rows, err := subRepo.pool.Query(ctx, query, chatID, linkID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags for link %d: %w", linkID, err)
	}
	defer rows.Close()

	tags := make([]string, 0)
	for rows.Next() {
		var tag string
		err := rows.Scan(&tag)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tags for link %d: %w", linkID, err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return tags, nil
}

func (subRepo *SubscriptionRepository) DeleteTags(ctx context.Context, linkID, chatID int64, tags []string) error {
	if len(tags) == 0 {
		return nil
	}

	tx, err := subRepo.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(context.Background())

	batch := &pgx.Batch{}
	query := `
		DELETE FROM subscription_tags
		WHERE chat_id = $1 AND link_id = $2 AND tag = $3;
	`

	for _, tag := range tags {
		batch.Queue(query, chatID, linkID, tag)
	}

	br := tx.SendBatch(ctx, batch)
	for i := 0; i < len(tags); i++ {
		_, err := br.Exec()
		if err != nil {
			br.Close()
			return fmt.Errorf("failed to delete tag %s: %w", tags[i], err)
		}
	}

	err = br.Close()
	if err != nil {
		return fmt.Errorf("failed to close batch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
