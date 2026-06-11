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

type DatabaseConfig struct {
	URL        string     `yaml:"url" env:"DATABASE_URL"`
	AccessType AccessType `yaml:"access-type" env:"DATABASE_ACCESS_TYPE" env-default:"BUILDER"`
}

type TelegramConfig struct {
	Token      string           `yaml:"token" env:"TELEGRAM_TOKEN"`
	ApiURL     string           `yaml:"api-url" env:"TELEGRAM_API_URL" env-default:"https://api.telegram.org/bot"`
	Resilience HTTPClientConfig `yaml:"resilience"`
}

type LoggerConfig struct {
	File  string `yaml:"file" env:"BOT_LOGS_FILE" env-default:""`
	Level string `yaml:"level" env:"BOT_LOG_LEVEL" env-default:"ERROR"`
}

type ScrapperServiceConfig struct {
	URL        string           `yaml:"url" env:"SCRAPPER_URL"`
	Protocol   Protocol         `yaml:"protocol" env:"SCRAPPER_API_PROTOCOL" env-default:"grpc"`
	Resilience HTTPClientConfig `yaml:"resilience"`
}

type ServerConfig struct {
	Port      uint16          `yaml:"port" env:"BOT_API_PORT"`
	Protocol  Protocol        `yaml:"protocol" env:"BOT_API_PROTOCOL" env-default:"grpc"`
	RateLimit RateLimitConfig `yaml:"rate-limit"`
}

type RetriesConfig struct {
	MaxRetries    int           `yaml:"max-retries" env:"KAFKA_MAX_RETRIES" env-default:"5"`
	Delay         time.Duration `yaml:"delay" env:"KAFKA_RETRY_DELAY" env-default:"100ms"`
	MaxDelay      time.Duration `yaml:"max-delay" env:"KAFKA_RETRY_MAX_DELAY" env-default:"30s"`
	BackoffFactor float64       `yaml:"backoff-factor" env:"KAFKA_RETRY_BACKOFF_FACTOR" env-default:"2.0"`
}

type KafkaConfig struct {
	Enable            bool          `yaml:"enabled" env:"KAFKA_ENABLED" env-default:"true"`
	Brokers           []string      `yaml:"brokers" env:"KAFKA_BROKERS"`
	Topic             string        `yaml:"topic" env:"KAFKA_TOPIC" env-default:"link-updates"`
	DLQTopic          string        `yaml:"dlq-topic" env:"KAFKA_DLQ_TOPIC" env-default:"link-updates-dlq"`
	SchemaRegistryURL string        `yaml:"schema-registry-url" env:"SCHEMA_REGISTRY_URL"`
	ConsumerGroup     string        `yaml:"consumer-group" env:"KAFKA_BOT_CONSUMER_GROUP" env-default:"bot-group-1"`
	Retries           RetriesConfig `yaml:"retries"`
}

type Config struct {
	Telegram        TelegramConfig        `yaml:"telegram"`
	Logger          LoggerConfig          `yaml:"logger"`
	ScrapperService ScrapperServiceConfig `yaml:"scrapper-service"`
	Server          ServerConfig          `yaml:"server"`
	Database        DatabaseConfig        `yaml:"database"`
	Kafka           KafkaConfig           `yaml:"kafka"`
}

func Load(configPath string) (Config, error) {
	var cfg Config
	err := cleanenv.ReadConfig(configPath, &cfg)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}
