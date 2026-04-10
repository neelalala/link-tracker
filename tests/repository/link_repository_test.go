package integration

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

func TestLinkRepository_Integration(t *testing.T) {
	ctx := context.Background()

	const (
		username   = "testuser"
		password   = "testpass"
		database   = "scrapper_test"
		migrations = "file://../../migrations"
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

	implementations := map[string]domain.LinkRepository{
		"BUILDER": sqlbuilder.NewLinkRepository(pool),
		"SQL":     sql.NewLinkRepository(pool),
	}

	for accessType, repo := range implementations {
		t.Run(fmt.Sprintf("AccessType: %s", accessType), func(t *testing.T) {
			_, err := pool.Exec(ctx, "TRUNCATE TABLE links RESTART IDENTITY CASCADE")
			require.NoErrorf(t, err, "Failed to truncate tables before tests: %v", err)

			t.Run("Add link", func(t *testing.T) {
				link := domain.Link{URL: "https://github.com/user/repo1", LastUpdated: time.Now().UTC().Truncate(time.Microsecond)}

				saved, err := repo.Save(ctx, link)
				require.NoErrorf(t, err, "Failed to save link: %v", err)

				assert.NotZerof(t, saved.ID, "Expected generated ID to be non-zero, got %d", saved.ID)
				assert.Equalf(t, link.URL, saved.URL, "Expected URL %s, got %s", link.URL, saved.URL)

				dbLink, err := repo.GetById(ctx, saved.ID)
				require.NoErrorf(t, err, "Failed to get saved link by ID %d: %v", saved.ID, err)
				assert.Equalf(t, saved.URL, dbLink.URL, "Expected db URL %s to match saved URL %s", dbLink.URL, saved.URL)
			})

			t.Run("Save existing link", func(t *testing.T) {
				link := domain.Link{URL: "https://github.com/user/repo2", LastUpdated: time.Now().UTC().Truncate(time.Microsecond)}

				_, err := repo.Save(ctx, link)
				require.NoErrorf(t, err, "Failed to save initial link: %v", err)

				newTime := time.Now().UTC().Add(10 * time.Minute).Truncate(time.Microsecond)
				link.LastUpdated = newTime

				updated, err := repo.Save(ctx, link)
				require.NoErrorf(t, err, "Failed to update duplicate link: %v", err)

				assert.WithinDurationf(t, newTime, updated.LastUpdated, 0, "Expected LastUpdated to change to %v, got %v", newTime, updated.LastUpdated)
			})

			t.Run("Get link by URL", func(t *testing.T) {
				link := domain.Link{URL: "https://github.com/user/repo-by-url", LastUpdated: time.Now().UTC().Truncate(time.Microsecond)}
				saved, err := repo.Save(ctx, link)
				require.NoErrorf(t, err, "Failed to save link: %v", err)

				dbLink, err := repo.GetByUrl(ctx, saved.URL)
				require.NoErrorf(t, err, "Failed to get saved link by URL %s: %v", saved.URL, err)
				assert.Equalf(t, saved.ID, dbLink.ID, "Expected db ID %d to match saved ID %d", dbLink.ID, saved.ID)

				_, err = repo.GetByUrl(ctx, "https://non-existent-url.com")
				require.Errorf(t, err, "Expected error when getting non-existent link by URL, got nil")
				assert.Truef(t, errors.Is(err, domain.ErrLinkNotFound), "Expected ErrLinkNotFound, got %v", err)
			})

			t.Run("Delete link", func(t *testing.T) {
				link := domain.Link{URL: "https://github.com/user/repo3", LastUpdated: time.Now().UTC().Truncate(time.Microsecond)}
				saved, err := repo.Save(ctx, link)
				require.NoErrorf(t, err, "Failed to save link before deletion: %v", err)

				err = repo.Delete(ctx, saved)
				require.NoErrorf(t, err, "Failed to delete link: %v", err)

				_, err = repo.GetById(ctx, saved.ID)
				require.Errorf(t, err, "Expected error when getting deleted link, got nil")
				assert.Truef(t, errors.Is(err, domain.ErrLinkNotFound), "Expected ErrLinkNotFound, got %v", err)
			})

			t.Run("Get batch of links", func(t *testing.T) {
				_, err := pool.Exec(ctx, "TRUNCATE TABLE links RESTART IDENTITY CASCADE")
				require.NoErrorf(t, err, "Failed to truncate tables before batch test: %v", err)

				var savedLinks []domain.Link
				for i := 1; i <= 5; i++ {
					link := domain.Link{
						URL:         fmt.Sprintf("https://github.com/user/repo-batch-%d", i),
						LastUpdated: time.Now().UTC().Truncate(time.Microsecond),
					}
					saved, err := repo.Save(ctx, link)
					require.NoErrorf(t, err, "Failed to save batch link %d: %v", i, err)
					savedLinks = append(savedLinks, saved)
				}

				batch1, err := repo.GetBatch(ctx, 2, 0)
				require.NoErrorf(t, err, "Failed to get batch 1: %v", err)
				require.Lenf(t, batch1, 2, "Expected 2 links in batch 1, got %d", len(batch1))
				assert.Equalf(t, savedLinks[0].URL, batch1[0].URL, "Batch 1 item 0 URL mismatch")
				assert.Equalf(t, savedLinks[1].URL, batch1[1].URL, "Batch 1 item 1 URL mismatch")

				batch2, err := repo.GetBatch(ctx, 2, 2)
				require.NoErrorf(t, err, "Failed to get batch 2: %v", err)
				require.Lenf(t, batch2, 2, "Expected 2 links in batch 2, got %d", len(batch2))
				assert.Equalf(t, savedLinks[2].URL, batch2[0].URL, "Batch 2 item 0 URL mismatch")
				assert.Equalf(t, savedLinks[3].URL, batch2[1].URL, "Batch 2 item 1 URL mismatch")

				batch3, err := repo.GetBatch(ctx, 2, 4)
				require.NoErrorf(t, err, "Failed to get batch 3: %v", err)
				require.Lenf(t, batch3, 1, "Expected 1 link in batch 3, got %d", len(batch3))
				assert.Equalf(t, savedLinks[4].URL, batch3[0].URL, "Batch 3 item 0 URL mismatch")

				batch4, err := repo.GetBatch(ctx, 2, 10)
				require.NoErrorf(t, err, "Failed to get batch 4: %v", err)
				assert.Emptyf(t, batch4, "Expected empty slice for out-of-bounds offset, got len %d", len(batch4))
			})
		})
	}
}
