package resilience

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimit_Limit(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	const (
		rps   = 3
		burst = 3
	)

	cfg := RateLimitConfig{
		Enabled: true,
		RPS:     rps,
		Burst:   burst,
		TTL:     time.Minute,
	}

	rl := NewIPRateLimiter(cfg, log)

	handler := rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	for i := range burst {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "status code on request number %d (within burst) should be OK", i+1)
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "status code after burst should be Too Many Requests")

	delay := time.Second / rps

	time.Sleep(delay)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "rate limit bucket should have been refilled")

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "rate limit bucket should be empty")
}

func TestRateLimit_IPSensitive(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	const (
		rps   = 3
		burst = 3
	)

	cfg := RateLimitConfig{
		Enabled: true,
		RPS:     rps,
		Burst:   burst,
		TTL:     time.Minute,
	}

	rl := NewIPRateLimiter(cfg, log)

	handler := rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "127.0.0.1"

	for i := range burst {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req1)
		assert.Equal(t, http.StatusOK, w.Code, "status code on request number %d (within burst) should be OK (first IP)", i+1)
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req1)
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "status code after burst should be Too Many Requests (first IP)")

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "127.0.0.2"

	for i := range burst {
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req2)
		assert.Equal(t, http.StatusOK, w.Code, "status code on request number %d (within burst) should be OK (second IP)", i+1)
	}

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req2)
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "status code after burst should be Too Many Requests (second IP)")
}
