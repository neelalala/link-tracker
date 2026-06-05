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

type LoggerConfig struct {
	File  string `config:"file,"`
	Level string `config:"level,ERROR"`
}

type DatabaseConfig struct {
	MigrationsDirUrl string     `config:"migrations-dir-url"`
	URL              string     `config:"url"`
	AccessType       AccessType `config:"access-type,BUILDER"`
}

type SchedulerConfig struct {
	FetchInterval time.Duration `config:"fetch-interval"`
	FetchTimeout  time.Duration `config:"fetch-timeout"`
}

type BotServiceConfig struct {
	URL      string   `config:"url"`
	Protocol Protocol `config:"protocol"`
}

type ServerConfig struct {
	Port     uint16   `config:"port"`
	Protocol Protocol `config:"protocol"`
}

type FetchersConfig struct {
	PreviewLimit     int           `config:"preview-limit,200"`
	Timeout          time.Duration `config:"timeout"`
	Concurrency      int           `config:"concurrency,1"`
	Batch            int           `config:"batch,100"`
	StackOverflowKey string        `config:"stackoverflow-key"`
}

type KafkaWorkerConfig struct {
	Count      int           `config:"count,1"`
	Interval   time.Duration `config:"interval,1m"`
	EventLimit int           `config:"event-limit,10"`
	MaxRetries int           `config:"max-retries,5"`
}

type KafkaConfig struct {
	Brokers           []string          `config:"brokers"`
	SchemaRegistryURL string            `config:"schema-registry-url"`
	Topic             string            `config:"topic,link-updates"`
	SchemaPath        string            `config:"schema-path"`
	Workers           KafkaWorkerConfig `config:"workers"`
}

type ValkeyConfig struct {
	Enabled         bool          `config:"enabled,true"`
	Addresses       []string      `config:"addresses"`
	Username        string        `config:"username,"`
	Password        string        `config:"password,"`
	TTL             time.Duration `config:"ttl,1h"`
	KeyPrefix       string        `config:"key-prefix,scrapper:links:"`
	ClientSideCache bool          `config:"client-side-cache,true"`
}

type Config struct {
	Logger     LoggerConfig     `config:"logger"`
	Scheduler  SchedulerConfig  `config:"scheduler"`
	BotService BotServiceConfig `config:"bot-service"`
	Server     ServerConfig     `config:"server"`
	Database   DatabaseConfig   `config:"database"`
	Fetchers   FetchersConfig   `config:"fetchers"`
	UseQueue   bool             `config:"use-queue,true"`
	Kafka      KafkaConfig      `config:"kafka"`
	Valkey     ValkeyConfig     `config:"valkey"`
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
