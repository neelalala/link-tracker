package sqlbuilder

import (
	"context"
	"errors"
	"fmt"
	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
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
	return &SubscriptionRepository{pool: pool}
}

func (subRepo *SubscriptionRepository) Save(ctx context.Context, sub domain.Subscription) error {
	tx, err := subRepo.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(context.Background())

	subQuery, args, err := psql.Insert("subscriptions").
		Rows(goqu.Record{
			"chat_id": sub.ChatID,
			"link_id": sub.LinkID,
		}).
		ToSQL()
	if err != nil {
		return fmt.Errorf("failed to build sub query: %w", err)
	}

	_, err = tx.Exec(ctx, subQuery, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("%w: chat id = %d, link id = %d", domain.ErrAlreadySubscribed, sub.ChatID, sub.LinkID)
		}
		return fmt.Errorf("failed to insert subscription: %w", err)
	}

	if len(sub.Tags) > 0 {
		if err := insertTagsBatch(ctx, tx, sub.ChatID, sub.LinkID, sub.Tags, false); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (subRepo *SubscriptionRepository) GetByChatId(ctx context.Context, chatId int64) ([]domain.Subscription, error) {
	query, args, err := psql.Select("s.chat_id", "s.link_id", "st.tag").
		From(goqu.T("subscriptions").As("s")).
		LeftJoin(
			goqu.T("subscription_tags").As("st"),
			goqu.On(
				goqu.Ex{"s.chat_id": goqu.I("st.chat_id")},
				goqu.Ex{"s.link_id": goqu.I("st.link_id")},
			),
		).
		Where(goqu.Ex{"s.chat_id": chatId}).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	return subRepo.scanSubscriptions(ctx, query, args, func(chatID, linkID int64) int64 { return linkID })
}

func (subRepo *SubscriptionRepository) GetByLinkId(ctx context.Context, linkId int64) ([]domain.Subscription, error) {
	query, args, err := psql.Select("s.chat_id", "s.link_id", "st.tag").
		From(goqu.T("subscriptions").As("s")).
		LeftJoin(
			goqu.T("subscription_tags").As("st"),
			goqu.On(
				goqu.Ex{"s.chat_id": goqu.I("st.chat_id")},
				goqu.Ex{"s.link_id": goqu.I("st.link_id")},
			),
		).
		Where(goqu.Ex{"s.link_id": linkId}).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	return subRepo.scanSubscriptions(ctx, query, args, func(chatID, linkID int64) int64 { return chatID })
}

func (subRepo *SubscriptionRepository) scanSubscriptions(
	ctx context.Context,
	query string,
	args []interface{},
	keyFn func(chatID, linkID int64) int64,
) ([]domain.Subscription, error) {
	rows, err := subRepo.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscriptions: %w", err)
	}
	defer rows.Close()

	subsMap := make(map[int64]*domain.Subscription)

	for rows.Next() {
		var chatID, linkID int64
		var tag *string

		if err := rows.Scan(&chatID, &linkID, &tag); err != nil {
			return nil, fmt.Errorf("failed to scan subscription row: %w", err)
		}

		key := keyFn(chatID, linkID)
		sub, exists := subsMap[key]
		if !exists {
			sub = &domain.Subscription{
				ChatID: chatID,
				LinkID: linkID,
				Tags:   make([]string, 0),
			}
			subsMap[key] = sub
		}

		if tag != nil {
			sub.Tags = append(sub.Tags, *tag)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	result := make([]domain.Subscription, 0, len(subsMap))
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
	defer tx.Rollback(context.Background())

	deleteSubQuery, args, err := psql.Delete("subscriptions").
		Where(goqu.Ex{
			"chat_id": sub.ChatID,
			"link_id": sub.LinkID,
		}).
		ToSQL()
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("failed to build delete sub query: %w", err)
	}

	ct, err := tx.Exec(ctx, deleteSubQuery, args...)
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("failed to delete subscription: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return domain.Subscription{}, fmt.Errorf("%w: link id = %d, chat id = %d", domain.ErrNotSubscribed, sub.LinkID, sub.ChatID)
	}

	deleteLinkQuery, linkArgs, err := psql.Delete("links").
		Where(
			goqu.Ex{"id": sub.LinkID},
			goqu.L("NOT EXISTS (SELECT 1 FROM subscriptions WHERE link_id = ?)", sub.LinkID),
		).
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

func (subRepo *SubscriptionRepository) AddTags(ctx context.Context, linkId, chatId int64, tags []string) error {
	if len(tags) == 0 {
		return nil
	}

	tx, err := subRepo.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(context.Background())

	if err := insertTagsBatch(ctx, tx, chatId, linkId, tags, true); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (subRepo *SubscriptionRepository) GetTags(ctx context.Context, linkId, chatId int64) ([]string, error) {
	query, args, err := psql.From("subscription_tags").
		Select("tag").
		Where(goqu.Ex{
			"chat_id": chatId,
			"link_id": linkId,
		}).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("failed to build get tags SQL for link %d: %w", linkId, err)
	}

	rows, err := subRepo.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags for link %d: %w", linkId, err)
	}
	defer rows.Close()

	tags := make([]string, 0)
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("failed to scan tag row: %w", err)
		}
		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return tags, nil
}

func (subRepo *SubscriptionRepository) DeleteTags(ctx context.Context, linkId, chatId int64, tags []string) error {
	if len(tags) == 0 {
		return nil
	}

	tx, err := subRepo.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(context.Background())

	batch := &pgx.Batch{}
	for _, tag := range tags {
		q, args, err := psql.Delete("subscription_tags").
			Where(goqu.Ex{
				"chat_id": chatId,
				"link_id": linkId,
				"tag":     tag,
			}).
			ToSQL()
		if err != nil {
			return fmt.Errorf("failed to build delete tag query for %s: %w", tag, err)
		}
		batch.Queue(q, args...)
	}

	br := tx.SendBatch(ctx, batch)
	for _, tag := range tags {
		if _, err := br.Exec(); err != nil {
			br.Close()
			return fmt.Errorf("failed to delete tag %s: %w", tag, err)
		}
	}

	if err := br.Close(); err != nil {
		return fmt.Errorf("failed to close batch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func insertTagsBatch(ctx context.Context, tx pgx.Tx, chatId, linkId int64, tags []string, onConflictDoNothing bool) error {
	batch := &pgx.Batch{}

	for _, tag := range tags {
		ds := psql.Insert("subscription_tags").
			Rows(goqu.Record{
				"chat_id": chatId,
				"link_id": linkId,
				"tag":     tag,
			})

		if onConflictDoNothing {
			ds = ds.OnConflict(goqu.DoNothing())
		}

		q, args, err := ds.ToSQL()
		if err != nil {
			return fmt.Errorf("failed to build insert tag query for %s: %w", tag, err)
		}
		batch.Queue(q, args...)
	}

	br := tx.SendBatch(ctx, batch)
	for _, tag := range tags {
		if _, err := br.Exec(); err != nil {
			br.Close()
			return fmt.Errorf("failed to insert tag %s: %w", tag, err)
		}
	}

	if err := br.Close(); err != nil {
		return fmt.Errorf("failed to close batch: %w", err)
	}

	return nil
}
