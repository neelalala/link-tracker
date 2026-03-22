package scrapper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	scrapperapplication "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	scrapperdomain "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"io"
	"net/http"
)

const (
	registerTgChatEndpoint = "tg-chat"
	deleteTgChatEndpoint   = "tg-chat"
	getLinksEndpoint       = "links"
	trackLinksEndpoint     = "links"
	deleteLinksEndpoint    = "links"
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
	query := fmt.Sprintf("%s/%s/%d", client.baseURL, registerTgChatEndpoint, chatId)
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
			return scrapperdomain.ErrChatAlreadyRegistered
		}
		return fmt.Errorf("scrapper api returned unexpected status: %d", response.StatusCode)
	}

	return nil
}

func (client *Client) DeleteChat(ctx context.Context, chatId int64) error {
	query := fmt.Sprintf("%s/%s/%d", client.baseURL, deleteTgChatEndpoint, chatId)
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
			return scrapperdomain.ErrChatNotRegistered
		}
		return fmt.Errorf("scrapper api returned unexpected status: %d", response.StatusCode)
	}

	return nil
}

func (client *Client) GetTrackedLinks(ctx context.Context, chatId int64) ([]scrapperdomain.TrackedLink, error) {
	query := fmt.Sprintf("%s/%s", client.baseURL, getLinksEndpoint)
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
			return nil, scrapperdomain.ErrChatNotRegistered
		}
		return nil, fmt.Errorf("scrapper api returned unexpected status: %d", response.StatusCode)
	}

	type linkJson struct {
		Id   int64    `json:"id"`
		Url  string   `json:"url"`
		Tags []string `json:"tags"`
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

	var links []scrapperdomain.TrackedLink
	for _, link := range linksJson.Links {
		links = append(links, scrapperdomain.TrackedLink{
			ID:   link.Id,
			URL:  link.Url,
			Tags: link.Tags,
		})
	}

	return links, nil
}

func (client *Client) AddLink(ctx context.Context, chatId int64, url string, tags []string) (scrapperdomain.TrackedLink, error) {
	query := fmt.Sprintf("%s/%s", client.baseURL, trackLinksEndpoint)

	type requestJson struct {
		Link string   `json:"link"`
		Tags []string `json:"tags"`
	}

	var reqJson = requestJson{
		Link: url,
		Tags: tags,
	}

	reqBody, err := json.Marshal(reqJson)
	if err != nil {
		return scrapperdomain.TrackedLink{}, fmt.Errorf("failed to marshal request body: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, query, bytes.NewReader(reqBody))
	if err != nil {
		return scrapperdomain.TrackedLink{}, fmt.Errorf("failed to create request: %w", err)
	}

	request.Header.Set("Tg-Chat-Id", fmt.Sprintf("%d", chatId))
	request.Header.Set("Content-Type", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return scrapperdomain.TrackedLink{}, fmt.Errorf("failed to send request to scrapper: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusNotFound {
			return scrapperdomain.TrackedLink{}, scrapperdomain.ErrChatNotRegistered
		}
		if response.StatusCode == http.StatusConflict {
			return scrapperdomain.TrackedLink{}, scrapperdomain.ErrAlreadySubscribed
		}
		if response.StatusCode == http.StatusUnprocessableEntity {
			return scrapperdomain.TrackedLink{}, scrapperapplication.ErrUrlNotSupported
		}
		return scrapperdomain.TrackedLink{}, fmt.Errorf("scrapper api returned unexpected status: %d", response.StatusCode)
	}

	type responseJson struct {
		Id   int64    `json:"id"`
		Url  string   `json:"url"`
		Tags []string `json:"tags"`
	}

	var respJson responseJson

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return scrapperdomain.TrackedLink{}, fmt.Errorf("failed to read response body: %w", err)
	}

	err = json.Unmarshal(data, &respJson)
	if err != nil {
		return scrapperdomain.TrackedLink{}, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return scrapperdomain.TrackedLink{
		ID:   respJson.Id,
		URL:  respJson.Url,
		Tags: respJson.Tags,
	}, nil
}

func (client *Client) RemoveLink(ctx context.Context, chatId int64, url string) (scrapperdomain.TrackedLink, error) {
	query := fmt.Sprintf("%s/%s", client.baseURL, deleteLinksEndpoint)

	type requestJson struct {
		Link string `json:"link"`
	}

	var reqJson = requestJson{
		Link: url,
	}

	reqBody, err := json.Marshal(reqJson)
	if err != nil {
		return scrapperdomain.TrackedLink{}, fmt.Errorf("failed to marshal request body: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, query, bytes.NewReader(reqBody))
	if err != nil {
		return scrapperdomain.TrackedLink{}, fmt.Errorf("failed to create request: %w", err)
	}

	request.Header.Set("Tg-Chat-Id", fmt.Sprintf("%d", chatId))
	request.Header.Set("Content-Type", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return scrapperdomain.TrackedLink{}, fmt.Errorf("failed to send request to scrapper: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusNotFound {
			return scrapperdomain.TrackedLink{}, fmt.Errorf("chat not registered or link not found")
		}

		return scrapperdomain.TrackedLink{}, fmt.Errorf("scrapper api returned unexpected status: %d", response.StatusCode)
	}

	type responseJson struct {
		Id   int64    `json:"id"`
		Url  string   `json:"url"`
		Tags []string `json:"tags"`
	}

	var respJson responseJson

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return scrapperdomain.TrackedLink{}, fmt.Errorf("failed to read response body: %w", err)
	}

	err = json.Unmarshal(data, &respJson)
	if err != nil {
		return scrapperdomain.TrackedLink{}, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return scrapperdomain.TrackedLink{
		ID:   respJson.Id,
		URL:  respJson.Url,
		Tags: respJson.Tags,
	}, nil
}
