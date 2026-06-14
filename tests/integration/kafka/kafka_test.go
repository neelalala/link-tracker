package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	agentdomain "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
	agentinkafka "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/adapter/in/kafka"
	agentoutkafka "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/adapter/out/kafka"
	agentmapper "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/adapter/out/kafka/mapper"
	agentsql "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/repository/sql"
	botdomain "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	botkafka "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/listener/kafka"
	scrapperdomain "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	scrapperkafka "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/notifier/kafka"
	scrappermapper "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/adapter/out/notifier/kafka/mapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/sql"
)

const (
	dbUser     = "testuser"
	dbPass     = "testpass"
	dbName     = "scrapper_test"
	migrations = "file://../../../migrations"

	topicRaw               = "test-topic.raw"
	topicProcessed         = "test-topic.processed"
	topicDql               = "test-topic-dql"
	consumerGroupRaw       = "test-consumer-group-raw"
	consumerGroupProcessed = "test-consumer-group-processed"

	delay         = 100 * time.Millisecond
	maxDelay      = 10 * delay
	backoffFactor = 2.0
	retries       = 3
)

func loadPostgresContainer(ctx context.Context, net *testcontainers.DockerNetwork) (testcontainers.Container, error) {
	pgReq := testcontainers.ContainerRequest{
		Image:        "postgres:17-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Networks:     []string{net.Name},
		NetworkAliases: map[string][]string{
			net.Name: {"postgres_db"},
		},
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

func loadKafkaContainer(ctx context.Context, net *testcontainers.DockerNetwork) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        "apache/kafka:latest",
		ExposedPorts: []string{"9094:9094/tcp"},
		Networks:     []string{net.Name},
		NetworkAliases: map[string][]string{
			net.Name: {"kafka"},
		},
		Env: map[string]string{
			"KAFKA_NODE_ID":                          "1",
			"KAFKA_PROCESS_ROLES":                    "controller,broker",
			"KAFKA_CONTROLLER_QUORUM_VOTERS":         "1@kafka:9093",
			"KAFKA_LISTENERS":                        "PLAINTEXT://:9092,CONTROLLER://:9093,EXTERNAL://:9094",
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":   "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,EXTERNAL:PLAINTEXT",
			"KAFKA_CONTROLLER_LISTENER_NAMES":        "CONTROLLER",
			"KAFKA_INTER_BROKER_LISTENER_NAME":       "PLAINTEXT",
			"KAFKA_ADVERTISED_LISTENERS":             "PLAINTEXT://kafka:9092,EXTERNAL://localhost:9094",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR": "1",
		},
		WaitingFor: wait.ForLog("Kafka Server started").
			WithStartupTimeout(60 * time.Second),
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}

func loadSchemaRegistryContainer(ctx context.Context, net *testcontainers.DockerNetwork) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        "confluentinc/cp-schema-registry:latest",
		ExposedPorts: []string{"8081:8081/tcp"},
		Networks:     []string{net.Name},
		NetworkAliases: map[string][]string{
			net.Name: {"schema-registry"},
		},
		Env: map[string]string{
			"SCHEMA_REGISTRY_HOST_NAME":                    "schema-registry",
			"SCHEMA_REGISTRY_KAFKASTORE_BOOTSTRAP_SERVERS": "PLAINTEXT://kafka:9092",
		},
		WaitingFor: wait.ForLog("Server started").
			WithStartupTimeout(60 * time.Second),
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}

func newSaramaConfig() *sarama.Config {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V4_0_0_0
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	cfg.Producer.Return.Successes = true
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	return cfg
}

func createKafkaTopic(broker, topic string) error {
	admin, err := sarama.NewClusterAdmin([]string{broker}, newSaramaConfig())
	if err != nil {
		return fmt.Errorf("err create kafka admin: %w", err)
	}
	defer admin.Close()

	err = admin.CreateTopic(topic, &sarama.TopicDetail{
		NumPartitions:     1,
		ReplicationFactor: 1,
	}, false)

	if err != nil && !errors.Is(err, sarama.ErrTopicAlreadyExists) {
		return fmt.Errorf("err create topic %s: %w", topic, err)
	}
	return nil
}

type mockProcessedUpdateHandler struct {
	ch chan botdomain.LinkUpdate
}

func newMockProcessedUpdateHandler() *mockProcessedUpdateHandler {
	return &mockProcessedUpdateHandler{ch: make(chan botdomain.LinkUpdate, 10)}
}

func (m *mockProcessedUpdateHandler) HandleUpdate(ctx context.Context, update botdomain.LinkUpdate) error {
	select {
	case m.ch <- update:
	default:
	}
	return nil
}

func (m *mockProcessedUpdateHandler) waitForUpdate(timeout time.Duration) (botdomain.LinkUpdate, error) {
	select {
	case u := <-m.ch:
		return u, nil
	case <-time.After(timeout):
		return botdomain.LinkUpdate{}, fmt.Errorf("timeout waiting for update")
	}
}

type mockRawUpdateHandler struct {
	ch chan agentdomain.LinkUpdate
}

func newMockRawUpdateHandler() *mockRawUpdateHandler {
	return &mockRawUpdateHandler{
		ch: make(chan agentdomain.LinkUpdate, 10),
	}
}

func (m *mockRawUpdateHandler) HandleUpdate(ctx context.Context, update agentdomain.LinkUpdate) error {
	select {
	case m.ch <- update:
	default:
	}
	return nil
}

func (m *mockRawUpdateHandler) waitForUpdate(timeout time.Duration) (agentdomain.LinkUpdate, error) {
	select {
	case u := <-m.ch:
		return u, nil
	case <-time.After(timeout):
		return agentdomain.LinkUpdate{}, fmt.Errorf("timeout waiting for update")
	}
}

func TestScrapperKafka_Integration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	newNetwork, err := network.New(ctx)
	require.NoErrorf(t, err, "Failed to create network")
	defer newNetwork.Remove(ctx)

	pgContainer, err := loadPostgresContainer(ctx, newNetwork)
	require.NoErrorf(t, err, "Failed to start PostgreSQL container")
	defer pgContainer.Terminate(ctx)

	dbHost, err := pgContainer.Host(ctx)
	require.NoError(t, err)
	dbPort, err := pgContainer.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPass, dbHost, dbPort.Port(), dbName)

	kafkaContainer, err := loadKafkaContainer(ctx, newNetwork)
	require.NoErrorf(t, err, "Failed to start kafka")
	defer kafkaContainer.Terminate(ctx)

	kafkaHost, err := kafkaContainer.Host(ctx)
	require.NoErrorf(t, err, "Failed to get kafka host")
	if kafkaHost == "localhost" {
		kafkaHost = "127.0.0.1"
	}
	testBroker := fmt.Sprintf("%s:9094", kafkaHost)

	require.NoError(t, createKafkaTopic(testBroker, topicRaw))
	require.NoError(t, createKafkaTopic(testBroker, topicProcessed))

	srContainer, err := loadSchemaRegistryContainer(ctx, newNetwork)
	require.NoError(t, err, "Failed to start Schema Registry container")
	defer srContainer.Terminate(ctx)

	srHost, err := srContainer.Host(ctx)
	require.NoErrorf(t, err, "Failed to get schema registry host")
	if srHost == "localhost" {
		srHost = "127.0.0.1"
	}
	testRegistry := fmt.Sprintf("http://%s:8081", srHost)

	m, err := migrate.New(migrations, dbURL)
	require.NoErrorf(t, err, "Failed to create migrate instance")

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		require.NoErrorf(t, err, "Failed to apply migrations")
	}

	pool, err := pgxpool.New(ctx, dbURL)
	require.NoErrorf(t, err, "Failed to connect to database")
	defer pool.Close()

	outRepo := sql.NewOutboxRepository(pool)

	rawSchemaString, err := os.ReadFile("../../../docs/raw_link_update.avsc")
	require.NoErrorf(t, err, "Failed to read schema from file %s", "docs/raw_link_update.avsc")

	rawTopicCfg := scrapperkafka.TopicConfig{
		Topic:        topicRaw,
		SchemaString: string(rawSchemaString),
		ParseFunc:    scrappermapper.RawLinkUpdateToNative,
	}

	producerRaw, err := scrapperkafka.NewProducer([]string{testBroker}, testRegistry, rawTopicCfg, outRepo, 50, 5, log)
	require.NoErrorf(t, err, "Failed to create scrapper producer")
	defer producerRaw.Close()

	scrapperNotifier := scrapperkafka.NewNotifier(outRepo, topicRaw, log)

	agentOutboxRepo := agentsql.NewOutboxRepository(pool)

	processedSchemaString, err := os.ReadFile("../../../docs/processed_link_update.avsc")
	require.NoErrorf(t, err, "Failed to read schema from file %s", "docs/processed_link_update.avsc")

	processedTopicCfg := agentoutkafka.TopicConfig{
		Topic:        topicProcessed,
		SchemaString: string(processedSchemaString),
		ParseFunc:    agentmapper.ProcessedLinkUpdateToNative,
	}

	producerProcessed, err := agentoutkafka.NewProducer([]string{testBroker}, testRegistry, processedTopicCfg, agentOutboxRepo, 50, 5, log)
	require.NoErrorf(t, err, "Failed to create agent producer")
	defer producerProcessed.Close()

	agentNotifier := agentoutkafka.NewNotifier(agentOutboxRepo, topicProcessed, log)

	agentRawUpdatesHandler := newMockRawUpdateHandler()

	listenerRaw, err := agentinkafka.NewListener(
		[]string{testBroker},
		consumerGroupRaw,
		topicRaw,
		topicDql,
		delay,
		maxDelay,
		backoffFactor,
		retries,
		testRegistry,
		agentRawUpdatesHandler,
		log,
	)

	botProcessedUpdateHandler := newMockProcessedUpdateHandler()

	listenerProcessed, err := botkafka.NewListener(
		[]string{testBroker},
		consumerGroupProcessed,
		topicProcessed,
		topicDql,
		delay,
		maxDelay,
		backoffFactor,
		retries,
		testRegistry,
		botProcessedUpdateHandler,
		log,
	)
	require.NoErrorf(t, err, "Failed to create listenerProcessed")

	go func() { listenerRaw.Start() }()
	defer listenerRaw.Stop(ctx)

	go func() { listenerProcessed.Start() }()
	defer listenerProcessed.Stop(ctx)

	require.NoError(t, listenerRaw.WaitReady(ctx))
	require.NoError(t, listenerProcessed.WaitReady(ctx))

	t.Run("Scrapper - Kafka - Agent - Kafka - Bot flow", func(t *testing.T) {
		updateRaw := scrapperdomain.LinkUpdate{
			ID:          1,
			URL:         "https://github.com/test/1",
			Description: "1",
			TgChatIDs:   []int64{1},
		}

		err := scrapperNotifier.SendUpdate(ctx, updateRaw)
		require.NoError(t, err, "Failed to send update from scrapper")

		err = producerRaw.SendEvents(ctx)
		require.NoError(t, err, "Failed to send events from scrapper")

		receivedRaw, err := agentRawUpdatesHandler.waitForUpdate(15 * time.Second)
		require.NoError(t, err, "Did not receive raw update in agent notifier")

		updateProcessed := agentdomain.ProcessedLinkUpdate{
			URL:         receivedRaw.URL,
			Description: receivedRaw.Description,
			Priority:    agentdomain.PriorityHigh,
			TgChatIDs:   receivedRaw.TgChatIDs,
		}
		err = agentNotifier.SendUpdate(ctx, updateProcessed)
		require.NoError(t, err, "Failed to send update from agent notifier")

		err = producerProcessed.SendEvents(ctx)
		require.NoError(t, err, "Did not send events from producer")

		assert.Equal(t, updateRaw.ID, receivedRaw.ID)
		assert.Equal(t, updateRaw.URL, receivedRaw.URL)
		assert.Equal(t, updateRaw.Author, receivedRaw.Author)
		assert.Equal(t, updateRaw.Description, receivedRaw.Description)
		assert.Equal(t, updateRaw.TgChatIDs, receivedRaw.TgChatIDs)

		receivedProcessed, err := botProcessedUpdateHandler.waitForUpdate(15 * time.Second)
		require.NoError(t, err, "Did not receive processed update in bot notifier")

		assert.Equal(t, updateRaw.URL, receivedProcessed.URL)
		assert.Equal(t, string(agentdomain.PriorityHigh), string(receivedProcessed.Priority))
		assert.Equal(t, updateRaw.TgChatIDs, receivedProcessed.TgChatIDs)
	})
}
