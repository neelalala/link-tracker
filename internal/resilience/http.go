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
	var retrier *retry.Retrier

	if len(cfg.RetryableStatuses) == 0 {
		cfg.RetryableStatuses = DefaultRetryableStatuses
	}

	if cfg.Backoff {
		retrier = retry.New(
			retry.Attempts(cfg.MaxRetries),
			retry.Delay(cfg.Delay),
			retry.DelayType(exponentialBackoffDelay(cfg.BackoffFactor, cfg.MaxDelay)),
			retry.OnRetry(func(n uint, err error) {
				log.Warn("retrying request",
					"client", name,
					"attempt", n+1,
					"error", err,
				)
			}),
		)
	} else {
		retrier = retry.New(
			retry.Attempts(cfg.MaxRetries),
			retry.Delay(cfg.Delay),
			retry.DelayType(retry.FixedDelay),
			retry.OnRetry(func(n uint, err error) {
				log.Warn("retrying request",
					"client", name,
					"attempt", n+1,
					"error", err,
				)
			}),
		)
	}

	return &retryTransport{
		base:      base,
		retrier:   retrier,
		retryable: cfg.RetryableStatuses,
		log:       log,
	}
}

func (transport *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var final *http.Response

	err := transport.retrier.Do(func() error {
		resp, err := transport.base.RoundTrip(req)
		if err != nil {
			if errors.Is(err, gobreaker.ErrOpenState) {
				return retry.Unrecoverable(err)
			}
			return err
		}

		if slices.Contains(transport.retryable, resp.StatusCode) {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return fmt.Errorf("retryable status code %d", resp.StatusCode)
		}

		final = resp
		return nil
	})
	if err != nil {
		return nil, err
	}

	return final, nil
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
	return transport.cb.Execute(func() (*http.Response, error) {
		resp, err := transport.base.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode >= http.StatusInternalServerError {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("server error: %d", resp.StatusCode)
		}

		return resp, nil
	})
}

func readyToTrip(minNumOfCalls uint32, failureThreshold float64) func(counts gobreaker.Counts) bool {
	return func(counts gobreaker.Counts) bool {
		failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
		return counts.Requests >= minNumOfCalls && failureRatio >= failureThreshold
	}
}
