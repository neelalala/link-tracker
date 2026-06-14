package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Protocol string

const (
	ProtocolHTTP Protocol = "http"
	ProtocolGRPC Protocol = "grpc"
)

func (protocol *Protocol) SetValue(s string) error {
	switch s {
	case "http":
		*protocol = ProtocolHTTP
		return nil
	case "grpc":
		*protocol = ProtocolGRPC
		return nil
	default:
		return fmt.Errorf("invalid protocol: %q. Allowed values are 'http' or 'grpc'", s)
	}
}

type AccessType string

const (
	AccessTypeBUILDER AccessType = "BUILDER"
	AccessTypeSQL     AccessType = "SQL"
)

func (accessType *AccessType) SetValue(s string) error {
	switch s {
	case "BUILDER":
		*accessType = AccessTypeBUILDER
		return nil
	case "SQL":
		*accessType = AccessTypeSQL
		return nil
	default:
		return fmt.Errorf("invalid access type: %q. Allowed values are 'SQL' or 'BUILDER'", s)
	}
}

type LoggerConfig struct {
	File  string `yaml:"file" env:"SCRAPPER_LOGS_FILE" env-default:""`
	Level string `yaml:"level" env:"SCRAPPER_LOG_LEVEL" env-default:"ERROR"`
}

type DatabaseConfig struct {
	MigrationsDirURL string     `yaml:"migrations-dir-url" env:"SCRAPPER_MIGRATIONS_DIR_URL"`
	URL              string     `yaml:"url" env:"DATABASE_URL"`
	AccessType       AccessType `yaml:"access-type" env:"DATABASE_ACCESS_TYPE" env-default:"BUILDER"`
}

type SchedulerConfig struct {
	JobInterval time.Duration `yaml:"job-interval" env:"SCHEDULER_JOB_INTERVAL" env-default:"5m"`
	JobTimeout  time.Duration `yaml:"job-timeout" env:"SCHEDULER_JOB_TIMEOUT" env-default:"5m"`
}

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
	Enabled bool          `yaml:"enabled" env:"SCRAPPER_API_RATELIMIT_ENABLED" env-default:"true"`
	RPS     int           `yaml:"rps" env:"SCRAPPER_API_RATELIMIT_RPS" env-default:"10"`
	Burst   int           `yaml:"burst" env:"SCRAPPER_API_RATELIMIT_BURST" env-default:"5"`
	TTL     time.Duration `yaml:"ttl" env:"SCRAPPER_API_RATELIMIT_TTL" env-default:"1m"`
}

type BotServiceConfig struct {
	URL        string           `yaml:"url" env:"BOT_URL"`
	Protocol   Protocol         `yaml:"protocol" env:"BOT_API_PROTOCOL" env-default:"grpc"`
	Resilience HTTPClientConfig `yaml:"resilience"`
}

type ServerConfig struct {
	Port      uint16          `yaml:"port" env:"SCRAPPER_API_PORT"`
	Protocol  Protocol        `yaml:"protocol" env:"SCRAPPER_API_PROTOCOL" env-default:"grpc"`
	RateLimit RateLimitConfig `yaml:"rate-limit"`
}

type FetchersConfig struct {
	DescriptionLimit int              `yaml:"description-limit" env:"FETCHER_DESCRIPTION_LIMIT" env-default:"5000"`
	Concurrency      int              `yaml:"concurrency" env:"FETCHER_COUNT" env-default:"1"`
	Batch            int              `yaml:"batch" env:"FETCHER_BATCH_SIZE" env-default:"100"`
	StackOverflowKey string           `yaml:"stackoverflow-key" env:"STACKOVERFLOW_KEY" env-default:""`
	Resilience       HTTPClientConfig `yaml:"resilience"`
}

type KafkaWorkerConfig struct {
	Count      int           `yaml:"count" env:"KAFKA_WORKER_COUNT" env-default:"1"`
	Interval   time.Duration `yaml:"interval" env:"KAFKA_WORKER_INTERVAL" env-default:"1m"`
	EventLimit int           `yaml:"event-limit" env:"KAFKA_WORKER_EVENT_LIMIT" env-default:"10"`
	MaxRetries int           `yaml:"max-retries" env:"KAFKA_WORKER_MAX_RETRIES" env-default:"5"`
}

type KafkaConfig struct {
	Enable            bool              `yaml:"enabled" env:"KAFKA_ENABLED" env-default:"true"`
	Brokers           []string          `yaml:"brokers" env:"KAFKA_BROKERS"`
	SchemaRegistryURL string            `yaml:"schema-registry-url" env:"SCHEMA_REGISTRY_URL"`
	Topic             string            `yaml:"topic" env:"KAFKA_SCRAPPER_TOPIC" env-default:"link-updates.raw"`
	SchemaPath        string            `yaml:"schema-path" env:"SCHEMA_PATH"`
	Workers           KafkaWorkerConfig `yaml:"workers"`
}

type ValkeyConfig struct {
	Enabled         bool          `yaml:"enabled" env:"CACHE_ENABLED" env-default:"true"`
	Addresses       []string      `yaml:"addresses" env:"CACHE_ADDRESSES"`
	TTL             time.Duration `yaml:"ttl" env:"CACHE_TTL" env-default:"1h"`
	KeyPrefix       string        `yaml:"key-prefix" env:"CACHE_KEY_PREFIX" env-default:"scrapper:links:"`
	ClientSideCache bool          `yaml:"client-side-cache" env:"CACHE_CLIENT_SIDE" env-default:"true"`
}

type Config struct {
	Logger     LoggerConfig     `yaml:"logger"`
	Scheduler  SchedulerConfig  `yaml:"scheduler"`
	BotService BotServiceConfig `yaml:"bot-service"`
	Server     ServerConfig     `yaml:"server"`
	Database   DatabaseConfig   `yaml:"database"`
	Fetchers   FetchersConfig   `yaml:"fetchers"`
	Kafka      KafkaConfig      `yaml:"kafka"`
	Valkey     ValkeyConfig     `yaml:"valkey"`
}

func Load(configPath string) (Config, error) {
	var cfg Config
	err := cleanenv.ReadConfig(configPath, &cfg)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}
