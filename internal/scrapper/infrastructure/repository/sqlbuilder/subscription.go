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
	return &SubscriptionRepository{pool: pool}
}

func (subRepo *SubscriptionRepository) Save(ctx context.Context, sub domain.Subscription) error {
	subQuery, args, err := psql.Insert("subscriptions").
		Rows(goqu.Record{
			"chat_id": sub.ChatID,
			"link_id": sub.LinkID,
		}).
		ToSQL()
	if err != nil {
		return fmt.Errorf("failed to build sub query: %w", err)
	}

	db := GetDB(ctx, subRepo.pool)

	_, err = db.Exec(ctx, subQuery, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("%w: chat id = %d, link id = %d", domain.ErrAlreadySubscribed, sub.ChatID, sub.LinkID)
		}
		return fmt.Errorf("failed to insert subscription: %w", err)
	}

	if len(sub.Tags) > 0 {
		if err := insertTagsBatch(ctx, db, sub.ChatID, sub.LinkID, sub.Tags, false); err != nil {
			return err
		}
	}

	return nil
}

func (subRepo *SubscriptionRepository) GetByChatID(ctx context.Context, chatID int64) ([]domain.Subscription, error) {
	query, args, err := psql.Select("s.chat_id", "s.link_id", "st.tag").
		From(goqu.T("subscriptions").As("s")).
		LeftJoin(
			goqu.T("subscription_tags").As("st"),
			goqu.On(
				goqu.Ex{"s.chat_id": goqu.I("st.chat_id")},
				goqu.Ex{"s.link_id": goqu.I("st.link_id")},
			),
		).
		Where(goqu.Ex{"s.chat_id": chatID}).
		Order(goqu.I("s.link_id").Asc()).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	return subRepo.scanSubscriptions(ctx, query, args, func(chatID, linkID int64) int64 { return linkID })
}

func (subRepo *SubscriptionRepository) GetByLinkID(ctx context.Context, linkID int64) ([]domain.Subscription, error) {
	query, args, err := psql.Select("s.chat_id", "s.link_id", "st.tag").
		From(goqu.T("subscriptions").As("s")).
		LeftJoin(
			goqu.T("subscription_tags").As("st"),
			goqu.On(
				goqu.Ex{"s.chat_id": goqu.I("st.chat_id")},
				goqu.Ex{"s.link_id": goqu.I("st.link_id")},
			),
		).
		Where(goqu.Ex{"s.link_id": linkID}).
		Order(goqu.I("s.chat_id").Asc()).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	return subRepo.scanSubscriptions(ctx, query, args, func(chatID, linkID int64) int64 { return chatID })
}

func (subRepo *SubscriptionRepository) Exists(ctx context.Context, chatID int64, linkID int64) (bool, error) {
	subquery := psql.Select(goqu.L("1")).
		From(goqu.T("subscriptions").As("s")).
		Where(goqu.Ex{
			"s.chat_id": chatID,
			"s.link_id": linkID,
		})

	query, args, err := psql.Select(goqu.L("EXISTS ?", subquery)).ToSQL()
	if err != nil {
		return false, fmt.Errorf("failed to build exists query: %w", err)
	}

	db := GetDB(ctx, subRepo.pool)

	var exists bool
	err = db.QueryRow(ctx, query, args...).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check subscription for link %d in chat %d: %w", linkID, chatID, err)
	}

	return exists, nil
}

func (subRepo *SubscriptionRepository) scanSubscriptions(
	ctx context.Context,
	query string,
	args []any,
	keyFn func(chatID, linkID int64) int64,
) ([]domain.Subscription, error) {
	db := GetDB(ctx, subRepo.pool)

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscriptions: %w", err)
	}
	defer rows.Close()

	subsMap := make(map[int64]*domain.Subscription)
	var orderedKeys []int64

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
			orderedKeys = append(orderedKeys, key)
		}

		if tag != nil {
			sub.Tags = append(sub.Tags, *tag)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	result := make([]domain.Subscription, 0, len(subsMap))
	for _, key := range orderedKeys {
		result = append(result, *subsMap[key])
	}

	return result, nil
}

func (subRepo *SubscriptionRepository) Delete(ctx context.Context, chatID int64, linkID int64) (domain.Subscription, error) {
	deleteSubQuery, args, err := psql.Delete("subscriptions").
		Where(goqu.Ex{
			"chat_id": chatID,
			"link_id": linkID,
		}).
		ToSQL()
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("failed to build delete sub query: %w", err)
	}

	db := GetDB(ctx, subRepo.pool)

	ct, err := db.Exec(ctx, deleteSubQuery, args...)
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("failed to delete subscription: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return domain.Subscription{}, fmt.Errorf("%w: link id = %d, chat id = %d", domain.ErrNotSubscribed, linkID, chatID)
	}

	deleteLinkQuery, linkArgs, err := psql.Delete("links").
		Where(
			goqu.Ex{"id": linkID},
			goqu.L("NOT EXISTS (SELECT 1 FROM subscriptions WHERE link_id = ?)", linkID),
		).
		ToSQL()
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("failed to build delete link query: %w", err)
	}

	_, err = db.Exec(ctx, deleteLinkQuery, linkArgs...)
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("failed to cleanup orphaned link: %w", err)
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

	db := GetDB(ctx, subRepo.pool)

	if err := insertTagsBatch(ctx, db, chatID, linkID, tags, true); err != nil {
		return err
	}

	return nil
}

func (subRepo *SubscriptionRepository) GetTags(ctx context.Context, linkID, chatID int64) ([]string, error) {
	query, args, err := psql.From("subscription_tags").
		Select("tag").
		Where(goqu.Ex{
			"chat_id": chatID,
			"link_id": linkID,
		}).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("failed to build get tags SQL for link %d: %w", linkID, err)
	}

	db := GetDB(ctx, subRepo.pool)

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags for link %d: %w", linkID, err)
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

func (subRepo *SubscriptionRepository) DeleteTags(ctx context.Context, linkID, chatID int64, tags []string) error {
	if len(tags) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, tag := range tags {
		q, args, err := psql.Delete("subscription_tags").
			Where(goqu.Ex{
				"chat_id": chatID,
				"link_id": linkID,
				"tag":     tag,
			}).
			ToSQL()
		if err != nil {
			return fmt.Errorf("failed to build delete tag query for %s: %w", tag, err)
		}
		batch.Queue(q, args...)
	}

	db := GetDB(ctx, subRepo.pool)

	br := db.SendBatch(ctx, batch)
	for _, tag := range tags {
		if _, err := br.Exec(); err != nil {
			br.Close()
			return fmt.Errorf("failed to delete tag %s: %w", tag, err)
		}
	}

	if err := br.Close(); err != nil {
		return fmt.Errorf("failed to close batch: %w", err)
	}

	return nil
}

func insertTagsBatch(ctx context.Context, db DB, chatID, linkID int64, tags []string, onConflictDoNothing bool) error {
	batch := &pgx.Batch{}

	for _, tag := range tags {
		ds := psql.Insert("subscription_tags").
			Rows(goqu.Record{
				"chat_id": chatID,
				"link_id": linkID,
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

	br := db.SendBatch(ctx, batch)
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
