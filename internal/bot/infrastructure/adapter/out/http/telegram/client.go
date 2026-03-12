package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"io"
	"net/http"
	"time"
)

const baseUrl = "https://api.telegram.org/bot"
const timeout = 60

type Client struct {
	offset int64
	url    string
	client *http.Client
}

func NewClient(token string) (*Client, error) {
	tgClient := &Client{
		offset: 0,
		url:    baseUrl + token,
		client: &http.Client{Timeout: timeout*time.Second + 10*time.Second},
	}

	query := fmt.Sprintf("%s/getMe", tgClient.url)
	resp, err := tgClient.client.Get(query)
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
		return nil, fmt.Errorf("%s", result.Description)
	}

	return tgClient, nil
}

func (api *Client) SendMessage(chatID int64, text string) error {
	query := fmt.Sprintf(`%s/sendMessage`, api.url)

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

	resp, err := api.client.Post(query, "application/json", bytes.NewReader(bodyReq))
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
		return fmt.Errorf("%s", result.Description)
	}

	return nil
}

func (api *Client) GetUpdates() ([]domain.Message, error) {
	query := fmt.Sprintf(`%s/getUpdates?timeout=%d&offset=%d&allowed_updates=["message"]`, api.url, timeout, api.offset)

	resp, err := api.client.Get(query)
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
		return nil, fmt.Errorf("%s", result.Description)
	}

	updates := make([]domain.Message, len(result.Result))
	for i, res := range result.Result {
		updates[i] = domain.Message{
			ID:     res.UpdateID,
			ChatID: res.Message.Chat.ID,
			Text:   res.Message.Text,
		}

		api.offset = max(api.offset, res.UpdateID)
	}
	api.offset++

	return updates, nil
}

func (api *Client) SetMyCommands(cmds []domain.Command) error {
	query := fmt.Sprintf("%s/setMyCommands", api.url)

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

	resp, err := api.client.Post(query, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
