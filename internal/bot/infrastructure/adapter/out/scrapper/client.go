package scrapper

import (
	"bytes"
	"encoding/json"
	"fmt"
	scrapper_domain "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
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

func (client *Client) RegisterChat(chatId int64) error {
	reqUrl := fmt.Sprintf("%s/%s/%d", client.baseURL, registerTgChatEndpoint, chatId)
	req, err := http.NewRequest(http.MethodPost, reqUrl, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to scrapper: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusConflict {
			return scrapper_domain.ErrChatAlreadyRegistered
		}
		return fmt.Errorf("scrapper api returned unexpected status: %d", resp.StatusCode)
	}

	return nil
}

func (client *Client) DeleteChat(chatId int64) error {
	reqUrl := fmt.Sprintf("%s/%s/%d", client.baseURL, deleteTgChatEndpoint, chatId)
	req, err := http.NewRequest(http.MethodDelete, reqUrl, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to scrapper: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return scrapper_domain.ErrChatNotRegistered
		}
		return fmt.Errorf("scrapper api returned unexpected status: %d", resp.StatusCode)
	}

	return nil
}

func (client *Client) GetTrackedLinks(chatId int64) ([]scrapper_domain.TrackedLink, error) {
	reqUrl := fmt.Sprintf("%s/%s", client.baseURL, getLinksEndpoint)
	req, err := http.NewRequest(http.MethodGet, reqUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Tg-Chat-Id", fmt.Sprintf("%d", chatId))

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to scrapper: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, scrapper_domain.ErrChatNotRegistered
		}
		return nil, fmt.Errorf("scrapper api returned unexpected status: %d", resp.StatusCode)
	}

	type linkJson struct {
		Id      int64    `json:"id"`
		Url     string   `json:"url"`
		Tags    []string `json:"tags"`
		Filters []string `json:"filters"`
	}

	type responseJson struct {
		Links []linkJson `json:"links"`
		Size  int32      `json:"size"`
	}

	var linksJson responseJson

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	err = json.Unmarshal(data, &linksJson)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	var links []scrapper_domain.TrackedLink
	for _, link := range linksJson.Links {
		links = append(links, scrapper_domain.TrackedLink{
			ID:      link.Id,
			URL:     link.Url,
			Tags:    link.Tags,
			Filters: link.Filters,
		})
	}

	return links, nil
}

func (client *Client) AddLink(chatId int64, url string, tags, filters []string) (scrapper_domain.TrackedLink, error) {
	reqUrl := fmt.Sprintf("%s/%s", client.baseURL, trackLinksEndpoint)

	type requestJson struct {
		Link    string   `json:"link"`
		Tags    []string `json:"tags"`
		Filters []string `json:"filters"`
	}

	var reqJson = requestJson{
		Link:    url,
		Tags:    tags,
		Filters: filters,
	}

	reqBody, err := json.Marshal(reqJson)
	if err != nil {
		return scrapper_domain.TrackedLink{}, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, reqUrl, bytes.NewReader(reqBody))
	if err != nil {
		return scrapper_domain.TrackedLink{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Tg-Chat-Id", fmt.Sprintf("%d", chatId))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return scrapper_domain.TrackedLink{}, fmt.Errorf("failed to send request to scrapper: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return scrapper_domain.TrackedLink{}, scrapper_domain.ErrChatNotRegistered
		}
		if resp.StatusCode == http.StatusConflict {
			return scrapper_domain.TrackedLink{}, scrapper_domain.ErrAlreadySubscribed
		}
		return scrapper_domain.TrackedLink{}, fmt.Errorf("scrapper api returned unexpected status: %d", resp.StatusCode)
	}

	type responseJson struct {
		Id      int64    `json:"id"`
		Url     string   `json:"url"`
		Tags    []string `json:"tags"`
		Filters []string `json:"filters"`
	}

	var response responseJson

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return scrapper_domain.TrackedLink{}, fmt.Errorf("failed to read response body: %w", err)
	}

	err = json.Unmarshal(data, &response)
	if err != nil {
		return scrapper_domain.TrackedLink{}, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return scrapper_domain.TrackedLink{
		ID:      response.Id,
		URL:     response.Url,
		Tags:    response.Tags,
		Filters: response.Filters,
	}, nil
}

func (client *Client) RemoveLink(chatId int64, url string) (scrapper_domain.TrackedLink, error) {
	reqUrl := fmt.Sprintf("%s/%s", client.baseURL, deleteLinksEndpoint)

	type requestJson struct {
		Link string `json:"link"`
	}

	var reqJson = requestJson{
		Link: url,
	}

	reqBody, err := json.Marshal(reqJson)
	if err != nil {
		return scrapper_domain.TrackedLink{}, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodDelete, reqUrl, bytes.NewReader(reqBody))
	if err != nil {
		return scrapper_domain.TrackedLink{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Tg-Chat-Id", fmt.Sprintf("%d", chatId))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return scrapper_domain.TrackedLink{}, fmt.Errorf("failed to send request to scrapper: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return scrapper_domain.TrackedLink{}, fmt.Errorf("chat not registered or link not found")
		}

		return scrapper_domain.TrackedLink{}, fmt.Errorf("scrapper api returned unexpected status: %d", resp.StatusCode)
	}

	type responseJson struct {
		Id      int64    `json:"id"`
		Url     string   `json:"url"`
		Tags    []string `json:"tags"`
		Filters []string `json:"filters"`
	}

	var response responseJson

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return scrapper_domain.TrackedLink{}, fmt.Errorf("failed to read response body: %w", err)
	}

	err = json.Unmarshal(data, &response)
	if err != nil {
		return scrapper_domain.TrackedLink{}, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return scrapper_domain.TrackedLink{
		ID:      response.Id,
		URL:     response.Url,
		Tags:    response.Tags,
		Filters: response.Filters,
	}, nil
}
