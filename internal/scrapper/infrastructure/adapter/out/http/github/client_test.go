package github

import (
	"context"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Fetch_Resilience(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		expectErr  bool
	}{
		{
			name:       "500 Internal Server Error",
			statusCode: http.StatusInternalServerError,
			body:       `{"message": "Internal Server Error"}`,
			expectErr:  true,
		},
		{
			name:       "404 Not Found",
			statusCode: http.StatusNotFound,
			body:       `{"message": "Not Found"}`,
			expectErr:  true,
		},
		{
			name:       "200 OK but corrupted/invalid JSON",
			statusCode: http.StatusOK,
			body:       `{ invalid json { `,
			expectErr:  true,
		},
		{
			name:       "403 Forbidden (API Rate Limit Exceeded)",
			statusCode: http.StatusForbidden,
			body:       `{"message": "API rate limit exceeded"}`,
			expectErr:  true,
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))

				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer ts.Close()

			client := NewClient(BaseURL, ts.URL, Timeout)

			_, err := client.Fetch(ctx, "https://github.com/octocat/Hello-World")

			assert.Equalf(t, tt.expectErr, err != nil, "Fetch() expected error = %v, got err = %v", tt.expectErr, err)
		})
	}
}
