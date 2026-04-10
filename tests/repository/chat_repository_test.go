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
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/sql"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/sqlbuilder"
)

func TestChatRepository_Integration(t *testing.T) {
	ctx := context.Background()

	const (
		username   = "testuser"
		password   = "testpass"
		database   = "scrapper_test"
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

	implementations := map[string]domain.ChatRepository{
		"BUILDER": sqlbuilder.NewChatRepository(pool),
		"SQL":     sql.NewChatRepository(pool),
	}

	for accessType, repo := range implementations {
		t.Run(fmt.Sprintf("AccessType: %s", accessType), func(t *testing.T) {
			_, err := pool.Exec(ctx, "TRUNCATE TABLE chats CASCADE")
			require.NoErrorf(t, err, "Failed to truncate tables before tests: %v", err)

			t.Run("Create chat", func(t *testing.T) {
				chat := domain.Chat{ID: 1}

				err := repo.Create(ctx, chat)
				require.NoErrorf(t, err, "Failed to create chat: %v", err)

				dbChat, err := repo.GetById(ctx, chat.ID)
				require.NoErrorf(t, err, "Failed to get saved chat by ID %d: %v", chat.ID, err)
				assert.Equalf(t, chat.ID, dbChat.ID, "Expected chat ID %d, got %d", chat.ID, dbChat.ID)
			})

			t.Run("Get non-existent chat", func(t *testing.T) {
				_, err := repo.GetById(ctx, 999)
				require.Errorf(t, err, "Expected error when getting non-existent chat, got nil")
				assert.Truef(t, errors.Is(err, domain.ErrChatNotRegistered), "Expected ErrChatNotRegistered, got %v", err)
			})

			t.Run("Delete chat", func(t *testing.T) {
				chat := domain.Chat{ID: 2}

				err := repo.Create(ctx, chat)
				require.NoErrorf(t, err, "Failed to create chat before deletion: %v", err)

				err = repo.Delete(ctx, chat)
				require.NoErrorf(t, err, "Failed to delete chat: %v", err)

				_, err = repo.GetById(ctx, chat.ID)
				require.Errorf(t, err, "Expected error when getting deleted chat, got nil")
				assert.Truef(t, errors.Is(err, domain.ErrChatNotRegistered), "Expected ErrChatNotRegistered, got %v", err)
			})

			t.Run("Create duplicate chat", func(t *testing.T) {
				chat := domain.Chat{ID: 3}

				err := repo.Create(ctx, chat)
				require.NoErrorf(t, err, "Failed to create initial chat: %v", err)

				err = repo.Create(ctx, chat)
				require.Errorf(t, err, "Expected error on duplicate create, got: %v", err)
			})
		})
	}
}
