package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"net/http"
	"time"
)

const (
	endpoint = "updates"
	timeout  = 10 * time.Second
)

type Bot struct {
	url        string
	httpClient *http.Client
}

func NewBot(url string) *Bot {
	return &Bot{
		url:        url,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (bot *Bot) SendUpdate(update domain.LinkUpdate) error {
	type request struct {
		Id          int64   `json:"id"`
		Url         string  `json:"url"`
		Description string  `json:"description"`
		TgChatIds   []int64 `json:"tgChatIds"`
	}

	var reqData = request{
		Id:          update.ID,
		Url:         update.URL,
		Description: update.Description,
		TgChatIds:   update.TgChatIDs,
	}

	body, err := json.Marshal(reqData)
	if err != nil {
		return fmt.Errorf("failed to marshal update request: %w", err)
	}

	reqUrl := fmt.Sprintf("%s/%s", bot.url, endpoint)
	req, err := http.NewRequest(http.MethodPost, reqUrl, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := bot.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to bot: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bot api returned unexpected status: %d", resp.StatusCode)
	}

	return nil
}
