package resilience

import (
	"context"
	"errors"
	"io"
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

	const (
		expectedBody = "success"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedBody))
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

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "expected status code 200")

	assert.Equal(t, 3, attempts, "expected 2 failures and 1 success")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, expectedBody, string(body), "corrupted body")
}

func TestRetry_CancelledContext(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	var attempts int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusOK)
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

	client := NewHTTPClient("test-retries-context_cancelled", cfg, nil, log)

	ctx, cancel := context.WithCancel(context.Background())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	cancel()

	resp, err := client.Do(req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "expected error to be cancelled")
	assert.Nil(t, resp, "expected resp to be nil")

	assert.Equal(t, 0, attempts, "expected 2 server calls")
}

func TestRetry_StatusBadRequest(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
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
	client := NewHTTPClient("test-retries-bad_request", cfg, nil, log)

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "expected status code 400")
	assert.Equal(t, 1, attempts, "expected no retries")
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
	assert.Nil(t, resp1)

	req2, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	resp2, err := client.Do(req2)
	require.Error(t, err, "expected server to return Internal Error (second request)")
	assert.Nil(t, resp2)

	req3, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	resp3, err := client.Do(req3)
	require.Error(t, err, "expected error on third attempt")
	assert.True(t, errors.Is(err, gobreaker.ErrOpenState), "expected error to be open state")
	assert.Nil(t, resp3)

	assert.Equal(t, 2, attempts, "expected 2 server calls")
}

func TestCircuitBreaker_CancelledContext(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	var attempts int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusOK)
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

	client := NewHTTPClient("test-circuit_breaker-context_cancelled", cfg, nil, log)

	ctx1, cancel1 := context.WithCancel(context.Background())

	req1, err := http.NewRequestWithContext(ctx1, http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	cancel1()

	resp1, err := client.Do(req1)
	require.Error(t, err, "expected error on first attempt")
	assert.True(t, errors.Is(err, context.Canceled), "expected error to be cancelled")
	assert.Nil(t, resp1)

	ctx2, cancel2 := context.WithCancel(context.Background())

	req2, err := http.NewRequestWithContext(ctx2, http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	cancel2()

	resp2, err := client.Do(req2)
	require.Error(t, err, "expected error on second attempt")
	assert.True(t, errors.Is(err, context.Canceled), "expected error to be cancelled")
	assert.Nil(t, resp2)

	ctx3, cancel3 := context.WithCancel(context.Background())

	req3, err := http.NewRequestWithContext(ctx3, http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	cancel3()

	resp3, err := client.Do(req3)
	require.Error(t, err, "expected error on third attempt")
	assert.True(t, errors.Is(err, gobreaker.ErrOpenState), "expected error to be open state")
	assert.Nil(t, resp3)

	assert.Equal(t, 0, attempts, "expected 2 server calls")
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

type customReader struct {
	data   string
	offset int
}

func (r *customReader) Read(p []byte) (n int, err error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}

	n = copy(p, r.data)

	r.offset += n
	return
}

func (r *customReader) Close() error {
	return nil
}

func TestResilience_ClientBody(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	const (
		expectedRespBody = "respBody"
		expectedReqBody  = "reqBody"
	)
	var attempts int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts >= 2 {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if string(body) != expectedReqBody {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(expectedRespBody))
			return
		}
		io.ReadAll(r.Body)
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

	client := NewHTTPClient("test-resilience-client_body", cfg, nil, log)

	reader := &customReader{data: expectedReqBody}
	req, err := http.NewRequest(http.MethodPost, server.URL, reader)
	require.NoError(t, err)
	req.GetBody = func() (io.ReadCloser, error) {
		return &customReader{data: expectedReqBody}, nil
	}
	resp, err := client.Do(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, expectedRespBody, string(body), "corrupted body")

	assert.Equal(t, 2, attempts, "expected 2 server calls")
}

func TestTimeout(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	const serverOperationDuration = 5 * time.Second
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(serverOperationDuration):
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cfg := HTTPClientConfig{
		Timeout: 1 * time.Second,
	}
	client := NewHTTPClient("test-resilience-timeout", cfg, nil, log)

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
	assert.Nil(t, resp)
	assert.Less(t, elapsed, serverOperationDuration)
}
