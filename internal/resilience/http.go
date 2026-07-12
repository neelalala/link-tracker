package resilience

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"time"

	"github.com/avast/retry-go/v5"
	"github.com/sony/gobreaker/v2"
)

var DefaultRetryableStatuses = []int{
	http.StatusRequestTimeout,
	http.StatusTooEarly,
	http.StatusTooManyRequests,
	http.StatusInternalServerError,
	http.StatusBadGateway,
	http.StatusServiceUnavailable,
	http.StatusGatewayTimeout,
}

type RetryConfig struct {
	Enabled           bool
	MaxRetries        uint
	Delay             time.Duration
	Backoff           bool
	BackoffFactor     float64
	MaxDelay          time.Duration
	RetryableStatuses []int
}

type CircuitBreakerConfig struct {
	Enabled              bool
	MaxRequests          uint32
	SlidingWindow        time.Duration
	WaitInOpenState      time.Duration
	MinimumNumberOfCalls uint32
	FailureRateThreshold float64
}

type HTTPClientConfig struct {
	Timeout time.Duration
	Retry   RetryConfig
	Breaker CircuitBreakerConfig
}

func NewHTTPClient(name string, cfg HTTPClientConfig, base *http.Client, log *slog.Logger) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}

	client := *base

	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	if cfg.Breaker.Enabled {
		transport = newBreakerTransport(transport, name, cfg.Breaker, log)
	}

	if cfg.Retry.Enabled {
		transport = newRetryTransport(transport, name, cfg.Retry, log)
	}

	client.Transport = transport
	client.Timeout = 2 * cfg.Timeout

	return &client
}

type retryTransport struct {
	base      http.RoundTripper
	retrier   *retry.Retrier
	retryable []int
	log       *slog.Logger
}

func newRetryTransport(base http.RoundTripper, name string, cfg RetryConfig, log *slog.Logger) http.RoundTripper {
	if len(cfg.RetryableStatuses) == 0 {
		cfg.RetryableStatuses = DefaultRetryableStatuses
	}

	var options = []retry.Option{
		retry.Attempts(cfg.MaxRetries),
		retry.Delay(cfg.Delay),
		retry.OnRetry(func(n uint, err error) {
			log.Warn("retrying request",
				"client", name,
				"attempt", n+1,
				"error", err,
			)
		}),
	}

	if cfg.Backoff {
		options = append(options, retry.DelayType(exponentialBackoffDelay(cfg.BackoffFactor, cfg.MaxDelay)))
	} else {
		options = append(options, retry.DelayType(retry.FixedDelay))
	}

	retrier := retry.New(options...)

	return &retryTransport{
		base:      base,
		retrier:   retrier,
		retryable: cfg.RetryableStatuses,
		log:       log,
	}
}

func (transport *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var lastResp *http.Response
	var lastErr error

	err := transport.retrier.Do(func() error {
		if lastResp != nil {
			io.Copy(io.Discard, lastResp.Body)
			lastResp.Body.Close()
			lastResp = nil
		}
		if err := req.Context().Err(); err != nil {
			return retry.Unrecoverable(err)
		}
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return retry.Unrecoverable(err)
			}
			req.Body = body
		}

		lastResp, lastErr = transport.base.RoundTrip(req)
		if lastErr != nil {
			if errors.Is(lastErr, gobreaker.ErrOpenState) {
				return retry.Unrecoverable(lastErr)
			}
			return lastErr
		}

		if slices.Contains(transport.retryable, lastResp.StatusCode) {
			return fmt.Errorf("retryable status code %d", lastResp.StatusCode)
		}

		return nil
	})
	if lastErr == nil && lastResp != nil {
		return lastResp, nil
	}

	return nil, err
}

func exponentialBackoffDelay(backoffFactor float64, maxDelay time.Duration) retry.DelayTypeFunc {
	return func(n uint, _ error, config retry.DelayContext) time.Duration {
		if n > config.MaxBackOffN() {
			n = config.MaxBackOffN()
		}

		n--

		baseDelay := float64(config.Delay())
		delay := time.Duration(baseDelay * math.Pow(backoffFactor, float64(n)))

		if maxDelay > 0 && delay > maxDelay {
			return maxDelay
		}

		return delay
	}
}

type breakerError struct {
	statusCode int
}

func (err breakerError) Error() string {
	return fmt.Sprintf("server error: %d", err.statusCode)
}

type breakerTransport struct {
	base http.RoundTripper
	cb   *gobreaker.CircuitBreaker[*http.Response]
}

func newBreakerTransport(base http.RoundTripper, name string, cfg CircuitBreakerConfig, log *slog.Logger) http.RoundTripper {
	cb := gobreaker.NewCircuitBreaker[*http.Response](gobreaker.Settings{
		Name:        name,
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.SlidingWindow,
		Timeout:     cfg.WaitInOpenState,
		ReadyToTrip: readyToTrip(cfg.MinimumNumberOfCalls, cfg.FailureRateThreshold),
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Warn("circuit breaker state change",
				"client", name,
				"state", from,
				"to", to,
			)
		},
	})
	return &breakerTransport{
		base: base,
		cb:   cb,
	}
}

func (transport *breakerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := transport.cb.Execute(func() (*http.Response, error) {
		if err := req.Context().Err(); err != nil {
			return nil, err
		}
		resp, err := transport.base.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode >= http.StatusInternalServerError {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return resp, breakerError{resp.StatusCode}
		}

		return resp, nil
	})
	if err != nil {
		var statusErr breakerError
		if errors.As(err, &statusErr) {
			return resp, nil
		}
		return nil, err
	}

	return resp, nil
}

func readyToTrip(minNumOfCalls uint32, failureThreshold float64) func(counts gobreaker.Counts) bool {
	return func(counts gobreaker.Counts) bool {
		failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
		return counts.Requests >= minNumOfCalls && failureRatio >= failureThreshold
	}
}
