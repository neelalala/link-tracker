package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_CanHandle(t *testing.T) {
	client := NewClient(BaseURL, BaseApiURL, 10*time.Second, 200)

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "Valid GitHub URL",
			url:      "https://github.com/owner/repo",
			expected: true,
		},
		{
			name:     "Invalid GitHub URL",
			url:      "https://stackoverflow.com/questions/123",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.CanHandle(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClient_Fetch_Success(t *testing.T) {
	since := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)

	mux := http.NewServeMux()

	mux.HandleFunc("/repos/owner/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		fmt.Fprintln(w, `[
			{
				"title": "New PR",
				"user": {"login": "alice"},
				"body_text": "new pr body",
				"created_at": "2026-04-11T12:00:00Z"
			},
			{
				"title": "Old PR",
				"user": {"login": "bob"},
				"body_text": "old pr body",
				"created_at": "2026-04-09T12:00:00Z"
			}
		]`)
	})

	mux.HandleFunc("/repos/owner/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		fmt.Fprintln(w, `[
			{
				"title": "New Issue",
				"user": {"login": "charlie"},
				"body_text": "new issue body",
				"created_at": "2026-04-11T12:00:00Z"
			},
			{
				"title": "Old Issue",
				"user": {"login": "dave"},
				"body_text": "old issue body",
				"created_at": "2026-04-09T12:00:00Z"
			},
			{
				"title": "Pull request",
				"user": {"login": "eve"},
				"body_text": "pr body",
				"created_at": "2026-04-11T12:00:00Z",
				"pull_request": {} 
			}
		]`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient(BaseURL, server.URL, 10*time.Second, 200)

	url := "https://github.com/owner/repo"
	updates, err := client.Fetch(context.Background(), url, since)

	require.NoError(t, err)
	require.Len(t, updates, 2)

	prUpdate, ok := updates[0].(*GithubNewPRUpdate)
	require.True(t, ok, "first update should be a PR")
	assert.Equal(t, "New PR", prUpdate.Title)
	assert.Equal(t, "alice", prUpdate.Author)

	issueUpdate, ok := updates[1].(*GithubNewIssueUpdate)
	require.True(t, ok, "second update should be an Issue")
	assert.Equal(t, "New Issue", issueUpdate.Title)
	assert.Equal(t, "charlie", issueUpdate.Author)
}

func TestClient_Fetch_InvalidURL(t *testing.T) {
	client := NewClient(BaseURL, BaseApiURL, 10*time.Second, 200)

	_, err := client.Fetch(context.Background(), "https://github.com/owner", time.Now())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid github url")
}

func TestClient_Fetch_ApiError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(BaseURL, server.URL, 10*time.Second, 200)

	_, err := client.Fetch(context.Background(), "https://github.com/owner/repo", time.Now())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error fetching pull requests")
}

func TestClient_Preview_MaxLength(t *testing.T) {
	since := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)

	mux := http.NewServeMux()

	mux.HandleFunc("/repos/owner/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, `[
			{
				"title": "New PR",
				"user": {"login": "alice"},
			"body_text": "long pr body: %s",
				"created_at": "2026-04-11T12:00:00Z"
			}
		]`, strings.Repeat("1234", 100))
	})

	mux.HandleFunc("/repos/owner/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, `[
			{
				"title": "New Issue",
				"user": {"login": "charlie"},
			"body_text": "long issue body: %s",
				"created_at": "2026-04-11T12:00:00Z"
			}
		]`, strings.Repeat("1234", 100))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient(BaseURL, server.URL, 10*time.Second, 200)

	url := "https://github.com/owner/repo"
	updates, err := client.Fetch(context.Background(), url, since)

	require.NoError(t, err)
	require.Len(t, updates, 2)

	for _, update := range updates {
		assert.LessOrEqual(t, 200, len(update.Preview()))
	}
}
