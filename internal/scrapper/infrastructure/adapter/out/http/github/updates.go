package github

import (
	"fmt"
	"time"
)

type NewPRUpdate struct {
	title     string
	author    string
	createdAt time.Time
	body      string

	maxPreviewLen int
}

func (prUpdate *NewPRUpdate) UpdatedAt() time.Time {
	return prUpdate.createdAt
}

func (prUpdate *NewPRUpdate) Author() string {
	return prUpdate.author
}

func (prUpdate *NewPRUpdate) Description() string {
	return fmt.Sprintf("New Pull Request: %s by %s", prUpdate.title, prUpdate.Author)
}

func (prUpdate *NewPRUpdate) Preview() string {
	return truncateText(prUpdate.body, prUpdate.maxPreviewLen)
}

type NewIssueUpdate struct {
	title     string
	author    string
	createdAt time.Time
	body      string

	maxPreviewLen int
}

func (issueUpdate *NewIssueUpdate) UpdatedAt() time.Time {
	return issueUpdate.createdAt
}

func (issueUpdate *NewIssueUpdate) Author() string {
	return issueUpdate.author
}

func (issueUpdate *NewIssueUpdate) Description() string {
	return fmt.Sprintf("New Issue: %s by %s", issueUpdate.title, issueUpdate.author)
}

func (issueUpdate *NewIssueUpdate) Preview() string {
	return truncateText(issueUpdate.body, issueUpdate.maxPreviewLen)
}

func truncateText(s string, maxLen int) string {
	runes := []rune(s)

	if len(runes) <= maxLen {
		return s
	}

	return string(runes[:maxLen-3]) + "..."
}
