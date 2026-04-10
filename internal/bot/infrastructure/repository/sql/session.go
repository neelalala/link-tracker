package sql

import (
	"context"
	"errors"
	"fmt"

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
	query := `
		INSERT INTO sessions (chat_id, state, url)
		VALUES ($1, $2, '')
		ON CONFLICT (chat_id) DO UPDATE
		SET 
		    chat_id = EXCLUDED.chat_id 
			RETURNING chat_id, state, url 
	`

	var session domain.Session
	err := sessionRepo.pool.QueryRow(ctx, query, chatID, domain.StateIdle).Scan(
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
	query := `
		INSERT INTO sessions (chat_id, state, url)
		VALUES ($1, $2, $3)
		ON CONFLICT (chat_id) DO UPDATE 
		SET 
			state = EXCLUDED.state,
			url = EXCLUDED.url
	`

	_, err := sessionRepo.pool.Exec(ctx, query, session.ChatID, session.State, session.URL)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}

func (sessionRepo *SessionRepository) Delete(ctx context.Context, chatID int64) (domain.Session, error) {
	query := `
		DELETE FROM sessions
		WHERE chat_id = $1
		RETURNING chat_id, state, url;
	`

	var session domain.Session

	err := sessionRepo.pool.QueryRow(ctx, query, chatID).Scan(
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
