package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

const (
	httpClientTimeout = 30 * time.Second
)

type Client struct {
	offset     int64
	url        string
	httpClient *http.Client
	timeout    time.Duration
}

func NewClient(apiURL, token string, timeout time.Duration) (*Client, error) {
	client := &Client{
		offset:     0,
		url:        apiURL + token,
		httpClient: &http.Client{Timeout: timeout + httpClientTimeout},
		timeout:    timeout,
	}

	ctx, cancel := context.WithTimeout(context.Background(), client.timeout)
	defer cancel()

	query := fmt.Sprintf("%s/getMe", client.url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, query, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating http request: %w", err)
	}

	response, err := client.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("register bot timed out: %w", err)
		}
		return nil, fmt.Errorf("error registering telegram bot: %w", err)
	}
	result := struct {
		Ok          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
	}{}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body from telegram: %w", err)
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error unmarshalling reposnse body: %w", err)
	}

	if !result.Ok {
		return nil, fmt.Errorf("%s", result.Description)
	}

	return client, nil
}

func (client *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	ctx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()

	query := fmt.Sprintf(`%s/sendMessage`, client.url)

	reqJson := struct {
		ChatID int64  `json:"chat_id"`
		Text   string `json:"text"`
	}{
		ChatID: chatID,
		Text:   text,
	}

	reqBody, err := json.Marshal(&reqJson)
	if err != nil {
		return fmt.Errorf("error marshalling request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, query, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("error creaing http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("send message timed out: %w", err)
		}
		return fmt.Errorf("error sending http request: %w", err)
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
		return fmt.Errorf("error unmarshalling response body: %w", err)
	}

	if !result.Ok {
		return fmt.Errorf("%s", result.Description)
	}

	return nil
}

func (client *Client) GetUpdates(ctx context.Context) ([]domain.Message, error) {
	query := fmt.Sprintf(`%s/getUpdates?timeout=%d&offset=%d&allowed_updates=["message"]`, client.url, int(client.timeout.Seconds()), client.offset)

	ctx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, query, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating http request: %w", err)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("get updates timed out: %w", err)
		}
		return nil, fmt.Errorf("error sending request: %w", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
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
		return nil, fmt.Errorf("error unmarshalling response body: %w", err)
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
	ctx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()

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
		return fmt.Errorf("error marshalling request body: %w", err)
	}

	setCtx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(setCtx, http.MethodPost, query, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("error creating http request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("set commands timed out: %w", err)
		}
		return fmt.Errorf("error senfing http request: %w", err)
	}
	defer response.Body.Close()

	return nil
}
