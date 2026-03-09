package github

import (
	"encoding/json"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"net/http"
	"strings"
	"time"
)

const (
	baseURL    = "https://github.com/"
	baseApiURL = "https://api.github.com"
	timeout    = 10 * time.Second
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) CanHandle(url string) bool {
	return strings.HasPrefix(url, baseURL)
}

func (c *Client) Fetch(url string) (application.FetchResult, error) {
	path := strings.TrimPrefix(url, baseURL)
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		return application.FetchResult{}, fmt.Errorf("invalid github url: %s", url)
	}
	owner, repo := parts[0], parts[1]

	apiUrl := fmt.Sprintf("%s/repos/%s/%s", baseApiURL, owner, repo)

	req, err := http.NewRequest(http.MethodGet, apiUrl, nil)
	if err != nil {
		return application.FetchResult{}, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return application.FetchResult{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return application.FetchResult{}, fmt.Errorf("github api returned status: %d for %s", resp.StatusCode, apiUrl)
	}

	var repoData struct {
		PushedAt time.Time `json:"pushed_at"`
		FullName string    `json:"full_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&repoData); err != nil {
		return application.FetchResult{}, err
	}

	return application.FetchResult{
		UpdatedAt:   repoData.PushedAt,
		Description: fmt.Sprintf("Something new to %s was pushed", repoData.FullName),
	}, nil
}
