package resilience

import (
	"net/http"
	"time"
)

type RetryConfig struct {
	Enabled           bool          `yaml:"enabled" env-default:"true"`
	MaxRetries        uint          `yaml:"max-retries" env-default:"3"`
	Delay             time.Duration `yaml:"delay" env-default:"200ms"`
	Backoff           bool          `yaml:"backoff" env-default:"false"`
	BackoffFactor     float64       `yaml:"backoff-factor" env-default:"2.0"`
	MaxDelay          time.Duration `yaml:"max-delay" env-default:"30s"`
	RetryableStatuses []int         `yaml:"retryable-statuses"`
}

type CircuitBreakerConfig struct {
	Enabled              bool          `yaml:"enabled" env-default:"true"`
	MaxRequests          uint32        `yaml:"max-requests" env-default:"10"`
	SlidingWindow        time.Duration `yaml:"sliding-window" env-default:"30s"`
	WaitInOpenState      time.Duration `yaml:"wait-in-open-state" env-default:"15s"`
	MinimumNumberOfCalls uint32        `yaml:"minimum-number-of-calls" env-default:"10"`
	FailureRateThreshold float64       `yaml:"failure-rate-threshold" env-default:"0.5"`
}

type HTTPClientConfig struct {
	Timeout time.Duration        `yaml:"timeout" env-default:"5s"`
	Retry   RetryConfig          `yaml:"retry"`
	Breaker CircuitBreakerConfig `yaml:"breaker"`
}

type RateLimitConfig struct {
	Enabled bool          `yaml:"enabled" env-default:"true"`
	RPS     int           `yaml:"rps" env-default:"10"`
	Burst   int           `yaml:"burst" env-default:"5"`
	TTL     time.Duration `yaml:"ttl" env-default:"1m"`
}

var DefaultRetryableStatuses = []int{
	http.StatusRequestTimeout,
	http.StatusTooEarly,
	http.StatusTooManyRequests,
	http.StatusInternalServerError,
	http.StatusBadGateway,
	http.StatusServiceUnavailable,
	http.StatusGatewayTimeout,
}
