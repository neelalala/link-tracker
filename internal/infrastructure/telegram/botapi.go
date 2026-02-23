package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"io"
	"net/http"
	"time"
)

const baseUrl = "https://api.telegram.org/bot"
const timeout = 60

type BotApi struct {
	offset int64
	url    string
	client *http.Client
}

func NewBot(token string) (*BotApi, error) {
	bot := &BotApi{
		offset: 0,
		url:    baseUrl + token,
		client: &http.Client{Timeout: timeout*time.Second + 10*time.Second},
	}

	query := fmt.Sprintf("%s/getMe", bot.url)
	resp, err := bot.client.Get(query)
	if err != nil {
		return nil, err
	}
	result := struct {
		Ok          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
	}{}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if !result.Ok {
		return nil, fmt.Errorf(result.Description)
	}

	return bot, nil
}

func (b *BotApi) SendMessage(chatID int64, text string) error {
	query := fmt.Sprintf(`%s/sendMessage`, b.url)

	type botMessage struct {
		ChatID int64  `json:"chat_id"`
		Text   string `json:"text"`
	}

	payload := botMessage{
		ChatID: chatID,
		Text:   text,
	}

	bodyReq, err := json.Marshal(&payload)
	if err != nil {
		return err
	}

	resp, err := b.client.Post(query, "application/json", bytes.NewReader(bodyReq))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	bodyResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	result := struct {
		Ok          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
	}{}

	if err := json.Unmarshal(bodyResp, &result); err != nil {
		return err
	}

	if !result.Ok {
		return fmt.Errorf(result.Description)
	}

	return nil
}

func (b *BotApi) GetUpdates() ([]domain.Message, error) {
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
				Chat struct {
					ID int64 `json:"id"`
				} `json:"chat"`
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
			ID:     r.UpdateID,
			ChatID: r.Message.Chat.ID,
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

func (b *BotApi) SetMyCommands(cmds []domain.Command) error {
	query := fmt.Sprintf("%s/setMyCommands", b.url)

	type botCommand struct {
		Command     string `json:"command"`
		Description string `json:"description"`
	}

	var botCommands []botCommand
	for _, cmd := range cmds {
		botCommands = append(botCommands, botCommand{
			Command:     cmd.Name,
			Description: cmd.Description,
		})
	}

	payload := map[string]any{
		"commands": botCommands,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := b.client.Post(query, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
