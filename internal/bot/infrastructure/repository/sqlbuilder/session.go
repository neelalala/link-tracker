package sqlbuilder

import (
	"context"
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
	panic("implement me")
}

func (sessionRepo *SessionRepository) Save(ctx context.Context, session domain.Session) error {
	panic("implement me")
}

func (sessionRepo *SessionRepository) Delete(ctx context.Context, chatID int64) (domain.Session, error) {
	panic("implement me")
}
