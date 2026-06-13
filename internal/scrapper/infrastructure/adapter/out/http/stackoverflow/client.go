package stackoverflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

const (
	BaseURL    = "https://stackoverflow.com/questions/"
	BaseApiURL = "https://api.stackexchange.com/2.3"

	site = "stackoverflow.com"
)

type Client struct {
	httpClient    *http.Client
	apiURL        string
	baseURL       string
	maxPreviewLen int
	keyQuery      string
	timeout       time.Duration
}

func NewClient(httpClient *http.Client, baseURL, baseAPIURL string, timeout time.Duration, maxPreviewLen int, key string) *Client {
	if key != "" {
		key = fmt.Sprintf("&key=%s", key)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		httpClient:    httpClient,
		apiURL:        baseAPIURL,
		baseURL:       baseURL,
		maxPreviewLen: maxPreviewLen,
		keyQuery:      key,
		timeout:       timeout,
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

	ctx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()

	questionID := parts[0]

	title, err := client.fetchQuestionTitle(ctx, questionID)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("fetch question title timed out: %w", err)
		}
		return nil, fmt.Errorf("error getting title for question with id %s: %w", questionID, err)
	}

	questionURL := fmt.Sprintf("%s/questions/%s", client.apiURL, questionID)

	answers, err := client.fetchAnswers(ctx, questionURL, since, title)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("fetch answers timed out: %w", err)
		}
		return nil, fmt.Errorf("error getting question answers: %w", err)
	}

	comments, err := client.fetchComments(ctx, questionURL, since, title)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("fetch comments timed out: %w", err)
		}
		return nil, fmt.Errorf("error getting question comments: %w", err)
	}

	updates := make([]domain.UpdateEvent, 0, len(answers)+len(comments))
	updates = append(updates, answers...)
	updates = append(updates, comments...)

	return updates, nil
}

func (client *Client) fetchQuestionTitle(ctx context.Context, questionID string) (string, error) {
	apiURL := fmt.Sprintf("%s/questions/%s?site=%s%s", client.apiURL, questionID, site, client.keyQuery)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return "", err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("stackoverflow api returned status: %d", response.StatusCode)
	}

	var soResponse struct {
		Items []struct {
			Title string `json:"title"`
		} `json:"items"`
	}

	if err := json.NewDecoder(response.Body).Decode(&soResponse); err != nil {
		return "", err
	}

	if len(soResponse.Items) == 0 {
		return "", fmt.Errorf("question not found")
	}

	return soResponse.Items[0].Title, nil
}

func (client *Client) fetchAnswers(ctx context.Context, questionURL string, since time.Time, questionTitle string) ([]domain.UpdateEvent, error) {
	apiURL := fmt.Sprintf("%s/answers?site=%s&filter=withbody&fromdate=%d%s", questionURL, site, since.Unix(), client.keyQuery)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stackoverflow api returned status: %d", response.StatusCode)
	}

	var answers struct {
		Items []struct {
			Owner struct {
				DisplayName string `json:"display_name"`
			} `json:"owner"`
			CreationDate int64  `json:"creation_date"`
			Body         string `json:"body"`
		} `json:"items"`
	}

	if err := json.NewDecoder(response.Body).Decode(&answers); err != nil {
		return nil, err
	}

	var answerUpdates []domain.UpdateEvent
	for _, answer := range answers.Items {
		timestamp := time.Unix(answer.CreationDate, 0).UTC()
		if !timestamp.After(since) {
			continue
		}
		answerUpdates = append(answerUpdates, &AnswerUpdate{
			title:         questionTitle,
			owner:         answer.Owner.DisplayName,
			createdAt:     timestamp,
			body:          answer.Body,
			maxPreviewLen: client.maxPreviewLen,
		})
	}

	return answerUpdates, nil
}

func (client *Client) fetchComments(ctx context.Context, questionURL string, since time.Time, questionTitle string) ([]domain.UpdateEvent, error) {
	apiURL := fmt.Sprintf("%s/comments?site=%s&filter=withbody&fromdate=%d%s", questionURL, site, since.Unix(), client.keyQuery)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stackoverflow api returned status: %d", response.StatusCode)
	}

	var comments struct {
		Items []struct {
			Owner struct {
				DisplayName string `json:"display_name"`
			} `json:"owner"`
			CreationDate int64  `json:"creation_date"`
			Body         string `json:"body"`
		} `json:"items"`
	}

	if err := json.NewDecoder(response.Body).Decode(&comments); err != nil {
		return nil, err
	}

	var commentUpdates []domain.UpdateEvent
	for _, comment := range comments.Items {
		commentUpdates = append(commentUpdates, &CommentUpdate{
			title:         questionTitle,
			owner:         comment.Owner.DisplayName,
			createdAt:     time.Unix(comment.CreationDate, 0).UTC(),
			body:          comment.Body,
			maxPreviewLen: client.maxPreviewLen,
		})
	}

	return commentUpdates, nil
}
