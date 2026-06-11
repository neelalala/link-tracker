package cache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application/subscription"
	valkeycache "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/cache/valkey"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/sql"
)

const (
	dbUser     = "testuser"
	dbPass     = "testpass"
	dbName     = "scrapper_test"
	migrations = "file://../../../migrations"
)

type mockLinkValidator struct{}

func (m mockLinkValidator) CanHandle(url string) bool {
	return true
}

type stats struct {
	name      string
	latencies []time.Duration
	total     time.Duration
}

func (s stats) avg() time.Duration {
	if len(s.latencies) == 0 {
		return 0
	}
	var sum time.Duration
	for _, l := range s.latencies {
		sum += l
	}
	return sum / time.Duration(len(s.latencies))
}

func (s stats) percentile(p float64) time.Duration {
	if len(s.latencies) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(s.latencies))
	copy(sorted, s.latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(p / 100 * float64(len(sorted)))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func (s stats) throughput() float64 {
	return float64(len(s.latencies)) / s.total.Seconds()
}

func loadPostgresContainer(ctx context.Context) (testcontainers.Container, error) {
	pgReq := testcontainers.ContainerRequest{
		Image:        "postgres:17-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     dbUser,
			"POSTGRES_PASSWORD": dbPass,
			"POSTGRES_DB":       dbName,
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(30 * time.Second),
	}

	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: pgReq,
		Started:          true,
	})
}

func newCacheWithPrefix(addr, keyPrefix string, clientSide bool, ttl time.Duration) (*valkeycache.Cache, error) {
	return valkeycache.New([]string{addr}, ttl, keyPrefix, clientSide)
}

func runRequests(
	t *testing.T,
	name string,
	svc application.SubscriptionService,
	chatID int64,
	totalRequests int,
	concurrency int,
) stats {
	t.Helper()

	_, err := svc.GetTrackedLinks(context.Background(), chatID)
	require.NoError(t, err)

	latencies := make([]time.Duration, totalRequests)
	var done atomic.Int64

	var wg sync.WaitGroup
	start := time.Now()
	for w := 0; w < concurrency; w++ {
		wg.Go(func() {
			for {
				i := done.Add(1) - 1
				if i >= int64(totalRequests) {
					return
				}
				reqStart := time.Now()
				_, err := svc.GetTrackedLinks(context.Background(), chatID)
				latencies[i] = time.Since(reqStart)
				if err != nil {
					t.Errorf("request failed: %v", err)
					return
				}
			}
		})
	}
	wg.Wait()
	total := time.Since(start)

	return stats{name: name, latencies: latencies, total: total}
}

func TestListLoad(t *testing.T) {
	bench := os.Getenv("BENCHMARK")
	if bench == "" {
		t.Skip("Skipping test because $BENCHMARK is not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	pgContainer, err := loadPostgresContainer(ctx)
	require.NoErrorf(t, err, "Failed to start PostgreSQL container")
	defer pgContainer.Terminate(ctx)

	dbHost, err := pgContainer.Host(ctx)
	require.NoError(t, err)
	dbPort, err := pgContainer.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPass, dbHost, dbPort.Port(), dbName)

	valkeyContainer, err := loadValkeyContainer(ctx)
	require.NoError(t, err, "Failed to start Valkey container")
	defer valkeyContainer.Terminate(ctx)

	valkeyHost, err := valkeyContainer.Host(ctx)
	require.NoError(t, err)
	valkeyPort, err := valkeyContainer.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	valkeyURL := fmt.Sprintf("%s:%s", valkeyHost, valkeyPort.Port())

	m, err := migrate.New(migrations, dbURL)
	require.NoError(t, err)
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		require.NoError(t, err)
	}

	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	chatRepo := sql.NewChatRepository(pool)
	linkRepo := sql.NewLinkRepository(pool)
	subRepo := sql.NewSubscriptionRepository(pool)
	transactor := sql.NewTransactor(pool)

	subService := subscription.NewService(chatRepo, linkRepo, subRepo, transactor, mockLinkValidator{}, log)

	const chatID = int64(1)
	require.NoError(t, subService.RegisterChat(ctx, chatID))

	var (
		seededLinks   = 50
		totalRequests = 20000
		concurrency   = 50
		cacheTTL      = 5 * time.Minute
	)

	for i := range seededLinks {
		url := fmt.Sprintf("https://github.com/neelalala/repo-%d", i)
		_, err := subService.AddLink(ctx, chatID, url, []string{"tag1", "tag2"})
		require.NoError(t, err)
	}

	noCache := runRequests(
		t,
		"no-cache (db only)",
		subService,
		chatID,
		totalRequests,
		concurrency,
	)

	cachePlain, err := newCacheWithPrefix(valkeyURL, "load:plain:", false, cacheTTL)
	require.NoError(t, err)
	defer cachePlain.Close()

	svcPlain := subscription.NewCachingService(subService, cachePlain)
	valkeyCache := runRequests(
		t,
		"valkey cache (no client-side)",
		svcPlain,
		chatID,
		totalRequests,
		concurrency,
	)

	cacheClientSide, err := newCacheWithPrefix(valkeyURL, "load:csc:", true, cacheTTL)
	require.NoError(t, err)
	defer cacheClientSide.Close()

	svcCSC := subscription.NewCachingService(subService, cacheClientSide)
	cscCache := runRequests(t,
		"valkey client-side cache",
		svcCSC,
		chatID,
		totalRequests,
		concurrency,
	)

	report := buildReport([]stats{noCache, valkeyCache, cscCache}, seededLinks, totalRequests, concurrency, cacheTTL)
	t.Log("\n" + report)

	require.NoError(t, os.WriteFile("valkey-benchmark-report.md", []byte(report), 0o644))
}

func buildReport(scenarios []stats, seededLinks, totalRequests, concurrency int, cacheTTL time.Duration) string {
	sb := &strings.Builder{}

	sb.WriteString("# Отчёт по нагрузочному тестированию кеша GET /links\n\n")
	sb.WriteString("Конфигурация прогона:\n\n")
	fmt.Fprintf(sb, "- Отслеживаемых ссылок у чата: **%d**\n", seededLinks)
	fmt.Fprintf(sb, "- Запросов / сценарий: **%d**\n", totalRequests)
	fmt.Fprintf(sb, "- Параллельных воркеров: **%d**\n", concurrency)
	fmt.Fprintf(sb, "- TTL кеша: **%s**\n", cacheTTL)

	sb.WriteString("## Результаты\n\n")
	sb.WriteString("| Сценарий | avg | p50 | p99 | RPS |\n")
	sb.WriteString("|---|---|---|---|---|\n")
	for _, s := range scenarios {
		fmt.Fprintf(sb, "| %s | %s | %s | %s | %.0f |\n",
			s.name,
			s.avg().Round(time.Microsecond),
			s.percentile(50).Round(time.Microsecond),
			s.percentile(99).Round(time.Microsecond),
			s.throughput(),
		)
	}

	return sb.String()
}
