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

				subs, err := repo.GetByChatId(ctx, chatID)
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

				subs, err := repo.GetByChatId(ctx, chatID)
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

				subs, err := repo.GetByLinkId(ctx, linkID)
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

				deletedSub, err := repo.Delete(ctx, sub)
				require.NoErrorf(t, err, "Failed to delete subscription: %v", err)
				assert.Equal(t, chatID, deletedSub.ChatID)

				subs, err := repo.GetByChatId(ctx, chatID)
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
		})
	}
}
