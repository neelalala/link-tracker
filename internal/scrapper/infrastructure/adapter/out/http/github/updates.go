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
}

func (prUpdate *GithubNewPRUpdate) UpdatedAt() time.Time {
	return prUpdate.CreatedAt
}

func (prUpdate *GithubNewPRUpdate) Description() string {
	return fmt.Sprintf("New Pull Request: %s by %s", prUpdate.Title, prUpdate.Author)
}

func (prUpdate *GithubNewPRUpdate) Preview() string {
	return prUpdate.Body
}

type GithubNewIssueUpdate struct {
	Title     string
	Author    string
	CreatedAt time.Time
	Body      string
}

func (issueUpdate *GithubNewIssueUpdate) UpdatedAt() time.Time {
	return issueUpdate.CreatedAt
}

func (issueUpdate *GithubNewIssueUpdate) Description() string {
	return fmt.Sprintf("New Issue: %s by %s", issueUpdate.Title, issueUpdate.Author)
}

func (issueUpdate *GithubNewIssueUpdate) Preview() string {
	return issueUpdate.Body
}
