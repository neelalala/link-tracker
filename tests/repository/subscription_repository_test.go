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

func TestSubscriptionRepository_Integration(t *testing.T) {
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

	implementations := map[string]domain.SubscriptionRepository{
		"BUILDER": sqlbuilder.NewSubscriptionRepository(pool),
		"SQL":     sql.NewSubscriptionRepository(pool),
	}

	linksRepo := sql.NewLinkRepository(pool)

	for accessType, repo := range implementations {
		t.Run(fmt.Sprintf("AccessType: %s", accessType), func(t *testing.T) {
			_, err := pool.Exec(ctx, "TRUNCATE TABLE chats, links RESTART IDENTITY CASCADE")
			require.NoErrorf(t, err, "Failed to truncate tables before tests: %v", err)

			createTestDeps := func(chatID int64, url string) int64 {
				_, _ = pool.Exec(ctx, "INSERT INTO chats (id) VALUES ($1) ON CONFLICT DO NOTHING", chatID)

				var linkID int64
				err := pool.QueryRow(ctx, "INSERT INTO links (url) VALUES ($1) RETURNING id", url).Scan(&linkID)
				require.NoErrorf(t, err, "Failed to insert test link: %v", err)
				return linkID
			}

			t.Run("Save subscription", func(t *testing.T) {
				chatID := int64(1)
				linkID := createTestDeps(chatID, "https://github.com/user/tags-repo")

				sub := domain.Subscription{
					ChatID: chatID,
					LinkID: linkID,
					Tags:   []string{"tag1", "tag2"},
				}

				err := repo.Save(ctx, sub)
				require.NoErrorf(t, err, "Failed to save subscription: %v", err)

				subs, err := repo.GetByChatID(ctx, chatID)
				require.NoErrorf(t, err, "Failed to get subscriptions: %v", err)
				require.Len(t, subs, 1)
				assert.Equal(t, sub.LinkID, subs[0].LinkID)

				assert.ElementsMatch(t, sub.Tags, subs[0].Tags, "Tags mismatch")
			})

			t.Run("Get by Chat ID with mixed tags", func(t *testing.T) {
				chatID := int64(2)
				linkID1 := createTestDeps(chatID, "https://github.com/user/repo1")
				linkID2 := createTestDeps(chatID, "https://github.com/user/repo2")

				sub1 := domain.Subscription{ChatID: chatID, LinkID: linkID1, Tags: []string{"tag1"}}
				sub2 := domain.Subscription{ChatID: chatID, LinkID: linkID2, Tags: []string{}}

				err := repo.Save(ctx, sub1)
				require.NoError(t, err)
				err = repo.Save(ctx, sub2)
				require.NoError(t, err)

				subs, err := repo.GetByChatID(ctx, chatID)
				require.NoErrorf(t, err, "Failed to get by ChatId: %v", err)
				require.Len(t, subs, 2)

				for _, s := range subs {
					if s.LinkID == linkID1 {
						assert.ElementsMatch(t, sub1.Tags, s.Tags)
					} else if s.LinkID == linkID2 {
						assert.Empty(t, s.Tags)
					} else {
						t.Errorf("Unexpected link ID: %d", s.LinkID)
					}
				}
			})

			t.Run("Get by Link ID", func(t *testing.T) {
				linkID := createTestDeps(3, "https://github.com/user/repo3")
				createTestDeps(4, "dummy.com")

				err := repo.Save(ctx, domain.Subscription{ChatID: 3, LinkID: linkID, Tags: []string{"tag1"}})
				require.NoError(t, err)
				err = repo.Save(ctx, domain.Subscription{ChatID: 4, LinkID: linkID, Tags: []string{"tag2"}})
				require.NoError(t, err)

				subs, err := repo.GetByLinkID(ctx, linkID)
				require.NoErrorf(t, err, "Failed to get by LinkId: %v", err)
				require.Len(t, subs, 2)

				chatIDs := map[int64]bool{subs[0].ChatID: true, subs[1].ChatID: true}
				assert.True(t, chatIDs[3], "Missing ChatID 3 in result")
				assert.True(t, chatIDs[4], "Missing ChatID 4 in result")
			})

			t.Run("Delete subscription", func(t *testing.T) {
				chatID := int64(5)
				linkID := createTestDeps(chatID, "https://github.com/user/repo5")
				sub := domain.Subscription{ChatID: chatID, LinkID: linkID, Tags: []string{"deleted"}}

				err := repo.Save(ctx, sub)
				require.NoError(t, err)

				link, err := linksRepo.GetByID(ctx, linkID)
				require.NoErrorf(t, err, "Failed to get link: %v", err)
				assert.Equalf(t, linkID, link.ID, "Expected link id %d, got %d", linkID, link.ID)

				deletedSub, err := repo.Delete(ctx, sub.ChatID, sub.LinkID)
				require.NoErrorf(t, err, "Failed to delete subscription: %v", err)
				assert.Equal(t, chatID, deletedSub.ChatID)

				link, err = linksRepo.GetByID(ctx, linkID)
				assert.Truef(t, errors.Is(err, domain.ErrLinkNotFound), "Expected Link not found")

				subs, err := repo.GetByChatID(ctx, chatID)
				require.NoError(t, err)
				assert.Empty(t, subs, "Expected no subscriptions after deletion")

				var count int
				err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM subscription_tags WHERE chat_id = $1 AND link_id = $2", chatID, linkID).Scan(&count)
				require.NoError(t, err)
				assert.Equal(t, 0, count, "Expected subscription_tags to be cascade deleted")
			})

			t.Run("Add and Get Tags", func(t *testing.T) {
				chatID := int64(6)
				linkID := createTestDeps(chatID, "https://github.com/user/repo6")

				err := repo.Save(ctx, domain.Subscription{ChatID: chatID, LinkID: linkID})
				require.NoError(t, err)

				tags := []string{"backend", "go"}
				err = repo.AddTags(ctx, linkID, chatID, tags)
				require.NoError(t, err)

				savedTags, err := repo.GetTags(ctx, linkID, chatID)
				require.NoError(t, err)
				assert.ElementsMatch(t, tags, savedTags)

				additionalTags := []string{"go", "postgres"}
				err = repo.AddTags(ctx, linkID, chatID, additionalTags)
				require.NoError(t, err)

				savedTags, err = repo.GetTags(ctx, linkID, chatID)
				require.NoError(t, err)
				assert.ElementsMatch(t, []string{"backend", "go", "postgres"}, savedTags)
			})

			t.Run("Delete Tags", func(t *testing.T) {
				chatID := int64(7)
				linkID := createTestDeps(chatID, "https://github.com/user/repo7")

				err := repo.Save(ctx, domain.Subscription{ChatID: chatID, LinkID: linkID})
				require.NoError(t, err)

				err = repo.AddTags(ctx, linkID, chatID, []string{"t1", "t2", "t3"})
				require.NoError(t, err)

				err = repo.DeleteTags(ctx, linkID, chatID, []string{"t2", "t3", "t4"})
				require.NoError(t, err)

				tags, err := repo.GetTags(ctx, linkID, chatID)
				require.NoError(t, err)
				assert.ElementsMatch(t, []string{"t1"}, tags)
			})

			t.Run("Tags Empty Edge Cases", func(t *testing.T) {
				chatID := int64(8)
				linkID := createTestDeps(chatID, "https://github.com/user/repo8")

				err := repo.Save(ctx, domain.Subscription{ChatID: chatID, LinkID: linkID})
				require.NoError(t, err)

				err = repo.AddTags(ctx, linkID, chatID, []string{})
				require.NoError(t, err)

				tags, err := repo.GetTags(ctx, linkID, chatID)
				require.NoError(t, err)
				assert.NotNil(t, tags)
				assert.Empty(t, tags)

				err = repo.DeleteTags(ctx, linkID, chatID, []string{})
				require.NoError(t, err)
			})

			t.Run("Delete 1 of 2 subscriptions should not delete link", func(t *testing.T) {
				chat1 := int64(9)
				chat2 := int64(10)

				_, _ = pool.Exec(ctx, "INSERT INTO chats (id) VALUES ($1) ON CONFLICT DO NOTHING", chat1)
				_, _ = pool.Exec(ctx, "INSERT INTO chats (id) VALUES ($1) ON CONFLICT DO NOTHING", chat2)

				var linkID int64
				err := pool.QueryRow(ctx, "INSERT INTO links (url) VALUES ($1) RETURNING id", "https://github.com/user/repo-multi").Scan(&linkID)
				require.NoErrorf(t, err, "Failed to insert test link: %v", err)

				require.NoError(t, repo.Save(ctx, domain.Subscription{ChatID: chat1, LinkID: linkID}))
				require.NoError(t, repo.Save(ctx, domain.Subscription{ChatID: chat2, LinkID: linkID}))

				_, err = repo.Delete(ctx, chat1, linkID)
				require.NoError(t, err)

				link, err := linksRepo.GetByID(ctx, linkID)
				require.NoError(t, err)
				assert.Equal(t, linkID, link.ID)
			})

			t.Run("Delete non-existing subscription", func(t *testing.T) {
				_, err := repo.Delete(ctx, 999, 999)

				require.Error(t, err)
				assert.True(t, errors.Is(err, domain.ErrNotSubscribed))
			})
		})
	}
}
