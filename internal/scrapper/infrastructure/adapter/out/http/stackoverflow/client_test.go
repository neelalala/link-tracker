package stackoverflow

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
	client := NewClient(nil, BaseURL, BaseApiURL, 10*time.Second, 200, "")

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "Valid StackOverflow URL",
			url:      "https://stackoverflow.com/questions/1234567/how-to-exit-vim",
			expected: true,
		},
		{
			name:     "Invalid StackOverflow URL",
			url:      "https://github.com/owner/repo",
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		switch r.URL.Path {
		case "/questions/1":

			fmt.Fprintln(w, `{
				"items": [{"title": "Test title"}]
			}`)

		case "/questions/1/answers":
			fmt.Fprintln(w, `{
				"items": [
					{
						"owner": {"display_name": "Test User"},
						"creation_date": 1775866000, 
						"body": "Test answer"
					}
				]
			}`)

		case "/questions/1/comments":

			assert.Equal(t, "withbody", r.URL.Query().Get("filter"))
			fmt.Fprintln(w, `{
				"items": [
					{
						"owner": {"display_name": "Test User 2"},
						"creation_date": 1775869000,
						"body": "Test comment"
					}
				]
			}`)

		default:

			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(nil, BaseURL, server.URL, 10*time.Second, 200, "")

	url := "https://stackoverflow.com/questions/1/test-question"
	updates, err := client.Fetch(context.Background(), url, since)

	require.NoError(t, err)

	require.Len(t, updates, 2)

	answerUpdate, ok := updates[0].(*AnswerUpdate)
	require.True(t, ok, "first update should be an answer")
	assert.Equal(t, "Test title", answerUpdate.title)
	assert.Equal(t, "Test User", answerUpdate.owner)
	assert.Equal(t, "Test answer", answerUpdate.body)

	commentUpdate, ok := updates[1].(*CommentUpdate)
	require.True(t, ok, "second update should be a comment")
	assert.Equal(t, "Test title", commentUpdate.title)
	assert.Equal(t, "Test User 2", commentUpdate.owner)
	assert.Equal(t, "Test comment", commentUpdate.body)
}

func TestClient_Fetch_InvalidURL(t *testing.T) {
	client := NewClient(nil, BaseURL, BaseApiURL, 10*time.Second, 200, "")

	_, err := client.Fetch(context.Background(), "https://stackoverflow.com/questions/", time.Now())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid stackoverflow url")
}

func TestClient_Fetch_QuestionNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"items": []}`)
	}))
	defer server.Close()

	client := NewClient(nil, BaseURL, server.URL, 10*time.Second, 200, "")

	_, err := client.Fetch(context.Background(), "https://stackoverflow.com/questions/99999", time.Now())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "question not found")
}

func TestClient_Fetch_ApiError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(nil, BaseURL, server.URL, 10*time.Second, 200, "")

	_, err := client.Fetch(context.Background(), "https://stackoverflow.com/questions/1", time.Now())

	require.Error(t, err)

	assert.Contains(t, err.Error(), "error getting title")
}

func TestClient_Preview_MaxLength(t *testing.T) {
	since := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		switch r.URL.Path {
		case "/questions/1":

			fmt.Fprintln(w, `{
				"items": [{"title": "Test title"}]
			}`)

		case "/questions/1/answers":
			fmt.Fprintf(w, `{
				"items": [
					{
						"owner": {"display_name": "Test User 1"},
						"creation_date": 1775866000, 
						"body": "long answer body: %s"
					},
					{
						"owner": {"display_name": "Test User 2"},
						"creation_date": 1775966000, 
						"body": "normal answer body: bad tests"
					}
				]
			}`, strings.Repeat("1234", 100))

		case "/questions/1/comments":

			assert.Equal(t, "withbody", r.URL.Query().Get("filter"))
			fmt.Fprintf(w, `{
				"items": [
					{
						"owner": {"display_name": "Test User 3"},
						"creation_date": 1775869000,
						"body": "long comment body: %s"
					},
					{
						"owner": {"display_name": "Test User 4"},
						"creation_date": 1775966000,
						"body": "normal comment body: this tests are fixed now"
					}
				]
			}`, strings.Repeat("1234", 100))

		default:

			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(nil, BaseURL, server.URL, 10*time.Second, 200, "")

	url := "https://stackoverflow.com/questions/1/test-question"
	updates, err := client.Fetch(context.Background(), url, since)

	require.NoError(t, err)

	require.Len(t, updates, 4)

	for _, update := range updates {
		assert.GreaterOrEqual(t, 200, len([]rune(update.Description())))
	}
}
