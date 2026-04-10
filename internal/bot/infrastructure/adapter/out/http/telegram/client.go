package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Client struct {
	offset  int64
	url     string
	client  *http.Client
	timeout time.Duration
}

func NewClient(apiUrl, token string, timeout time.Duration) (*Client, error) {
	client := &Client{
		offset:  0,
		url:     apiUrl + token,
		client:  &http.Client{Timeout: timeout*time.Second + 10*time.Second},
		timeout: timeout,
	}

	query := fmt.Sprintf("%s/getMe", client.url)
	response, err := client.client.Get(query)
	if err != nil {
		return nil, err
	}
	result := struct {
		Ok          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
	}{}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if !result.Ok {
		return nil, fmt.Errorf("%s", result.Description)
	}

	return client, nil
}

func (client *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	query := fmt.Sprintf(`%s/sendMessage`, client.url)

	type requestJson struct {
		ChatID int64  `json:"chat_id"`
		Text   string `json:"text"`
	}

	reqJson := requestJson{
		ChatID: chatID,
		Text:   text,
	}

	reqBody, err := json.Marshal(&reqJson)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, query, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := client.client.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	bodyResp, err := io.ReadAll(response.Body)
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
		return fmt.Errorf("%s", result.Description)
	}

	return nil
}

func (client *Client) GetUpdates(ctx context.Context) ([]domain.Message, error) {
	query := fmt.Sprintf(`%s/getUpdates?timeout=%d&offset=%d&allowed_updates=["message"]`, client.url, int(client.timeout.Seconds()), client.offset)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, query, nil)
	if err != nil {
		return nil, err
	}

	response, err := client.client.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
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
		return nil, fmt.Errorf("%s", result.Description)
	}

	updates := make([]domain.Message, len(result.Result))
	for i, res := range result.Result {
		updates[i] = domain.Message{
			ID:     res.UpdateID,
			ChatID: res.Message.Chat.ID,
			Text:   res.Message.Text,
		}

		client.offset = max(client.offset, res.UpdateID)
	}
	client.offset++

	return updates, nil
}

func (client *Client) SetMyCommands(ctx context.Context, cmds []domain.CommandInfo) error {
	query := fmt.Sprintf("%s/setMyCommands", client.url)

	type botCommandJson struct {
		Command     string `json:"command"`
		Description string `json:"description"`
	}

	var botCommands []botCommandJson
	for _, cmd := range cmds {
		botCommands = append(botCommands, botCommandJson{
			Command:     cmd.Name,
			Description: cmd.Description,
		})
	}

	body := map[string]any{
		"commands": botCommands,
	}

	reqBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, query, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return nil
}
