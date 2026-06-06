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

type DatabaseConfig struct {
	URL        string     `yaml:"url" env:"DATABASE_URL"`
	AccessType AccessType `yaml:"access-type" env:"DATABASE_ACCESS_TYPE" env-default:"BUILDER"`
}

type TelegramConfig struct {
	Token   string        `yaml:"token" env:"TELEGRAM_TOKEN"`
	ApiURL  string        `yaml:"api-url" env:"TELEGRAM_API_URL" env-default:"https://api.telegram.org/bot"`
	Timeout time.Duration `yaml:"timeout" env:"TELEGRAM_TIMEOUT" env-default:"10s"`
}

type LoggerConfig struct {
	File  string `yaml:"file" env:"BOT_LOGS_FILE" env-default:""`
	Level string `yaml:"level" env:"BOT_LOG_LEVEL" env-default:"ERROR"`
}

type ScrapperServiceConfig struct {
	URL      string   `yaml:"url" env:"SCRAPPER_URL"`
	Protocol Protocol `yaml:"protocol" env:"SCRAPPER_API_PROTOCOL" env-default:"grpc"`
}

type ServerConfig struct {
	Port     uint16   `yaml:"port" env:"BOT_API_PORT"`
	Protocol Protocol `yaml:"protocol" env:"BOT_API_PROTOCOL" env-default:"grpc"`
}

type KafkaConfig struct {
	Enable            bool     `yaml:"enabled" env:"KAFKA_ENABLED" env-default:"true"`
	Brokers           []string `yaml:"brokers" env:"KAFKA_BROKERS"`
	Topic             string   `yaml:"topic" env:"KAFKA_TOPIC" env-default:"link-updates"`
	DLQTopic          string   `yaml:"dlq-topic" env:"KAFKA_DLQ_TOPIC" env-default:"link-updates-dlq"`
	SchemaRegistryURL string   `yaml:"schema-registry-url" env:"SCHEMA_REGISTRY_URL"`
	ConsumerGroup     string   `yaml:"consumer-group" env:"KAFKA_BOT_CONSUMER_GROUP" env-default:"bot-group-1"`
	Retries           int      `yaml:"retries" env:"KAFKA_RETRIES" env-default:"5"`
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
