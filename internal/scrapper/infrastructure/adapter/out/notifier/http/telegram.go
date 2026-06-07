package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

const (
	endpoint = "updates"

	httpClientTimeout = 1 * time.Minute
)

type Bot struct {
	url        string
	httpClient *http.Client
	timeout    time.Duration
	log        *slog.Logger
}

func NewBot(url string, timeout time.Duration, log *slog.Logger) *Bot {
	return &Bot{
		url:        url,
		httpClient: &http.Client{Timeout: httpClientTimeout},
		timeout:    timeout,
		log:        log,
	}
}

func (bot *Bot) SendUpdate(ctx context.Context, update domain.LinkUpdate) error {
	bot.log.Debug("sending update to bot",
		"url", update.URL,
		"description", update.Description,
		"preview", update.Preview,
	)
	var requestJson = struct {
		Id          int64   `json:"id"`
		Url         string  `json:"url"`
		Description string  `json:"description"`
		Preview     string  `json:"preview"`
		TgChatIds   []int64 `json:"tgChatIds"`
	}{
		Id:          update.ID,
		Url:         update.URL,
		Description: update.Description,
		Preview:     update.Preview,
		TgChatIds:   update.TgChatIDs,
	}

	body, err := json.Marshal(requestJson)
	if err != nil {
		return fmt.Errorf("failed to marshal update request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, bot.timeout)
	defer cancel()

	query := fmt.Sprintf("%s/%s", bot.url, endpoint)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, query, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := bot.httpClient.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("send update request timed out: %w", err)
		}
		return fmt.Errorf("failed to send request to bot: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("bot api returned unexpected status: %d", response.StatusCode)
	}

	bot.log.Debug("update sent to bot",
		"url", update.URL,
	)

	return nil
}
