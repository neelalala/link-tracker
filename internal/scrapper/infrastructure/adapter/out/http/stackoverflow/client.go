package stackoverflow

import (
	"context"
	"encoding/json"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"net/http"
	"strings"
	"time"
)

const (
	baseURL    = "https://stackoverflow.com/questions/"
	baseApiURL = "https://api.stackexchange.com/2.3"
	timeout    = 10 * time.Second
)

type Client struct {
	httpClient *http.Client
	apiURL     string
	baseURL    string
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		apiURL:     baseApiURL,
		baseURL:    baseURL,
	}
}

func (c *Client) CanHandle(url string) bool {
	return strings.HasPrefix(url, c.baseURL)
}

func (c *Client) Fetch(ctx context.Context, url string) (application.FetchResult, error) {
	path := strings.TrimPrefix(url, c.baseURL)
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		return application.FetchResult{}, fmt.Errorf("invalid stackoverflow url: %s", url)
	}

	questionID := parts[0]

	apiUrl := fmt.Sprintf("%s/questions/%s?site=stackoverflow.com", c.apiURL, questionID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiUrl, nil)
	if err != nil {
		return application.FetchResult{}, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return application.FetchResult{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return application.FetchResult{}, fmt.Errorf("stackoverflow api returned status: %d for %s", resp.StatusCode, apiUrl)
	}

	var apiResponse struct {
		Items []struct {
			LastActivityDate int64  `json:"last_activity_date"`
			Title            string `json:"title"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return application.FetchResult{}, err
	}

	if len(apiResponse.Items) == 0 {
		return application.FetchResult{}, fmt.Errorf("question not found for url: %s", url)
	}

	questionData := apiResponse.Items[0]

	return application.FetchResult{
		UpdatedAt:   time.Unix(questionData.LastActivityDate, 0),
		Description: fmt.Sprintf("Question '%s' was updated", questionData.Title),
	}, nil
}
