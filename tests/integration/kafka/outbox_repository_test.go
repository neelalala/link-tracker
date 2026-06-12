package kafka

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/sql"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/sqlbuilder"
)

func TestOutboxRepository_Integration(t *testing.T) {
	ctx := context.Background()

	const (
		username   = "testuser"
		password   = "testpass"
		database   = "outbox_test"
		migrations = "file://../../../migrations"
	)

	req := testcontainers.ContainerRequest{
		Image:        "postgres:17-alpine",
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

	implementations := map[string]domain.OutboxRepository{
		"BUILDER": sqlbuilder.NewOutboxRepository(pool),
		"SQL":     sql.NewOutboxRepository(pool),
	}

	topic := "test-outbox-topic"
	payload := `{"test": "payload"}`

	for accessType, repo := range implementations {
		t.Run(fmt.Sprintf("AccessType: %s", accessType), func(t *testing.T) {
			_, err := pool.Exec(ctx, "TRUNCATE TABLE outbox CASCADE")
			require.NoErrorf(t, err, "Failed to truncate tables before tests: %v", err)

			saved, err := repo.Add(ctx, topic, []byte(payload))
			require.NoErrorf(t, err, "Failed to add outbox: %v", err)

			assert.Equalf(t, topic, saved.Topic, "Expected topic %s, got %s", topic, saved.Topic)
			assert.Equalf(t, payload, string(saved.Payload), "Expected payload %s, got %s", payload, saved.Payload)

			pending, err := repo.GetPending(ctx, 1)
			require.NoErrorf(t, err, "Failed to get saved outbox")
			require.Len(t, pending, 1)
			assert.Equalf(t, saved.Topic, pending[0].Topic, "Expected topic %s, got %s", saved.Topic, pending[0].Topic)
			assert.Equalf(t, string(saved.Payload), string(pending[0].Payload), "Expected payload %s, got %s", saved.Payload, pending[0].Payload)

			err = repo.UpdateStatus(ctx, pending[0].ID, domain.OutboxStatusFailed)
			assert.NoErrorf(t, err, "Failed to update status: %v", err)

			failed, err := repo.GetFailed(ctx, 1)
			require.NoErrorf(t, err, "Failed to get saved outbox")
			require.Len(t, failed, 1)
			assert.Equalf(t, saved.Topic, failed[0].Topic, "Expected topic %s, got %s", saved.Topic, failed[0].Topic)
			assert.Equalf(t, string(saved.Payload), string(pending[0].Payload), "Expected payload %s, got %s", saved.Payload, failed[0].Payload)
		})
	}
}
