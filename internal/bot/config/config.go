package config

import (
	"fmt"
	"time"

	"github.com/byrnedo/typesafe-config/parse"
)

type Protocol string

const (
	ProtocolHTTP Protocol = "http"
	ProtocolGRPC Protocol = "grpc"
)

func (protocol Protocol) Validate() error {
	switch protocol {
	case ProtocolHTTP, ProtocolGRPC:
		return nil
	default:
		return fmt.Errorf("invalid protocol: %q. Allowed values are 'http' or 'grpc'", protocol)
	}
}

type AccessType string

const (
	AccessTypeBUILDER AccessType = "BUILDER"
	AccessTypeSQL     AccessType = "SQL"
)

func (accessType AccessType) Validate() error {
	switch accessType {
	case AccessTypeBUILDER, AccessTypeSQL:
		return nil
	default:
		return fmt.Errorf("invalid access type: %q. Allowed values are 'SQL' or 'BUILDER'", accessType)
	}
}

type DatabaseConfig struct {
	URL        string     `config:"url"`
	AccessType AccessType `config:"access-type,BUILDER"`
}

type TelegramConfig struct {
	Token   string        `config:"token"`
	ApiUrl  string        `config:"api-url"`
	Timeout time.Duration `config:"timeout"`
}

type LoggerConfig struct {
	File  string `config:"file,"`
	Level string `config:"level,ERROR"`
}

type ScrapperServiceConfig struct {
	URL      string   `config:"url"`
	Protocol Protocol `config:"protocol"`
}

type ServerConfig struct {
	Port     uint16   `config:"port"`
	Protocol Protocol `config:"protocol"`
}

type RetriesConfig struct {
	MaxRetries    int           `config:"max-retries,5"`
	Delay         time.Duration `config:"delay,100ms"`
	MaxDelay      time.Duration `config:"max-delay,30s"`
	BackoffFactor float64       `config:"backoff-factor,2.0"`
}

type KafkaConfig struct {
	Brokers           []string      `config:"brokers"`
	Topic             string        `config:"topic,link-updates"`
	DLQTopic          string        `config:"dlq-topic,link-updates-dlq"`
	SchemaRegistryURL string        `config:"schema-registry-url"`
	ConsumerGroup     string        `config:"consumer-group,bot-group-1"`
	Retries           RetriesConfig `config:"retries"`
}

type Config struct {
	Telegram        TelegramConfig        `config:"telegram"`
	Logger          LoggerConfig          `config:"logger"`
	ScrapperService ScrapperServiceConfig `config:"scrapper-service"`
	Server          ServerConfig          `config:"server"`
	Database        DatabaseConfig        `config:"database"`
	UseQueue        bool                  `config:"use-queue,true"`
	Kafka           KafkaConfig           `config:"kafka"`
}

func Load(configPath string) (*Config, error) {
	tree, err := parse.ParseFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	cfg := &Config{}

	parse.Populate(cfg, tree.GetConfig(), "")

	return cfg, nil
}
