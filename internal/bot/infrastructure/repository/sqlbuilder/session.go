package sqlbuilder

import (
	"context"
	"errors"
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type SessionRepository struct {
	pool *pgxpool.Pool
}

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{
		pool: pool,
	}
}

func (sessionRepo *SessionRepository) GetOrCreate(ctx context.Context, chatID int64) (domain.Session, error) {
	query, args, err := psql.Insert("sessions").
		Rows(goqu.Record{
			"chat_id": chatID,
			"state":   domain.StateIdle,
			"url":     "",
		}).
		OnConflict(goqu.DoUpdate("chat_id", goqu.Record{
			"chat_id": goqu.L("EXCLUDED.chat_id"),
		})).
		Returning("chat_id", "state", "url").
		ToSQL()

	if err != nil {
		return domain.Session{}, fmt.Errorf("failed to build get_or_create query: %w", err)
	}

	var session domain.Session
	err = sessionRepo.pool.QueryRow(ctx, query, args...).Scan(
		&session.ChatID,
		&session.State,
		&session.URL,
	)
	if err != nil {
		return domain.Session{}, fmt.Errorf("failed to get or create session: %w", err)
	}

	return session, nil
}

func (sessionRepo *SessionRepository) Save(ctx context.Context, session domain.Session) error {
	query, args, err := psql.Insert("sessions").
		Rows(goqu.Record{
			"chat_id": session.ChatID,
			"state":   session.State,
			"url":     session.URL,
		}).
		OnConflict(goqu.DoUpdate("chat_id", goqu.Record{
			"state": goqu.L("EXCLUDED.state"),
			"url":   goqu.L("EXCLUDED.url"),
		})).
		ToSQL()

	if err != nil {
		return fmt.Errorf("failed to build save query: %w", err)
	}

	_, err = sessionRepo.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}

func (sessionRepo *SessionRepository) Delete(ctx context.Context, chatID int64) (domain.Session, error) {
	query, args, err := psql.Delete("sessions").
		Where(goqu.Ex{
			"chat_id": chatID,
		}).
		Returning("chat_id", "state", "url").
		ToSQL()

	if err != nil {
		return domain.Session{}, fmt.Errorf("failed to build delete query: %w", err)
	}

	var session domain.Session
	err = sessionRepo.pool.QueryRow(ctx, query, args...).Scan(
		&session.ChatID,
		&session.State,
		&session.URL,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.NewSession(chatID), nil
		}
		return domain.Session{}, fmt.Errorf("failed to delete session: %w", err)
	}

	return session, nil
}
