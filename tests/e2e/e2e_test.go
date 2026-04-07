package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/byrnedo/typesafe-config/parse"
	"github.com/docker/go-connections/nat"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"
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

func TestEndToEnd_ScrapperAndBot(t *testing.T) {
	cfg, err := Load("../../cmd/bot/bot.conf")
	require.NoErrorf(t, err, "error loading config: %v", err)

	const (
		BOT_API_PORT      = 63342
		BOT_URL           = "bot:63342"
		SCRAPPER_API_PORT = 63343
		SCRAPPER_URL      = "scrapper:63343"
	)

	ctx := context.Background()

	newNetwork, err := network.New(ctx)
	require.NoErrorf(t, err, "failed to create network: %v", err)

	defer newNetwork.Remove(ctx)

	const (
		dbUser = "testuser"
		dbPass = "testpass"
		dbName = "scrapper_test"
	)

	pgReq := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Networks:     []string{newNetwork.Name},
		NetworkAliases: map[string][]string{
			newNetwork.Name: {"postgres_db"},
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

	pgContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: pgReq,
		Started:          true,
	})
	require.NoErrorf(t, err, "Failed to start PostgreSQL container: %v", err)
	defer pgContainer.Terminate(ctx)

	dbURL := fmt.Sprintf("postgres://%s:%s@postgres_db:5432/%s?sslmode=disable", dbUser, dbPass, dbName)

	botReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../../",
			Dockerfile: "cmd/bot/Dockerfile",
		},
		ExposedPorts: []string{fmt.Sprintf("%d/tcp", BOT_API_PORT)},
		Networks:     []string{newNetwork.Name},
		NetworkAliases: map[string][]string{
			newNetwork.Name: {"bot"},
		},
		Env: map[string]string{
			"APP_TELEGRAM_TOKEN":    cfg.Telegram.Token,
			"BOT_API_PORT":          strconv.Itoa(BOT_API_PORT),
			"SCRAPPER_URL":          SCRAPPER_URL,
			"BOT_API_PROTOCOL":      "http",
			"SCRAPPER_API_PROTOCOL": "http",
		},
		WaitingFor: wait.ForListeningPort(nat.Port(fmt.Sprintf("%d/tcp", BOT_API_PORT))).WithStartupTimeout(30 * time.Second),
	}

	botContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: botReq,
		Started:          true,
	})
	require.NoErrorf(t, err, "failed to start bot container: %v", err)

	defer botContainer.Terminate(ctx)

	scrapperReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../../",
			Dockerfile: "cmd/scrapper/Dockerfile",
		},
		ExposedPorts: []string{fmt.Sprintf("%d/tcp", SCRAPPER_API_PORT)},
		Networks:     []string{newNetwork.Name},
		NetworkAliases: map[string][]string{
			newNetwork.Name: {"scrapper"},
		},

		Env: map[string]string{
			"SCRAPPER_API_PORT":     strconv.Itoa(SCRAPPER_API_PORT),
			"BOT_URL":               BOT_URL,
			"DATABASE_URL":      dbURL,
			"BOT_API_PROTOCOL":      "http",
			"SCRAPPER_API_PROTOCOL": "http",
		},
		WaitingFor: wait.ForListeningPort(nat.Port(fmt.Sprintf("%d/tcp", SCRAPPER_API_PORT))).WithStartupTimeout(30 * time.Second),
	}

	scrapperContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: scrapperReq,
		Started:          true,
	})
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
