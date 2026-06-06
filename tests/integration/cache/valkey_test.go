package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	valkeycache "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/cache/valkey"
)

func loadValkeyContainer(ctx context.Context) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        "valkey/valkey:9-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor: wait.ForListeningPort("6379/tcp").
			WithStartupTimeout(60 * time.Second),
	}

	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

func newCache(addr string, clientSide bool) (*valkeycache.Cache, error) {
	return valkeycache.New([]string{addr}, 2*time.Second, "scrapper:links:", clientSide)
}

func TestValkeyCache_Integration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	newNetwork, err := network.New(ctx)
	require.NoError(t, err, "Failed to create network")
	defer newNetwork.Remove(ctx)

	valkeyContainer, err := loadValkeyContainer(ctx)
	require.NoError(t, err, "Failed to start Valkey container")
	defer valkeyContainer.Terminate(ctx)

	valkeyHost, err := valkeyContainer.Host(ctx)
	require.NoError(t, err)
	valkeyPort, err := valkeyContainer.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	valkeyURL := fmt.Sprintf("%s:%s", valkeyHost, valkeyPort.Port())

	for _, clientSide := range []bool{false, true} {
		name := "plain"
		if clientSide {
			name = "client-side-cache"
		}

		chatID := int64(1000)
		if clientSide {
			chatID = int64(2000)
		}

		t.Run(name, func(t *testing.T) {
			cache, err := newCache(valkeyURL, clientSide)
			require.NoError(t, err, "Failed to create cache", "client-side-cache", clientSide)

			links := []domain.TrackedLink{
				{ID: 1, URL: "https://github.com/neelalala/123", Tags: []string{"go", "test"}},
				{ID: 2, URL: "https://stackoverflow.com/q/123", Tags: nil},
			}

			t.Run("Miss on empty cache", func(t *testing.T) {
				chatID++
				got, hit, err := cache.GetLinks(ctx, chatID)
				require.NoError(t, err, "Empty cache must not return an error")
				assert.False(t, hit, "Empty cache must report a miss")
				assert.Nil(t, got)
			})

			t.Run("Set then hit", func(t *testing.T) {
				chatID++
				require.NoError(t, cache.SetLinks(ctx, chatID, links))

				got, hit, err := cache.GetLinks(ctx, chatID)
				require.NoError(t, err)
				assert.True(t, hit, "Value stored must be a hit")
				assert.Equal(t, links, got)
			})

			t.Run("Invalidate removes entry", func(t *testing.T) {
				chatID++
				require.NoError(t, cache.SetLinks(ctx, chatID, links))
				require.NoError(t, cache.Invalidate(ctx, chatID))

				_, hit, err := cache.GetLinks(ctx, chatID)
				require.NoError(t, err)
				assert.False(t, hit, "Invalidated entry must be a miss")
			})

			t.Run("TTL expires entry", func(t *testing.T) {
				chatID++
				require.NoError(t, cache.SetLinks(ctx, chatID, links))

				require.Eventually(t, func() bool {
					_, hit, err := cache.GetLinks(ctx, chatID)
					return err == nil && !hit
				}, 6*time.Second, 250*time.Millisecond, "Entry must expire after TTL")
			})

			t.Run("Empty slice", func(t *testing.T) {
				chatID++
				var empty []domain.TrackedLink
				require.NoError(t, cache.SetLinks(ctx, chatID, empty))

				got, hit, err := cache.GetLinks(ctx, chatID)
				require.NoError(t, err)
				assert.True(t, hit)
				assert.Empty(t, got)
			})
		})
	}
}
