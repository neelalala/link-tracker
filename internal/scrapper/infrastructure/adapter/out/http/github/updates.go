package github

import (
	"fmt"
	"time"
)

type NewPRUpdate struct {
	Title     string
	Author    string
	CreatedAt time.Time
	Body      string

	MaxPreviewLen int
}

func (prUpdate *NewPRUpdate) UpdatedAt() time.Time {
	return prUpdate.CreatedAt
}

func (prUpdate *NewPRUpdate) Description() string {
	return fmt.Sprintf("New Pull Request: %s by %s", prUpdate.Title, prUpdate.Author)
}

func (prUpdate *NewPRUpdate) Preview() string {
	return truncateText(prUpdate.Body, prUpdate.MaxPreviewLen)
}

type NewIssueUpdate struct {
	Title     string
	Author    string
	CreatedAt time.Time
	Body      string

	MaxPreviewLen int
}

func (issueUpdate *NewIssueUpdate) UpdatedAt() time.Time {
	return issueUpdate.CreatedAt
}

func (issueUpdate *NewIssueUpdate) Description() string {
	return fmt.Sprintf("New Issue: %s by %s", issueUpdate.Title, issueUpdate.Author)
}

func (issueUpdate *NewIssueUpdate) Preview() string {
	return truncateText(issueUpdate.Body, issueUpdate.MaxPreviewLen)
}

func truncateText(s string, maxLen int) string {
	runes := []rune(s)

	if len(runes) <= maxLen {
		return s
	}

	return string(runes[:maxLen-3]) + "..."
}
