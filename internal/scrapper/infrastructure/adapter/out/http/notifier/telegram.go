package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

const (
	endpoint = "updates"
)

type Bot struct {
	url        string
	httpClient *http.Client
}

func NewBot(url string) *Bot {
	return &Bot{
		url:        url,
		httpClient: &http.Client{},
	}
}

func (bot *Bot) SendUpdate(ctx context.Context, update domain.LinkUpdate) error {
	type requestJson struct {
		Id          int64   `json:"id"`
		Url         string  `json:"url"`
		Description string  `json:"description"`
		TgChatIds   []int64 `json:"tgChatIds"`
	}

	reqJson := requestJson{
		Id:          update.ID,
		Url:         update.URL,
		Description: update.Description,
		TgChatIds:   update.TgChatIDs,
	}

	body, err := json.Marshal(reqJson)
	if err != nil {
		return fmt.Errorf("failed to marshal update request: %w", err)
	}

	query := fmt.Sprintf("%s/%s", bot.url, endpoint)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, query, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := bot.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed to send request to bot: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("bot api returned unexpected status: %d", response.StatusCode)
	}

	return nil
}
