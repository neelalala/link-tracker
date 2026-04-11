package stackoverflow

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
	BaseURL    = "https://stackoverflow.com/questions/"
	BaseApiURL = "https://api.stackexchange.com/2.3"
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

func (client *Client) Fetch(ctx context.Context, url string, since time.Time) ([]domain.UpdateEvent, error) {
	path := strings.TrimPrefix(url, client.baseURL)
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		return nil, fmt.Errorf("invalid stackoverflow url: %s", url)
	}

	questionID := parts[0]

	query := fmt.Sprintf("%s/questions/%s?site=stackoverflow.com", client.apiURL, questionID)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, query, nil)
	if err != nil {
		return nil, err
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stackoverflow api returned status: %d for %s", response.StatusCode, query)
	}

	var responseJson struct {
		Items []struct {
			LastActivityDate int64  `json:"last_activity_date"`
			Title            string `json:"title"`
		} `json:"items"`
	}

	if err := json.NewDecoder(response.Body).Decode(&responseJson); err != nil {
		return nil, err
	}

	if len(responseJson.Items) == 0 {
		return nil, fmt.Errorf("question not found for url: %s", url)
	}

	// 	questionData := responseJson.Items[0]

	//		return application.FetchResult{
	//			UpdatedAt:   time.Unix(questionData.LastActivityDate, 0),
	//			Description: fmt.Sprintf("Question '%s' was updated", questionData.Title),
	//	}, nil

	return nil, nil
}
