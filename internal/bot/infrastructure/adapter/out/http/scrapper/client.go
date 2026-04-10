package scrapper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

const (
	tgChatEndpoint = "tg-chat"
	linksEndpoint  = "links"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(url string) *Client {
	return &Client{
		httpClient: &http.Client{},
		baseURL:    url,
	}
}

func (client *Client) RegisterChat(ctx context.Context, chatId int64) error {
	query := fmt.Sprintf("%s/%s/%d", client.baseURL, tgChatEndpoint, chatId)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, query, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed to send request to scrapper: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusConflict {
			return domain.ErrChatAlreadyRegistered
		}
		return fmt.Errorf("scrapper api returned unexpected status: %d", response.StatusCode)
	}

	return nil
}

func (client *Client) DeleteChat(ctx context.Context, chatId int64) error {
	query := fmt.Sprintf("%s/%s/%d", client.baseURL, tgChatEndpoint, chatId)
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, query, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed to send request to scrapper: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusNotFound {
			return domain.ErrChatNotRegistered
		}
		return fmt.Errorf("scrapper api returned unexpected status: %d", response.StatusCode)
	}

	return nil
}

type linkJson struct {
	Id   int64    `json:"id"`
	Url  string   `json:"url"`
	Tags []string `json:"tags"`
}

func (client *Client) GetTrackedLinks(ctx context.Context, chatId int64) ([]domain.TrackedLink, error) {
	query := fmt.Sprintf("%s/%s", client.baseURL, linksEndpoint)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	request.Header.Set("Tg-Chat-Id", fmt.Sprintf("%d", chatId))

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to scrapper: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusNotFound {
			return nil, domain.ErrChatNotRegistered
		}
		return nil, fmt.Errorf("scrapper api returned unexpected status: %d", response.StatusCode)
	}

	type responseJson struct {
		Links []linkJson `json:"links"`
		Size  int32      `json:"size"`
	}

	var linksJson responseJson

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	err = json.Unmarshal(data, &linksJson)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	var links []domain.TrackedLink
	for _, link := range linksJson.Links {
		links = append(links, domain.TrackedLink{
			ID:   link.Id,
			URL:  link.Url,
			Tags: link.Tags,
		})
	}

	return links, nil
}

func (client *Client) AddLink(ctx context.Context, chatId int64, url string, tags []string) (domain.TrackedLink, error) {
	query := fmt.Sprintf("%s/%s", client.baseURL, linksEndpoint)

	type requestJson struct {
		Link string   `json:"link"`
		Tags []string `json:"tags"`
	}

	reqJson := requestJson{
		Link: url,
		Tags: tags,
	}

	reqBody, err := json.Marshal(reqJson)
	if err != nil {
		return domain.TrackedLink{}, fmt.Errorf("failed to marshal request body: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, query, bytes.NewReader(reqBody))
	if err != nil {
		return domain.TrackedLink{}, fmt.Errorf("failed to create request: %w", err)
	}

	request.Header.Set("Tg-Chat-Id", fmt.Sprintf("%d", chatId))
	request.Header.Set("Content-Type", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return domain.TrackedLink{}, fmt.Errorf("failed to send request to scrapper: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusNotFound {
			return domain.TrackedLink{}, domain.ErrChatNotRegistered
		}
		if response.StatusCode == http.StatusConflict {
			return domain.TrackedLink{}, domain.ErrAlreadySubscribed
		}
		if response.StatusCode == http.StatusUnprocessableEntity {
			return domain.TrackedLink{}, domain.ErrUrlNotSupported
		}
		return domain.TrackedLink{}, fmt.Errorf("scrapper api returned unexpected status: %d", response.StatusCode)
	}

	var respJson linkJson

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return domain.TrackedLink{}, fmt.Errorf("failed to read response body: %w", err)
	}

	err = json.Unmarshal(data, &respJson)
	if err != nil {
		return domain.TrackedLink{}, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return domain.TrackedLink{
		ID:   respJson.Id,
		URL:  respJson.Url,
		Tags: respJson.Tags,
	}, nil
}

func (client *Client) RemoveLink(ctx context.Context, chatId int64, url string) (domain.TrackedLink, error) {
	query := fmt.Sprintf("%s/%s", client.baseURL, linksEndpoint)

	type requestJson struct {
		Link string `json:"link"`
	}

	reqJson := requestJson{
		Link: url,
	}

	reqBody, err := json.Marshal(reqJson)
	if err != nil {
		return domain.TrackedLink{}, fmt.Errorf("failed to marshal request body: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, query, bytes.NewReader(reqBody))
	if err != nil {
		return domain.TrackedLink{}, fmt.Errorf("failed to create request: %w", err)
	}

	request.Header.Set("Tg-Chat-Id", fmt.Sprintf("%d", chatId))
	request.Header.Set("Content-Type", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return domain.TrackedLink{}, fmt.Errorf("failed to send request to scrapper: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusNotFound {
			return domain.TrackedLink{}, domain.ErrChatNotRegisteredOrLinkNotFound
		}

		return domain.TrackedLink{}, fmt.Errorf("scrapper api returned unexpected status: %d", response.StatusCode)
	}

	var respJson linkJson

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return domain.TrackedLink{}, fmt.Errorf("failed to read response body: %w", err)
	}

	err = json.Unmarshal(data, &respJson)
	if err != nil {
		return domain.TrackedLink{}, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return domain.TrackedLink{
		ID:   respJson.Id,
		URL:  respJson.Url,
		Tags: respJson.Tags,
	}, nil
}
