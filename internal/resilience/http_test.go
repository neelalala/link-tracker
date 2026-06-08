package resilience

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/sony/gobreaker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetry_SuccessOnThirdAttempt(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	var attempts int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	cfg := HTTPClientConfig{
		Timeout: 5 * time.Second,
		Retry: RetryConfig{
			Enabled:           true,
			MaxRetries:        3,
			Delay:             10 * time.Millisecond,
			Backoff:           false,
			RetryableStatuses: []int{http.StatusInternalServerError},
		},
	}

	client := NewHTTPClient("test-retries", cfg, nil, log)

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := client.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "expected status code 200")

	assert.Equal(t, 3, attempts, "expected 2 failures and 1 success")
}

func TestCircuitBreaker_OpensAfterFailures(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	var attempts int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := HTTPClientConfig{
		Timeout: 5 * time.Second,
		Breaker: CircuitBreakerConfig{
			Enabled:              true,
			MinimumNumberOfCalls: 2,
			FailureRateThreshold: 0.5,
			SlidingWindow:        10 * time.Second,
			WaitInOpenState:      1 * time.Minute,
		},
	}

	client := NewHTTPClient("test-circuit_breaker", cfg, nil, log)

	req1, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	resp1, err := client.Do(req1)
	require.Error(t, err, "expected server to return Internal Error (first request)")

	if resp1 != nil && resp1.Body != nil {
		resp1.Body.Close()
	}

	req2, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	resp2, err := client.Do(req2)
	require.Error(t, err, "expected server to return Internal Error (second request)")

	if resp2 != nil && resp2.Body != nil {
		resp2.Body.Close()
	}

	req3, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	resp3, err := client.Do(req3)

	assert.True(t, errors.Is(err, gobreaker.ErrOpenState), "expected error to be open state")

	if resp3 != nil && resp3.Body != nil {
		resp3.Body.Close()
	}

	assert.Equal(t, 2, attempts, "expected 2 server calls")
}

func TestResilience_RetryAfterCircuitBreakerOpens(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	var attempts int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := HTTPClientConfig{
		Timeout: 5 * time.Second,
		Retry: RetryConfig{
			Enabled:           true,
			MaxRetries:        3,
			Delay:             10 * time.Millisecond,
			Backoff:           false,
			RetryableStatuses: []int{http.StatusInternalServerError},
		},
		Breaker: CircuitBreakerConfig{
			Enabled:              true,
			MinimumNumberOfCalls: 2,
			FailureRateThreshold: 0.5,
			SlidingWindow:        5 * time.Second,
			WaitInOpenState:      5 * time.Second,
		},
	}

	client := NewHTTPClient("test-retries_after_cb_opens", cfg, nil, log)

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)

	assert.True(t, errors.Is(err, gobreaker.ErrOpenState), "expected error to be open state")
	assert.Nil(t, resp, "expected resp to be nil")
	assert.Equal(t, 2, attempts, "expected 2 server calls")
}
