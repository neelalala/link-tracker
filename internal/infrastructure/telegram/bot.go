package telegram

import (
	"encoding/json"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"io"
	"net/http"
	"time"
)

const baseUrl = "https://api.telegram.org/bot"
const timeout = 60

type Bot struct {
	offset int64
	url    string
	client *http.Client
	router *application.Router
}

func NewBot(token string, router *application.Router) *Bot {
	bot := &Bot{
		offset: 0,
		url:    baseUrl + token,
		client: &http.Client{Timeout: timeout * time.Second},
		router: router,
	}
	return bot
}

func (b *Bot) SendMessage(chatID int64, text string) error {
	query := fmt.Sprintf(`%s/sendMessage?chat_id=%d&text=%s`, b.url, chatID, text)

	resp, err := b.client.Get(query)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	result := struct {
		Ok          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
	}{}

	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}

	if !result.Ok {
		return fmt.Errorf(result.Description)
	}

	return nil
}

func (b *Bot) GetUpdates() ([]domain.Message, error) {
	query := fmt.Sprintf(`%s/getUpdates?timeout=%d&offset=%d&allowed_updates=["message"]`, b.url, timeout, b.offset)

	resp, err := b.client.Get(query)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	result := struct {
		Ok     bool `json:"ok"`
		Result []struct {
			UpdateID int64 `json:"update_id"`
			Message  struct {
				From struct {
					ID        int64  `json:"id"`
					FirstName string `json:"first_name"`
					LastName  string `json:"last_name"`
					Username  string `json:"username"`
				} `json:"from"`
				Text string `json:"text"`
			} `json:"message"`
		} `json:"result"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
	}{}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if !result.Ok {
		return nil, fmt.Errorf(result.Description)
	}

	updates := make([]domain.Message, len(result.Result))
	for i, r := range result.Result {
		updates[i] = domain.Message{
			ID: r.UpdateID,
			From: domain.User{
				Name:     fmt.Sprintf("%s %s", r.Message.From.FirstName, r.Message.From.LastName),
				Username: r.Message.From.Username,
				UserID:   r.Message.From.ID,
			},
			Text: r.Message.Text,
		}

		b.offset = max(b.offset, r.UpdateID)
	}
	b.offset++

	return updates, nil
}
