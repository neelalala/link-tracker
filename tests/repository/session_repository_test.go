package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/repository/sql"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/repository/sqlbuilder"
)

func TestSessionRepository_Integration(t *testing.T) {
	ctx := context.Background()

	const (
		username   = "testuser"
		password   = "testpass"
		database   = "bot_test"
		migrations = "file://../../migrations"
	)

	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     username,
			"POSTGRES_PASSWORD": password,
			"POSTGRES_DB":       database,
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(30 * time.Second),
	}

	pgContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoErrorf(t, err, "Failed to start PostgreSQL container: %v", err)
	defer pgContainer.Terminate(ctx)

	host, err := pgContainer.Host(ctx)
	require.NoErrorf(t, err, "Failed to get container host: %v", err)

	port, err := pgContainer.MappedPort(ctx, "5432/tcp")
	require.NoErrorf(t, err, "Failed to get mapped port: %v", err)

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", username, password, host, port.Port(), database)

	m, err := migrate.New(migrations, dbURL)
	require.NoErrorf(t, err, "Failed to create migrate instance: %v", err)

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		require.NoErrorf(t, err, "Failed to apply migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, dbURL)
	require.NoErrorf(t, err, "Failed to connect to database: %v", err)
	defer pool.Close()

	implementations := map[string]domain.SessionRepository{
		"BUILDER": sqlbuilder.NewSessionRepository(pool),
		"SQL":     sql.NewSessionRepository(pool),
	}

	for accessType, repo := range implementations {
		t.Run(fmt.Sprintf("AccessType: %s", accessType), func(t *testing.T) {
			_, err := pool.Exec(ctx, "TRUNCATE TABLE chats, sessions RESTART IDENTITY CASCADE")
			require.NoErrorf(t, err, "Failed to truncate tables before tests: %v", err)

			createTestChat := func(chatID int64) {
				_, err := pool.Exec(ctx, "INSERT INTO chats (id) VALUES ($1) ON CONFLICT DO NOTHING", chatID)
				require.NoErrorf(t, err, "Failed to insert test chat: %v", err)
			}

			t.Run("GetOrCreate new session", func(t *testing.T) {
				chatID := int64(1)
				createTestChat(chatID)

				session, err := repo.GetOrCreate(ctx, chatID)
				require.NoErrorf(t, err, "Failed to get or create new session: %v", err)

				assert.Equalf(t, chatID, session.ChatID, "Expected chat ID %d, got %d", chatID, session.ChatID)
				assert.Equalf(t, domain.StateIdle, session.State, "Expected state %s, got %s", domain.StateIdle, session.State)
				assert.Emptyf(t, session.URL, "Expected empty URL, got %s", session.URL)
			})

			t.Run("GetOrCreate existing session", func(t *testing.T) {
				chatID := int64(2)
				createTestChat(chatID)

				initialSession := domain.Session{
					ChatID: chatID,
					State:  domain.StateWaitingForURLTrack,
					URL:    "https://github.com/user",
				}

				err := repo.Save(ctx, initialSession)
				require.NoErrorf(t, err, "Failed to save initial session setup: %v", err)

				session, err := repo.GetOrCreate(ctx, chatID)
				require.NoErrorf(t, err, "Failed to get existing session: %v", err)

				assert.Equalf(t, chatID, session.ChatID, "Expected chat ID %d, got %d", chatID, session.ChatID)
				assert.Equalf(t, domain.StateWaitingForURLTrack, session.State, "Expected state %s, got %s", domain.StateWaitingForURLTrack, session.State)
				assert.Equalf(t, "https://github.com/user", session.URL, "Expected URL %s, got %s", "https://github.com/user", session.URL)
			})

			t.Run("Save new session directly", func(t *testing.T) {
				chatID := int64(3)
				createTestChat(chatID)

				newSession := domain.Session{
					ChatID: chatID,
					State:  domain.StateWaitingForTags,
					URL:    "https://github.com/test",
				}

				err := repo.Save(ctx, newSession)
				require.NoErrorf(t, err, "Failed to save new session: %v", err)

				dbSession, err := repo.GetOrCreate(ctx, chatID)
				require.NoErrorf(t, err, "Failed to get saved session: %v", err)

				assert.Equalf(t, domain.StateWaitingForTags, dbSession.State, "Expected state %s, got %s", domain.StateWaitingForTags, dbSession.State)
				assert.Equalf(t, "https://github.com/test", dbSession.URL, "Expected URL %s, got %s", "https://github.com/test", dbSession.URL)
			})

			t.Run("Save existing session update", func(t *testing.T) {
				chatID := int64(4)
				createTestChat(chatID)

				session, err := repo.GetOrCreate(ctx, chatID)
				require.NoErrorf(t, err, "Failed to get or create session: %v", err)

				session.State = domain.StateWaitingForURLUntrack
				session.URL = "https://github.com/remove"

				err = repo.Save(ctx, session)
				require.NoErrorf(t, err, "Failed to update existing session: %v", err)

				updatedSession, err := repo.GetOrCreate(ctx, chatID)
				require.NoErrorf(t, err, "Failed to get updated session: %v", err)

				assert.Equalf(t, domain.StateWaitingForURLUntrack, updatedSession.State, "Expected updated state %s, got %s", domain.StateWaitingForURLUntrack, updatedSession.State)
				assert.Equalf(t, "https://github.com/remove", updatedSession.URL, "Expected updated URL %s, got %s", "https://github.com/remove", updatedSession.URL)
			})

			t.Run("Delete existing session", func(t *testing.T) {
				chatID := int64(5)
				createTestChat(chatID)

				sessionToSave := domain.Session{
					ChatID: chatID,
					State:  domain.StateWaitingForURLTrack,
					URL:    "https://github.com/delete-me",
				}

				err := repo.Save(ctx, sessionToSave)
				require.NoErrorf(t, err, "Failed to setup session for deletion: %v", err)

				deletedSession, err := repo.Delete(ctx, chatID)
				require.NoErrorf(t, err, "Failed to delete session: %v", err)

				assert.Equalf(t, chatID, deletedSession.ChatID, "Expected chat ID %d, got %d", chatID, deletedSession.ChatID)
				assert.Equalf(t, domain.StateWaitingForURLTrack, deletedSession.State, "Expected state %s, got %s", domain.StateWaitingForURLTrack, deletedSession.State)
				assert.Equalf(t, "https://github.com/delete-me", deletedSession.URL, "Expected URL %s, got %s", "https://github.com/delete-me", deletedSession.URL)

				freshSession, err := repo.GetOrCreate(ctx, chatID)
				require.NoErrorf(t, err, "Failed to GetOrCreate after deletion: %v", err)
				assert.Equalf(t, domain.StateIdle, freshSession.State, "Expected state %s after deletion, got %s", domain.StateIdle, freshSession.State)
				assert.Emptyf(t, freshSession.URL, "Expected empty URL after deletion, got %s", freshSession.URL)
			})

			t.Run("Delete non-existent session", func(t *testing.T) {
				chatID := int64(6)
				createTestChat(chatID)

				_, err := repo.Delete(ctx, chatID)
				require.True(t, errors.Is(err, domain.ErrSessionNotFound), "Expected no error when deleting non-existent session, got: %v", err)
			})
		})
	}
}
