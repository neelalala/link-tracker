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
	File  string `yaml:"file" env:"BOT_LOGS_FILE" env-default:""`
	Level string `yaml:"level" env:"BOT_LOG_LEVEL" env-default:"ERROR"`
}

type KafkaRetryConfig struct {
	MaxRetries    int           `yaml:"max-retries" env:"KAFKA_MAX_RETRIES" env-default:"5"`
	Delay         time.Duration `yaml:"delay" env:"KAFKA_RETRY_DELAY" env-default:"100ms"`
	MaxDelay      time.Duration `yaml:"max-delay" env:"KAFKA_RETRY_MAX_DELAY" env-default:"30s"`
	BackoffFactor float64       `yaml:"backoff-factor" env:"KAFKA_RETRY_BACKOFF_FACTOR" env-default:"2.0"`
}

type KafkaConfig struct {
	Enable            bool             `yaml:"enabled" env:"KAFKA_ENABLED" env-default:"true"`
	Brokers           []string         `yaml:"brokers" env:"KAFKA_BROKERS"`
	Topic             string           `yaml:"topic" env:"KAFKA_TOPIC" env-default:"link-updates"`
	DLQTopic          string           `yaml:"dlq-topic" env:"KAFKA_DLQ_TOPIC" env-default:"link-updates-dlq"`
	SchemaRegistryURL string           `yaml:"schema-registry-url" env:"SCHEMA_REGISTRY_URL"`
	ConsumerGroup     string           `yaml:"consumer-group" env:"KAFKA_BOT_CONSUMER_GROUP" env-default:"bot-group-1"`
	Retries           KafkaRetryConfig `yaml:"retries"`
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

type Config struct {
	Logger   LoggerConfig   `yaml:"logger"`
	Kafka    KafkaConfig    `yaml:"kafka"`
	Database DatabaseConfig `yaml:"database"`
	Filters  FiltersConfig  `yaml:"filters"`
}

func Load(configPath string) (Config, error) {
	var cfg Config
	err := cleanenv.ReadConfig(configPath, &cfg)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}
