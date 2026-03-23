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

	subQuery, subArgs, err := psql.Insert("subscriptions").
		Rows(goqu.Record{
			"chat_id": sub.ChatID,
			"link_id": sub.LinkID,
		}).
		ToSQL()
	if err != nil {
		return fmt.Errorf("failed to build sub query: %w", err)
	}

	_, err = tx.Exec(ctx, subQuery, subArgs...)
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
		for _, tag := range sub.Tags {
			tagQuery, tagArgs, err := psql.Insert("subscription_tags").
				Rows(goqu.Record{
					"chat_id": sub.ChatID,
					"link_id": sub.LinkID,
					"tag":     tag,
				}).
				ToSQL()
			if err != nil {
				return fmt.Errorf("failed to build tag query: %w", err)
			}

			_, err = tx.Exec(ctx, tagQuery, tagArgs...)
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
	query, args, err := psql.Select(
		goqu.I("subRepo.chat_id"),
		goqu.I("l.id"),
		goqu.I("st.tag"),
	).From(goqu.T("subscriptions").As("subRepo")).
		Join(
			goqu.T("links").As("l"),
			goqu.On(goqu.I("subRepo.link_id").Eq(goqu.I("l.id"))),
		).
		LeftJoin(
			goqu.T("subscription_tags").As("st"),
			goqu.On(
				goqu.I("subRepo.chat_id").Eq(goqu.I("st.chat_id")),
				goqu.I("subRepo.link_id").Eq(goqu.I("st.link_id")),
			),
		).
		Where(goqu.Ex{"subRepo.chat_id": chatId}).
		ToSQL()

	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := subRepo.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscriptions for chat %d: %w", chatId, err)
	}
	defer rows.Close()

	subsMap := make(map[int64]*domain.Subscription)
	for rows.Next() {
		var subChatID, linkID int64
		var tag *string

		if err := rows.Scan(&subChatID, &linkID, &tag); err != nil {
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
	query, args, err := psql.Select(
		goqu.I("subRepo.chat_id"),
		goqu.I("l.id"),
		goqu.I("st.tag"),
	).From(goqu.T("subscriptions").As("subRepo")).
		Join(
			goqu.T("links").As("l"),
			goqu.On(goqu.I("subRepo.link_id").Eq(goqu.I("l.id"))),
		).
		LeftJoin(
			goqu.T("subscription_tags").As("st"),
			goqu.On(
				goqu.I("subRepo.chat_id").Eq(goqu.I("st.chat_id")),
				goqu.I("subRepo.link_id").Eq(goqu.I("st.link_id")),
			),
		).
		Where(goqu.Ex{"subRepo.link_id": linkId}).
		ToSQL()

	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := subRepo.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscriptions for link %d: %w", linkId, err)
	}
	defer rows.Close()

	subsMap := make(map[int64]*domain.Subscription)
	for rows.Next() {
		var subChatID, linkID int64
		var tag *string

		if err := rows.Scan(&subChatID, &linkID, &tag); err != nil {
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

	// 1. Удаляем подписку (используем tx.QueryRow!)
	deleteSubQuery, subArgs, err := psql.Delete("subscriptions").
		Where(goqu.Ex{"chat_id": sub.ChatID, "link_id": sub.LinkID}).
		Returning("chat_id").
		ToSQL()
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("failed to build delete sub query: %w", err)
	}

	var deletedChatId int64
	err = tx.QueryRow(ctx, deleteSubQuery, subArgs...).Scan(&deletedChatId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Subscription{}, fmt.Errorf("subscription not found for chat %d and link %d", sub.ChatID, sub.LinkID)
		}
		return domain.Subscription{}, fmt.Errorf("failed to delete subscription: %w", err)
	}

	deleteLinkQuery, linkArgs, err := psql.Delete("links").
		Where(goqu.Ex{"id": sub.LinkID}).
		Where(goqu.L("NOT EXISTS (SELECT 1 FROM subscriptions WHERE link_id = ?)", sub.LinkID)).
		ToSQL()
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("failed to build delete link query: %w", err)
	}

	_, err = tx.Exec(ctx, deleteLinkQuery, linkArgs...)
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("failed to cleanup orphaned link: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Subscription{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return sub, nil
}
