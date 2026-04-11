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

// TODO better link parsing (trim, net/url)
func (client *Client) Fetch(ctx context.Context, url string, since time.Time) ([]domain.UpdateEvent, error) {
	path := strings.TrimPrefix(url, client.baseURL)
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid github url: %s", url)
	}
	owner, repo := parts[0], parts[1]

	repoURL := fmt.Sprintf("%s/repos/%s/%s", client.apiURL, owner, repo)

	updates := []domain.UpdateEvent{}

	pullRequests, err := client.fetchPullRequests(ctx, repoURL, since)
	if err != nil {
		return nil, fmt.Errorf("error fetching pull requests: %w", err)
	}

	updates = append(updates, pullRequests...)

	issues, err := client.fetchIssues(ctx, repoURL, since)
	if err != nil {
		return nil, fmt.Errorf("error fetching issues: %w", err)
	}

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

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status: %d", response.StatusCode)
	}

	defer response.Body.Close()

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

	prUpdates := []domain.UpdateEvent{}
	for _, pullRequest := range pullRequests {
		if pullRequest.CreatedAt.Before(since) {
			continue
		}
		prUpdates = append(prUpdates, &GithubNewPRUpdate{
			Title:     pullRequest.Title,
			Author:    pullRequest.User.Login,
			CreatedAt: pullRequest.CreatedAt,
			Body:      pullRequest.BodyText,
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

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status: %d", response.StatusCode)
	}

	defer response.Body.Close()

	var issues []struct {
		Title string `json:"title"`
		User  struct {
			Login string `json:"login"`
		} `json:"user"`
		BodyText  string    `json:"body_text"`
		CreatedAt time.Time `json:"created_at"`

		PullRequst *struct{} `json:"pull_request"`
	}

	if err = json.NewDecoder(response.Body).Decode(&issues); err != nil {
		return nil, err
	}

	issueUpdates := []domain.UpdateEvent{}
	for _, issue := range issues {
		if issue.CreatedAt.Before(since) {
			continue
		}
		if issue.PullRequst != nil {
			continue
		}

		issueUpdates = append(issueUpdates, &GithubNewIssueUpdate{
			Title:     issue.Title,
			Author:    issue.User.Login,
			CreatedAt: issue.CreatedAt,
			Body:      issue.BodyText,
		})
	}

	return issueUpdates, nil
}
