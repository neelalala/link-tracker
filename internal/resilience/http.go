package resilience

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"time"

	"github.com/avast/retry-go/v5"
)

func NewHTTPClient(name string, cfg HTTPClientConfig, base *http.Client, log *slog.Logger) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}

	client := base

	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	if cfg.Retry.Enabled {
		transport = newRetryTransport(transport, name, cfg.Retry, log)
	}

	client.Transport = transport
	client.Timeout = 2 * cfg.Timeout

	return client
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
		//if req.Body != nil && req.GetBody != nil {
		//	body, err := req.GetBody()
		//	if err != nil {
		//		return retry.Unrecoverable(err)
		//	}
		//	req.Body = body
		//}

		resp, err := transport.base.RoundTrip(req)
		if err != nil {
			return err
		}

		if slices.Contains(transport.retryable, resp.StatusCode) {
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
