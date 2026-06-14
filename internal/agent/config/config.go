package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

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

type DatabaseConfig struct {
	URL        string     `yaml:"url" env:"DATABASE_URL"`
	AccessType AccessType `yaml:"access-type" env:"DATABASE_ACCESS_TYPE" env-default:"BUILDER"`
}

type LoggerConfig struct {
	File  string `yaml:"file" env:"AGENT_LOGS_FILE" env-default:""`
	Level string `yaml:"level" env:"AGENT_LOG_LEVEL" env-default:"ERROR"`
}

type KafkaRetryConfig struct {
	MaxRetries    int           `yaml:"max-retries" env:"KAFKA_MAX_RETRIES" env-default:"5"`
	Delay         time.Duration `yaml:"delay" env:"KAFKA_RETRY_DELAY" env-default:"100ms"`
	MaxDelay      time.Duration `yaml:"max-delay" env:"KAFKA_RETRY_MAX_DELAY" env-default:"30s"`
	BackoffFactor float64       `yaml:"backoff-factor" env:"KAFKA_RETRY_BACKOFF_FACTOR" env-default:"2.0"`
}

type KafkaWorkerConfig struct {
	Count      int           `yaml:"count" env:"KAFKA_WORKER_COUNT" env-default:"1"`
	Interval   time.Duration `yaml:"interval" env:"KAFKA_WORKER_INTERVAL" env-default:"1m"`
	EventLimit int           `yaml:"event-limit" env:"KAFKA_WORKER_EVENT_LIMIT" env-default:"10"`
	MaxRetries int           `yaml:"max-retries" env:"KAFKA_WORKER_MAX_RETRIES" env-default:"5"`
}

type KafkaConfig struct {
	Enable               bool              `yaml:"enabled" env:"KAFKA_ENABLED" env-default:"true"`
	Brokers              []string          `yaml:"brokers" env:"KAFKA_BROKERS"`
	RawUpdateTopic       string            `yaml:"raw-topic" env:"RAW_KAFKA_TOPIC" env-default:"link-updates.raw"`
	ProcessedUpdateTopic string            `yaml:"processed-topic" env:"PROCESSED_KAFKA_TOPIC" env-default:"link-updates.processed"`
	DLQTopic             string            `yaml:"dlq-topic" env:"KAFKA_DLQ_TOPIC" env-default:"link-updates-dlq"`
	SchemaRegistryURL    string            `yaml:"schema-registry-url" env:"SCHEMA_REGISTRY_URL"`
	SchemaPath           string            `yaml:"schema-path" env:"SCHEMA_PATH"`
	ConsumerGroup        string            `yaml:"consumer-group" env:"KAFKA_AGENT_CONSUMER_GROUP" env-default:"agent-group-1"`
	Retries              KafkaRetryConfig  `yaml:"retries"`
	Workers              KafkaWorkerConfig `yaml:"workers"`
}

type AuthorFilerConfig struct {
	Enabled  bool     `yaml:"enabled" env:"AUTHOR_FILTER_ENABLED" env-default:"false"`
	Excluded []string `yaml:"excluded" env:"EXCLUDED_AUTHORS" env-default:"bot,bot-user"`
}

type StopWordsFilerConfig struct {
	Enabled   bool     `yaml:"enabled" env:"STOP_WORDS_FILTER_ENABLED" env-default:"false"`
	StopWords []string `yaml:"stop-words" env:"STOP_WORDS" env-default:"spam,ads,promo"`
}

type LengthFilerConfig struct {
	Enabled   bool `yaml:"enabled" env:"MIN_LENGTH_FILTER_ENABLED" env-default:"false"`
	MinLength int  `yaml:"min-length" env:"MIN_LENGTH" env-default:"20"`
}

type FiltersConfig struct {
	Author    AuthorFilerConfig    `yaml:"author"`
	StopWords StopWordsFilerConfig `yaml:"stop-words"`
	Length    LengthFilerConfig    `yaml:"length"`
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
	Timeout time.Duration        `yaml:"timeout" env-default:"15s"`
	Retry   RetryConfig          `yaml:"retry"`
	Breaker CircuitBreakerConfig `yaml:"breaker"`
}

type GeminiConfig struct {
	APIKey     string           `yaml:"api-key" env:"GEMINI_API_KEY"`
	Resilience HTTPClientConfig `yaml:"resilience"`
}

type TransformerConfig struct {
	SummarizerEnabled bool `yaml:"summarizer-enabled" env:"SUMMARIZER_ENABLED" env-default:"false"`
	Threshold         int  `yaml:"threshold" env:"THRESHOLD" env-default:"500"`
}

type Config struct {
	Logger       LoggerConfig      `yaml:"logger"`
	Kafka        KafkaConfig       `yaml:"kafka"`
	Database     DatabaseConfig    `yaml:"database"`
	Filters      FiltersConfig     `yaml:"filters"`
	Transformers TransformerConfig `yaml:"transformers"`
	Gemini       GeminiConfig      `yaml:"gemini"`
}

func Load(configPath string) (Config, error) {
	var cfg Config
	err := cleanenv.ReadConfig(configPath, &cfg)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}
