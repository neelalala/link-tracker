package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/byrnedo/typesafe-config/parse"
	"github.com/docker/go-connections/nat"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	BOT_API_PORT      = 63342
	SCRAPPER_API_PORT = 63343

	dbUser = "testuser"
	dbPass = "testpass"
	dbName = "scrapper_test"

	kafkaInternalBroker = "kafka:9092"
	kafkaExternalPort   = "9094/tcp"
	kafkaTopic          = "link-updates"
)

type TelegramConfig struct {
	Token string `config:"token"`
}

type Config struct {
	Telegram TelegramConfig `config:"telegram"`
}

func Load(botConfig string) (*Config, error) {
	godotenv.Load("../../.env")
	tree, err := parse.ParseFile(botConfig)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	cfg := &Config{}

	parse.Populate(cfg, tree.GetConfig(), "")

	return cfg, nil
}

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

func loadBotContainer(ctx context.Context, net *testcontainers.DockerNetwork, cfg *Config) (testcontainers.Container, error) {
	botReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../../",
			Dockerfile: "cmd/bot/Dockerfile",
		},
		ExposedPorts: []string{fmt.Sprintf("%d/tcp", BOT_API_PORT)},
		Networks:     []string{net.Name},
		NetworkAliases: map[string][]string{
			net.Name: {"bot"},
		},
		Env: map[string]string{
			"TELEGRAM_TOKEN":        cfg.Telegram.Token,
			"BOT_API_PORT":          strconv.Itoa(BOT_API_PORT),
			"SCRAPPER_URL":          fmt.Sprintf("scrapper:%d", SCRAPPER_API_PORT),
			"BOT_API_PROTOCOL":      "http",
			"SCRAPPER_API_PROTOCOL": "http",
			"USE_QUEUE":             "false",
		},
		WaitingFor: wait.ForListeningPort(nat.Port(fmt.Sprintf("%d/tcp", BOT_API_PORT))).WithStartupTimeout(30 * time.Second),
	}

	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: botReq,
		Started:          true,
	})
}

func loadScrapperContainer(ctx context.Context, net *testcontainers.DockerNetwork, dbURL string) (testcontainers.Container, error) {
	scrapperReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../../",
			Dockerfile: "cmd/scrapper/Dockerfile",
		},
		ExposedPorts: []string{fmt.Sprintf("%d/tcp", SCRAPPER_API_PORT)},
		Networks:     []string{net.Name},
		NetworkAliases: map[string][]string{
			net.Name: {"scrapper"},
		},

		Env: map[string]string{
			"SCRAPPER_API_PORT":     strconv.Itoa(SCRAPPER_API_PORT),
			"BOT_URL":               fmt.Sprintf("bot:%d", BOT_API_PORT),
			"DATABASE_URL":          dbURL,
			"BOT_API_PROTOCOL":      "http",
			"SCRAPPER_API_PROTOCOL": "http",
			"USE_QUEUE":             "false",
		},
		WaitingFor: wait.ForListeningPort(nat.Port(fmt.Sprintf("%d/tcp", SCRAPPER_API_PORT))).WithStartupTimeout(30 * time.Second),
	}

	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: scrapperReq,
		Started:          true,
	})
}

func TestEndToEnd_BotScrapperHTTP(t *testing.T) {
	cfg, err := Load("../../cmd/bot/bot.conf")
	require.NoErrorf(t, err, "error loading config: %v", err)

	ctx := context.Background()

	newNetwork, err := network.New(ctx)
	require.NoErrorf(t, err, "failed to create network: %v", err)
	defer newNetwork.Remove(ctx)

	pgContainer, err := loadPostgresContainer(ctx, newNetwork)
	require.NoErrorf(t, err, "failed to start PostgreSQL container: %v", err)
	defer pgContainer.Terminate(ctx)

	dbURL := fmt.Sprintf("postgres://%s:%s@postgres_db:5432/%s?sslmode=disable", dbUser, dbPass, dbName)

	botContainer, err := loadBotContainer(ctx, newNetwork, cfg)
	require.NoErrorf(t, err, "failed to start bot container: %v", err)
	defer botContainer.Terminate(ctx)

	scrapperContainer, err := loadScrapperContainer(ctx, newNetwork, dbURL)
	require.NoErrorf(t, err, "failed to start scrapper container: %v", err)
	defer scrapperContainer.Terminate(ctx)

	scrapperHost, _ := scrapperContainer.Host(ctx)
	scrapperPort, _ := scrapperContainer.MappedPort(ctx, nat.Port(strconv.Itoa(SCRAPPER_API_PORT)))
	scrapperURL := fmt.Sprintf("http://%s:%s", scrapperHost, scrapperPort.Port())

	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("Add and get link", func(t *testing.T) {
		chatId := "1"
		link := "https://github.com/user/repo" + chatId

		req1, err := http.NewRequest(http.MethodPost, scrapperURL+"/tg-chat/"+chatId, nil)
		require.NoErrorf(t, err, "Failed to create request: %v", err)
		resp1, err := client.Do(req1)
		require.NoErrorf(t, err, "Failed to register chat: %v", err)

		defer resp1.Body.Close()
		assert.Equalf(t, http.StatusOK, resp1.StatusCode, "Expected status OK for chat registration, got %d", resp1.StatusCode)

		reqBody := map[string]string{"link": link}
		bodyBytes, err := json.Marshal(reqBody)
		require.NoErrorf(t, err, "Failed to marshal request body: %v", err)

		req2, err := http.NewRequest(http.MethodPost, scrapperURL+"/links", bytes.NewBuffer(bodyBytes))
		require.NoErrorf(t, err, "Failed to create request: %v", err)
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Tg-Chat-Id", "1")

		resp2, err := client.Do(req2)
		require.NoErrorf(t, err, "Failed to add link: %v", err)

		defer resp2.Body.Close()
		assert.Equalf(t, http.StatusOK, resp2.StatusCode, "Expected status OK for adding link, got %d", resp2.StatusCode)

		req3, err := http.NewRequest(http.MethodGet, scrapperURL+"/links", nil)
		require.NoErrorf(t, err, "Failed to create request: %v", err)
		req3.Header.Set("Tg-Chat-Id", chatId)

		resp3, err := client.Do(req3)
		require.NoErrorf(t, err, "Failed to get links: %v", err)

		defer resp3.Body.Close()
		assert.Equalf(t, http.StatusOK, resp3.StatusCode, "Expected status OK for getting links, got %d", resp3.StatusCode)
	})

	t.Run("Add and delete link", func(t *testing.T) {
		chatId := "2"
		link := "https://github.com/user/repo" + chatId

		req1, err := http.NewRequest(http.MethodPost, scrapperURL+"/tg-chat/"+chatId, nil)
		require.NoErrorf(t, err, "Failed to create request: %v", err)
		resp1, err := client.Do(req1)
		require.NoErrorf(t, err, "Failed to register chat: %v", err)

		defer resp1.Body.Close()
		assert.Equalf(t, http.StatusOK, resp1.StatusCode, "Expected status OK for chat registration, got %d", resp1.StatusCode)

		reqBody := map[string]string{"link": link}
		bodyBytes, err := json.Marshal(reqBody)
		require.NoErrorf(t, err, "Failed to marshal request body: %v", err)

		req2, err := http.NewRequest(http.MethodPost, scrapperURL+"/links", bytes.NewBuffer(bodyBytes))
		require.NoErrorf(t, err, "Failed to create request: %v", err)
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Tg-Chat-Id", chatId)

		resp2, err := client.Do(req2)
		require.NoErrorf(t, err, "Failed to add link: %v", err)

		defer resp2.Body.Close()
		assert.Equalf(t, http.StatusOK, resp2.StatusCode, "Expected status OK for adding link, got %d", resp2.StatusCode)

		req3, err := http.NewRequest(http.MethodDelete, scrapperURL+"/links", bytes.NewBuffer(bodyBytes))
		require.NoErrorf(t, err, "Failed to create request: %v", err)
		req3.Header.Set("Content-Type", "application/json")
		req3.Header.Set("Tg-Chat-Id", chatId)

		resp3, err := client.Do(req3)
		require.NoErrorf(t, err, "Failed to delete link: %v", err)

		defer resp3.Body.Close()
		assert.Equal(t, http.StatusOK, resp3.StatusCode, "Expected status OK for deleting link, got %d", resp3.StatusCode)

		req4, err := http.NewRequest(http.MethodGet, scrapperURL+"/links", nil)
		require.NoErrorf(t, err, "Failed to create request: %v", err)
		req4.Header.Set("Tg-Chat-Id", chatId)

		resp4, err := client.Do(req4)
		require.NoErrorf(t, err, "Failed to get links: %v", err)

		defer resp4.Body.Close()
		assert.Equalf(t, http.StatusOK, resp4.StatusCode, "Expected status OK for getting links, got %d", resp4.StatusCode)

		respBody, err := io.ReadAll(resp4.Body)
		require.NoErrorf(t, err, "Failed to read response body: %v", err)
		assert.NotContains(t, string(respBody), link, "Response must not contain deleted link")
	})

	t.Run("Attempt to delete link from non-existent chat", func(t *testing.T) {
		chatId := "3"
		fakeChatId := "888"
		link := "https://github.com/user/repo" + chatId

		req1, err := http.NewRequest(http.MethodPost, scrapperURL+"/tg-chat/"+chatId, nil)
		require.NoErrorf(t, err, "Failed to create request: %v", err)

		resp1, err := client.Do(req1)
		require.NoErrorf(t, err, "Failed to register chat: %v", err)

		defer resp1.Body.Close()
		assert.Equalf(t, http.StatusOK, resp1.StatusCode, "Expected status OK for chat registration, got %d", resp1.StatusCode)

		reqBody := map[string]string{"link": link}
		bodyBytes, err := json.Marshal(reqBody)
		require.NoErrorf(t, err, "Failed to marshal request body: %v", err)

		req2, err := http.NewRequest(http.MethodPost, scrapperURL+"/links", bytes.NewBuffer(bodyBytes))
		require.NoErrorf(t, err, "Failed to create request: %v", err)
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Tg-Chat-Id", chatId)

		resp2, err := client.Do(req2)
		require.NoErrorf(t, err, "Failed to add link: %v", err)

		defer resp2.Body.Close()
		assert.Equalf(t, http.StatusOK, resp2.StatusCode, "Expected status OK for adding link, got %d", resp2.StatusCode)

		req3, err := http.NewRequest(http.MethodDelete, scrapperURL+"/links", bytes.NewBuffer(bodyBytes))
		require.NoErrorf(t, err, "Failed to create request: %v", err)
		req3.Header.Set("Content-Type", "application/json")
		req3.Header.Set("Tg-Chat-Id", fakeChatId)

		resp3, err := client.Do(req3)
		require.NoErrorf(t, err, "Failed to delete link from fake chat: %v", err)

		defer resp3.Body.Close()
		assert.NotEqualf(t, http.StatusOK, resp3.StatusCode, "Expected error status (not 200) for deleting from non-existent chat, got %d", resp3.StatusCode)

		req4, err := http.NewRequest(http.MethodGet, scrapperURL+"/links", nil)
		require.NoErrorf(t, err, "Failed to create request: %v", err)
		req4.Header.Set("Tg-Chat-Id", chatId)

		resp4, err := client.Do(req4)
		require.NoErrorf(t, err, "Failed to get links: %v", err)

		defer resp4.Body.Close()
		assert.Equalf(t, http.StatusOK, resp4.StatusCode, "Expected status OK for getting links, got %d", resp4.StatusCode)

		respBody, err := io.ReadAll(resp4.Body)
		require.NoErrorf(t, err, "Failed to read response body: %v", err)
		assert.Containsf(t, string(respBody), link, "Response must contain the link since deletion should have failed. Response: \n%s", string(respBody))
	})

	t.Run("Add link to non-existent chat", func(t *testing.T) {
		chatId := "4"
		fakeChatId := "999"
		link := "https://github.com/user/repo" + chatId

		req1, err := http.NewRequest(http.MethodPost, scrapperURL+"/tg-chat/"+chatId, nil)
		require.NoErrorf(t, err, "Failed to create request: %v", err)

		resp1, err := client.Do(req1)
		require.NoErrorf(t, err, "Failed to register chat: %v", err)

		defer resp1.Body.Close()
		assert.Equalf(t, http.StatusOK, resp1.StatusCode, "Expected status OK for chat registration, got %d", resp1.StatusCode)

		reqBody := map[string]string{"link": link}
		bodyBytes, err := json.Marshal(reqBody)
		require.NoErrorf(t, err, "Failed to marshal request body: %v", err)

		req2, err := http.NewRequest(http.MethodPost, scrapperURL+"/links", bytes.NewBuffer(bodyBytes))
		require.NoErrorf(t, err, "Failed to create request: %v", err)
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Tg-Chat-Id", fakeChatId)

		resp2, err := client.Do(req2)
		require.NoErrorf(t, err, "Failed to add link to fake chat: %v", err)

		defer resp2.Body.Close()
		assert.NotEqualf(t, http.StatusOK, resp2.StatusCode, "Expected error status (not 200) for adding link to non-existent chat, got %d", resp2.StatusCode)
	})

	t.Run("Work with deleted chat", func(t *testing.T) {
		chatId := "5"
		link := "https://github.com/user/repo" + chatId

		req1, err := http.NewRequest(http.MethodPost, scrapperURL+"/tg-chat/"+chatId, nil)
		require.NoErrorf(t, err, "Failed to create request: %v", err)

		resp1, err := client.Do(req1)
		require.NoErrorf(t, err, "Failed to register chat: %v", err)

		defer resp1.Body.Close()
		assert.Equalf(t, http.StatusOK, resp1.StatusCode, "Expected status OK for chat registration, got %d", resp1.StatusCode)

		req2, err := http.NewRequest(http.MethodDelete, scrapperURL+"/tg-chat/"+chatId, nil)
		require.NoErrorf(t, err, "Failed to create request: %v", err)

		resp2, err := client.Do(req2)
		require.NoErrorf(t, err, "Failed to delete chat: %v", err)

		defer resp2.Body.Close()
		assert.Equalf(t, http.StatusOK, resp2.StatusCode, "Expected status OK for deleting chat, got %d", resp2.StatusCode)

		reqBody := map[string]string{"link": link}
		bodyBytes, err := json.Marshal(reqBody)
		require.NoErrorf(t, err, "Failed to marshal request body: %v", err)

		req3, err := http.NewRequest(http.MethodPost, scrapperURL+"/links", bytes.NewBuffer(bodyBytes))
		require.NoErrorf(t, err, "Failed to create request: %v", err)
		req3.Header.Set("Content-Type", "application/json")
		req3.Header.Set("Tg-Chat-Id", chatId)

		resp3, err := client.Do(req3)
		require.NoErrorf(t, err, "Failed to add link to deleted chat: %v", err)

		defer resp3.Body.Close()
		assert.NotEqualf(t, http.StatusOK, resp3.StatusCode, "Expected error status (not 200) for adding link to deleted chat, got %d", resp3.StatusCode)
	})

	t.Run("Delete non-existent chat", func(t *testing.T) {
		chatId := "6"

		req1, err := http.NewRequest(http.MethodDelete, scrapperURL+"/tg-chat/"+chatId, nil)
		require.NoErrorf(t, err, "Failed to create request: %v", err)

		resp1, err := client.Do(req1)
		require.NoErrorf(t, err, "Failed to delete non-existent chat: %v", err)

		defer resp1.Body.Close()
		assert.Equalf(t, http.StatusNotFound, resp1.StatusCode, "Expected Not Found for deleting non-existent chat, got %d", resp1.StatusCode)
	})
}

type linkUpdateMsg struct {
	ID          int64   `json:"id"`
	URL         string  `json:"url"`
	Description string  `json:"description"`
	Preview     string  `json:"preview"`
	TgChatIDs   []int64 `json:"tgChatIds"`
}

func TestEndToEnd_ScrapperKafkaBot(t *testing.T) {
	cfg, err := Load("../../cmd/bot/bot.conf")
	require.NoErrorf(t, err, "error loading config: %v", err)

	ctx := context.Background()

	newNetwork, err := network.New(ctx)
	require.NoErrorf(t, err, "failed to create network: %v", err)
	defer newNetwork.Remove(ctx)

	pgContainer, err := loadPostgresContainer(ctx, newNetwork)
	require.NoErrorf(t, err, "failed to start PostgreSQL container: %v", err)
	defer pgContainer.Terminate(ctx)

	dbHost, err := pgContainer.Host(ctx)
	require.NoErrorf(t, err, "failed to get PostgreSQL host: %v", err)
	dbPort, err := pgContainer.MappedPort(ctx, "5432/tcp")
	require.NoErrorf(t, err, "failed to get PostgreSQL port: %v", err)

	internalDBURL := fmt.Sprintf("postgres://%s:%s@postgres_db:5432/%s?sslmode=disable", dbUser, dbPass, dbName)
	testDBURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPass, dbHost, dbPort.Port(), dbName)

	kafkaContainer, err := loadKafkaBroker(ctx, newNetwork)
	require.NoErrorf(t, err, "failed to start Kafka container: %v", err)
	defer kafkaContainer.Terminate(ctx)

	kafkaHost, err := kafkaContainer.Host(ctx)
	require.NoErrorf(t, err, "failed to get Kafka host: %v", err)
	kafkaPort, err := kafkaContainer.MappedPort(ctx, kafkaExternalPort)
	require.NoErrorf(t, err, "failed to get Kafka port: %v", err)

	testBroker := fmt.Sprintf("%s:%s", kafkaHost, kafkaPort.Port())

	require.NoError(t, createKafkaTopic(testBroker, kafkaTopic))

	scrapperContainer, err := loadKafkaScrapper(ctx, newNetwork, internalDBURL)
	require.NoErrorf(t, err, "failed to start Kafka scrapper: %v", err)
	defer scrapperContainer.Terminate(ctx)

	scrapperHost, err := scrapperContainer.Host(ctx)
	require.NoErrorf(t, err, "failed to get Scrapper host: %v", err)
	scrapperPort, err := scrapperContainer.MappedPort(ctx, nat.Port(fmt.Sprintf("%d/tcp", SCRAPPER_API_PORT)))
	require.NoErrorf(t, err, "failed to get Scrapper port: %v", err)
	scrapperURL := fmt.Sprintf("http://%s:%s", scrapperHost, scrapperPort.Port())

	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("Outbox worker sends pending record to Kafka", func(t *testing.T) {
		chatID := "1"
		req, err := http.NewRequest(http.MethodPost, scrapperURL+"/tg-chat/"+chatID, nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		conn, err := pgx.Connect(ctx, testDBURL)
		require.NoError(t, err)
		defer conn.Close(ctx)

		payload := linkUpdateMsg{
			ID:          1,
			URL:         "https://github.com/test/repo",
			Description: "new commit",
			TgChatIDs:   []int64{1},
		}
		payloadBytes, err := json.Marshal(payload)
		require.NoError(t, err)

		_, err = conn.Exec(ctx,
			`INSERT INTO outbox (topic, payload) VALUES ($1, $2)`,
			kafkaTopic, payloadBytes,
		)
		require.NoErrorf(t, err, "failed to insert into outbox")

		msg, err := consumeOneKafkaMessage(testBroker, kafkaTopic, 40*time.Second)
		assert.NoError(t, err)
		require.NotNil(t, msg, "expected message in Kafka, got none within timeout")

		var received linkUpdateMsg
		require.NoError(t, json.Unmarshal(msg.Value, &received))
		assert.Equal(t, payload.ID, received.ID)
		assert.Equal(t, payload.URL, received.URL)
		assert.Equal(t, payload.TgChatIDs, received.TgChatIDs)

		assertOutboxStatus(t, ctx, conn, 42, "SENT")
	})

	t.Run("Outbox worker sends batch of pending records", func(t *testing.T) {
		conn, err := pgx.Connect(ctx, testDBURL)
		require.NoError(t, err)
		defer conn.Close(ctx)

		const count = 3
		updates := []linkUpdateMsg{
			{ID: 100, URL: "https://github.com/a/repo", TgChatIDs: []int64{200}},
			{ID: 101, URL: "https://stackoverflow.com/q/1", TgChatIDs: []int64{200, 201}},
			{ID: 102, URL: "https://github.com/b/repo", TgChatIDs: []int64{202}},
		}

		for _, u := range updates {
			b, _ := json.Marshal(u)
			_, err := conn.Exec(ctx,
				`INSERT INTO outbox (topic, payload) VALUES ($1, $2)`,
				kafkaTopic, b,
			)
			require.NoError(t, err)
		}

		received, err := consumeNKafkaMessages(testBroker, kafkaTopic, count, 40*time.Second)
		assert.NoError(t, err)
		assert.Len(t, received, count, "expected %d messages in Kafka", count)
	})

	t.Run("Bot starts and connects to Kafka consumer group", func(t *testing.T) {
		botContainer, err := loadKafkaBot(ctx, newNetwork, cfg, internalDBURL)
		require.NoError(t, err)
		defer botContainer.Terminate(ctx)

		msg := linkUpdateMsg{
			ID:        999,
			URL:       "https://github.com/test/kafka-bot",
			TgChatIDs: []int64{300},
		}
		require.NoError(t, produceKafkaMessage(testBroker, kafkaTopic, msg))

		time.Sleep(8 * time.Second)

		state, err := botContainer.State(ctx)
		require.NoError(t, err)
		assert.True(t, state.Running, "bot container must still be running after consuming message")
	})
}

func loadKafkaBroker(ctx context.Context, net *testcontainers.DockerNetwork) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        "bitnami/kafka:3.7",
		ExposedPorts: []string{kafkaExternalPort},
		Networks:     []string{net.Name},
		NetworkAliases: map[string][]string{
			net.Name: {"kafka"},
		},
		Env: map[string]string{
			"KAFKA_CFG_NODE_ID":                        "1",
			"KAFKA_CFG_PROCESS_ROLES":                  "broker,controller",
			"KAFKA_CFG_CONTROLLER_QUORUM_VOTERS":       "1@kafka:9093",
			"KAFKA_CFG_CONTROLLER_LISTENER_NAMES":      "CONTROLLER",
			"KAFKA_CFG_INTER_BROKER_LISTENER_NAME":     "PLAINTEXT",
			"KAFKA_CFG_LISTENERS":                      "PLAINTEXT://:9092,CONTROLLER://:9093,EXTERNAL://:9094",
			"KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP": "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,EXTERNAL:PLAINTEXT",
			"KAFKA_CFG_ADVERTISED_LISTENERS":           "PLAINTEXT://kafka:9092,EXTERNAL://localhost:9094",
			"ALLOW_PLAINTEXT_LISTENER":                 "yes",
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

func loadKafkaScrapper(ctx context.Context, net *testcontainers.DockerNetwork, dbURL string) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../../",
			Dockerfile: "cmd/scrapper/Dockerfile",
		},
		ExposedPorts: []string{fmt.Sprintf("%d/tcp", SCRAPPER_API_PORT)},
		Networks:     []string{net.Name},
		NetworkAliases: map[string][]string{
			net.Name: {"scrapper"},
		},
		Env: map[string]string{
			"SCRAPPER_API_PORT":         strconv.Itoa(SCRAPPER_API_PORT),
			"BOT_URL":                   fmt.Sprintf("bot:%d", BOT_API_PORT),
			"DATABASE_URL":              dbURL,
			"BOT_API_PROTOCOL":          "http",
			"SCRAPPER_API_PROTOCOL":     "http",
			"USE_QUEUE":                 "true",
			"KAFKA_BROKERS":             kafkaInternalBroker,
			"KAFKA_TOPIC":               kafkaTopic,
			"KAFKA_WORKERS_INTERVAL":    "5s",
			"KAFKA_WORKERS_EVENT_LIMIT": "50",
			"STACKOVERFLOW_KEY":         "test-key",
		},
		WaitingFor: wait.ForListeningPort(nat.Port(fmt.Sprintf("%d/tcp", SCRAPPER_API_PORT))).
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

func loadKafkaBot(
	ctx context.Context,
	net *testcontainers.DockerNetwork,
	cfg *Config,
	telegramURL string,
	dbURL string,
) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../../",
			Dockerfile: "cmd/bot/Dockerfile",
		},
		ExposedPorts: []string{fmt.Sprintf("%d/tcp", BOT_API_PORT)},
		Networks:     []string{net.Name},
		NetworkAliases: map[string][]string{
			net.Name: {"bot"},
		},
		Env: map[string]string{
			"TELEGRAM_TOKEN":           cfg.Telegram.Token,
			"TELEGRAM_API_URL":         telegramURL,
			"BOT_API_PORT":             strconv.Itoa(SCRAPPER_API_PORT),
			"SCRAPPER_URL":             fmt.Sprintf("scrapper:%d", SCRAPPER_API_PORT),
			"BOT_API_PROTOCOL":         "http",
			"SCRAPPER_API_PROTOCOL":    "http",
			"DATABASE_URL":             dbURL,
			"USE_QUEUE":                "true",
			"KAFKA_BROKERS":            kafkaInternalBroker,
			"KAFKA_TOPIC":              kafkaTopic,
			"KAFKA_BOT_CONSUMER_GROUP": "bot-test-group",
		},
		WaitingFor: wait.ForListeningPort(nat.Port(fmt.Sprintf("%d/tcp", BOT_API_PORT))).
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
	cfg.Version = sarama.V3_6_0_0
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	cfg.Producer.Return.Successes = true
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	return cfg
}

func createKafkaTopic(broker, topic string) error {
	admin, err := sarama.NewClusterAdmin([]string{broker}, newSaramaConfig())
	if err != nil {
		return fmt.Errorf("failed to create kafka admin: %w", err)
	}
	defer admin.Close()

	err = admin.CreateTopic(topic, &sarama.TopicDetail{
		NumPartitions:     1,
		ReplicationFactor: 1,
	}, false)

	if err != nil && !errors.Is(err, sarama.ErrTopicAlreadyExists) {
		return fmt.Errorf("failed to create topic %s: %w", topic, err)
	}
	return nil
}

func consumeOneKafkaMessage(broker, topic string, timeout time.Duration) (*sarama.ConsumerMessage, error) {
	msgs, err := consumeNKafkaMessages(broker, topic, 1, timeout)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("no kafka messages found for topic %s", topic)
	}
	return msgs[0], nil
}

func consumeNKafkaMessages(broker, topic string, n int, timeout time.Duration) ([]*sarama.ConsumerMessage, error) {
	consumer, err := sarama.NewConsumer([]string{broker}, newSaramaConfig())
	if err != nil {
		return nil, err
	}
	defer consumer.Close()

	partConsumer, err := consumer.ConsumePartition(topic, 0, sarama.OffsetOldest)
	if err != nil {
		return nil, err
	}
	defer partConsumer.Close()

	var collected []*sarama.ConsumerMessage
	deadline := time.After(timeout)

	for len(collected) < n {
		select {
		case msg, ok := <-partConsumer.Messages():
			if !ok {
				return collected, nil
			}
			collected = append(collected, msg)
		case err := <-partConsumer.Errors():
			return nil, err
		case <-deadline:
			return nil, fmt.Errorf("timeout waiting for %d messages, got %d", n, len(collected))
		}
	}
	return collected, nil
}

func produceKafkaMessage(broker, topic string, payload any) error {
	producer, err := sarama.NewSyncProducer([]string{broker}, newSaramaConfig())
	if err != nil {
		return fmt.Errorf("failed to create sarama producer: %w", err)
	}
	defer producer.Close()

	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	_, _, err = producer.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(b),
	})
	return err
}

func assertOutboxStatus(t *testing.T, ctx context.Context, conn *pgx.Conn, payloadID int64, wantStatus string) {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var status string
		err := conn.QueryRow(ctx,
			`SELECT status FROM outbox WHERE payload::jsonb->>'id' = $1 ORDER BY created_at DESC LIMIT 1`,
			strconv.FormatInt(payloadID, 10),
		).Scan(&status)
		if err == nil && status == wantStatus {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	var status string
	_ = conn.QueryRow(ctx,
		`SELECT status FROM outbox WHERE payload::jsonb->>'id' = $1 ORDER BY created_at DESC LIMIT 1`,
		strconv.FormatInt(payloadID, 10),
	).Scan(&status)
	assert.Equalf(t, wantStatus, status, "outbox status for payload id=%d", payloadID)
}
