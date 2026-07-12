package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

const (
	tgChatEndpoint = "tg-chat"
	linksEndpoint  = "links"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	timeout    time.Duration
	log        *slog.Logger
}

func NewClient(url string, httpClient *http.Client, timeout time.Duration, log *slog.Logger) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    url,
		timeout:    timeout,
		log:        log,
	}
}

func (client *Client) Close() error {
	return nil
}

func (client *Client) RegisterChat(ctx context.Context, chatId int64) error {
	client.log.Debug("registering chat",
		"id", chatId,
	)

	ctx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()

	query := fmt.Sprintf("%s/%s/%d", client.baseURL, tgChatEndpoint, chatId)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, query, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("register chat timed out: %w", err)
		}
		return fmt.Errorf("failed to send request to scrapper: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusConflict {
			return domain.ErrChatAlreadyRegistered
		}
		return fmt.Errorf("scrapper api returned unexpected status: %d", resp.StatusCode)
	}

	client.log.Debug("chat registered",
		"id", chatId,
	)

	return nil
}

func (client *Client) DeleteChat(ctx context.Context, chatId int64) error {
	client.log.Debug("deleting chat",
		"id", chatId,
	)

	ctx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()

	query := fmt.Sprintf("%s/%s/%d", client.baseURL, tgChatEndpoint, chatId)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, query, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("delete chat timed out: %w", err)
		}
		return fmt.Errorf("failed to send request to scrapper: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return domain.ErrChatNotRegistered
		}
		return fmt.Errorf("scrapper api returned unexpected status: %d", resp.StatusCode)
	}

	client.log.Debug("chat deleted",
		"id", chatId,
	)

	return nil
}

type linkJson struct {
	Id   int64    `json:"id"`
	Url  string   `json:"url"`
	Tags []string `json:"tags"`
}

func (client *Client) GetTrackedLinks(ctx context.Context, chatId int64) ([]domain.TrackedLink, error) {
	client.log.Debug("getting tracked links",
		"id", chatId,
	)

	ctx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()

	query := fmt.Sprintf("%s/%s", client.baseURL, linksEndpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Tg-Chat-Id", fmt.Sprintf("%d", chatId))

	resp, err := client.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("get tracked links timed out: %w", err)
		}
		return nil, fmt.Errorf("failed to send request to scrapper: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, domain.ErrChatNotRegistered
		}
		return nil, fmt.Errorf("scrapper api returned unexpected status: %d", resp.StatusCode)
	}

	var linksJson struct {
		Links []linkJson `json:"links"`
		Size  int32      `json:"size"`
	}

	data, err := io.ReadAll(resp.Body)
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

	client.log.Debug("got tracked links",
		"id", chatId,
		"count", linksJson.Size,
	)

	return links, nil
}

func (client *Client) AddLink(ctx context.Context, chatId int64, url string, tags []string) (domain.TrackedLink, error) {
	client.log.Debug("adding link",
		"id", chatId,
		"url", url,
		"tags", tags,
	)

	ctx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()

	query := fmt.Sprintf("%s/%s", client.baseURL, linksEndpoint)

	reqJson := struct {
		Link string   `json:"link"`
		Tags []string `json:"tags"`
	}{
		Link: url,
		Tags: tags,
	}

	reqBody, err := json.Marshal(reqJson)
	if err != nil {
		return domain.TrackedLink{}, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, query, bytes.NewReader(reqBody))
	if err != nil {
		return domain.TrackedLink{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Tg-Chat-Id", fmt.Sprintf("%d", chatId))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return domain.TrackedLink{}, fmt.Errorf("add link timed out: %w", err)
		}
		return domain.TrackedLink{}, fmt.Errorf("failed to send request to scrapper: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return domain.TrackedLink{}, domain.ErrChatNotRegistered
		}
		if resp.StatusCode == http.StatusConflict {
			return domain.TrackedLink{}, domain.ErrAlreadySubscribed
		}
		if resp.StatusCode == http.StatusUnprocessableEntity {
			return domain.TrackedLink{}, domain.ErrURLNotSupported
		}
		return domain.TrackedLink{}, fmt.Errorf("scrapper api returned unexpected status: %d", resp.StatusCode)
	}

	var link linkJson

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.TrackedLink{}, fmt.Errorf("failed to read response body: %w", err)
	}

	err = json.Unmarshal(data, &link)
	if err != nil {
		return domain.TrackedLink{}, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	client.log.Debug("add link",
		"id", chatId,
		"url", url,
		"id", link.Id,
	)

	return domain.TrackedLink{
		ID:   link.Id,
		URL:  link.Url,
		Tags: link.Tags,
	}, nil
}

func (client *Client) RemoveLink(ctx context.Context, chatId int64, url string) (domain.TrackedLink, error) {
	client.log.Debug("removing link",
		"id", chatId,
		"url", url,
	)

	ctx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()

	query := fmt.Sprintf("%s/%s", client.baseURL, linksEndpoint)

	reqJson := struct {
		Link string `json:"link"`
	}{
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

	resp, err := client.httpClient.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return domain.TrackedLink{}, fmt.Errorf("remove link timed out: %w", err)
		}
		return domain.TrackedLink{}, fmt.Errorf("failed to send request to scrapper: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return domain.TrackedLink{}, domain.ErrChatNotRegisteredOrLinkNotFound
		}
		return domain.TrackedLink{}, fmt.Errorf("scrapper api returned unexpected status: %d", resp.StatusCode)
	}

	var link linkJson

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.TrackedLink{}, fmt.Errorf("failed to read response body: %w", err)
	}

	err = json.Unmarshal(data, &link)
	if err != nil {
		return domain.TrackedLink{}, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	client.log.Debug("link removed",
		"id", chatId,
		"url", url,
		"id", link.Id,
	)

	return domain.TrackedLink{
		ID:   link.Id,
		URL:  link.Url,
		Tags: link.Tags,
	}, nil
}
