package github

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
	BaseURL    = "https://github.com/"
	BaseApiURL = "https://api.github.com"
)

type Client struct {
	httpClient        *http.Client
	apiURL            string
	baseURL           string
	maxDescriptionLen int
	timeout           time.Duration
}

func NewClient(httpClient *http.Client, baseURL, baseApiURL string, timeout time.Duration, maxDescriptionLen int) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		httpClient:        httpClient,
		apiURL:            baseApiURL,
		baseURL:           baseURL,
		maxDescriptionLen: maxDescriptionLen,
		timeout:           timeout,
	}
}

func (client *Client) CanHandle(url string) bool {
	return strings.HasPrefix(url, client.baseURL)
}

func (client *Client) Fetch(ctx context.Context, url string, since time.Time) ([]domain.UpdateEvent, error) {
	path := strings.TrimPrefix(url, client.baseURL)
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid github url: %s", url)
	}
	owner, repo := parts[0], parts[1]

	ctx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()

	repoURL := fmt.Sprintf("%s/repos/%s/%s", client.apiURL, owner, repo)

	pullRequests, err := client.fetchPullRequests(ctx, repoURL, since)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("fetch pull requests timed out: %w", err)
		}
		return nil, fmt.Errorf("error fetching pull requests: %w", err)
	}

	issues, err := client.fetchIssues(ctx, repoURL, since)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("fetch issues timed out: %w", err)
		}
		return nil, fmt.Errorf("error fetching issues: %w", err)
	}

	updates := make([]domain.UpdateEvent, 0, len(pullRequests)+len(issues))
	updates = append(updates, pullRequests...)
	updates = append(updates, issues...)

	return updates, nil
}

func (client *Client) fetchPullRequests(ctx context.Context, repoURL string, since time.Time) ([]domain.UpdateEvent, error) {
	apiURL := fmt.Sprintf("%s/pulls", repoURL)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Accept", "application/vnd.github.text+json")
	request.Header.Set("X-GitHub-Api-Version", "2026-03-10")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status: %d", response.StatusCode)
	}

	var pullRequests []struct {
		Title string `json:"title"`
		User  struct {
			Login string `json:"login"`
		} `json:"user"`
		BodyText  string    `json:"body_text"`
		CreatedAt time.Time `json:"created_at"`
	}

	if err := json.NewDecoder(response.Body).Decode(&pullRequests); err != nil {
		return nil, err
	}

	var prUpdates []domain.UpdateEvent
	for _, pullRequest := range pullRequests {
		if !pullRequest.CreatedAt.After(since) {
			continue
		}
		prUpdates = append(prUpdates, &NewPRUpdate{
			title:         pullRequest.Title,
			author:        pullRequest.User.Login,
			createdAt:     pullRequest.CreatedAt.UTC(),
			body:          pullRequest.BodyText,
			maxPreviewLen: client.maxDescriptionLen,
		})
	}

	return prUpdates, nil
}

func (client *Client) fetchIssues(ctx context.Context, repoURL string, since time.Time) ([]domain.UpdateEvent, error) {
	apiURL := fmt.Sprintf("%s/issues", repoURL)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Accept", "application/vnd.github.text+json")
	request.Header.Set("X-GitHub-Api-Version", "2026-03-10")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status: %d", response.StatusCode)
	}

	var issues []struct {
		Title string `json:"title"`
		User  struct {
			Login string `json:"login"`
		} `json:"user"`
		BodyText  string    `json:"body_text"`
		CreatedAt time.Time `json:"created_at"`

		PullRequest *struct{} `json:"pull_request"`
	}

	if err = json.NewDecoder(response.Body).Decode(&issues); err != nil {
		return nil, err
	}

	var issueUpdates []domain.UpdateEvent
	for _, issue := range issues {
		if !issue.CreatedAt.After(since) {
			continue
		}
		if issue.PullRequest != nil {
			continue
		}

		issueUpdates = append(issueUpdates, &NewIssueUpdate{
			title:         issue.Title,
			author:        issue.User.Login,
			createdAt:     issue.CreatedAt.UTC(),
			body:          issue.BodyText,
			maxPreviewLen: client.maxDescriptionLen,
		})
	}

	return issueUpdates, nil
}
