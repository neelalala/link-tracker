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

type HTTPClientConfig struct {
	Timeout time.Duration `yaml:"timeout" env-default:"5s"`
	Retry   RetryConfig   `yaml:"retry"`
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
