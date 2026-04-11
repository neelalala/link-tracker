package github

import (
	"fmt"
	"time"
)

type GithubNewPRUpdate struct {
	Title     string
	Author    string
	CreatedAt time.Time
	Body      string

	MaxPreviewLen int
}

func (prUpdate *GithubNewPRUpdate) UpdatedAt() time.Time {
	return prUpdate.CreatedAt
}

func (prUpdate *GithubNewPRUpdate) Description() string {
	return fmt.Sprintf("New Pull Request: %s by %s", prUpdate.Title, prUpdate.Author)
}

func (prUpdate *GithubNewPRUpdate) Preview() string {
	return truncateText(prUpdate.Body, prUpdate.MaxPreviewLen)
}

type GithubNewIssueUpdate struct {
	Title     string
	Author    string
	CreatedAt time.Time
	Body      string

	MaxPreviewLen int
}

func (issueUpdate *GithubNewIssueUpdate) UpdatedAt() time.Time {
	return issueUpdate.CreatedAt
}

func (issueUpdate *GithubNewIssueUpdate) Description() string {
	return fmt.Sprintf("New Issue: %s by %s", issueUpdate.Title, issueUpdate.Author)
}

func (issueUpdate *GithubNewIssueUpdate) Preview() string {
	return truncateText(issueUpdate.Body, issueUpdate.MaxPreviewLen)
}

func truncateText(s string, maxLen int) string {
	runes := []rune(s)

	if len(runes) <= maxLen {
		return s
	}

	return string(runes[:maxLen-3]) + "..."
}
