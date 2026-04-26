package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

const (
	BaseURL    = "https://github.com/"
	BaseApiURL = "https://api.github.com"
	Timeout    = 10 * time.Second
)

type Client struct {
	httpClient *http.Client
	apiURL     string
	baseURL    string
}

func NewClient(baseUrl, baseApiUrl string, timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		apiURL:     baseApiUrl,
		baseURL:    baseUrl,
	}
}

func (client *Client) CanHandle(url string) bool {
	return strings.HasPrefix(url, client.baseURL)
}

func (client *Client) Fetch(ctx context.Context, url string) (domain.FetchResult, error) {
	path := strings.TrimPrefix(url, client.baseURL)
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		return domain.FetchResult{}, fmt.Errorf("invalid github url: %s", url)
	}
	owner, repo := parts[0], parts[1]

	apiUrl := fmt.Sprintf("%s/repos/%s/%s", client.apiURL, owner, repo)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiUrl, nil)
	if err != nil {
		return domain.FetchResult{}, err
	}

	request.Header.Set("Accept", "application/vnd.github+json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return domain.FetchResult{}, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return domain.FetchResult{}, fmt.Errorf("github api returned status: %d for %s", response.StatusCode, apiUrl)
	}

	var repoData struct {
		PushedAt time.Time `json:"pushed_at"`
		FullName string    `json:"full_name"`
	}

	if err := json.NewDecoder(response.Body).Decode(&repoData); err != nil {
		return domain.FetchResult{}, err
	}

	return domain.FetchResult{
		UpdatedAt:   repoData.PushedAt,
		Description: fmt.Sprintf("Something new to %s was pushed", repoData.FullName),
	}, nil
}
