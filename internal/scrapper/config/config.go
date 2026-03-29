package config

import (
	"fmt"
	"github.com/byrnedo/typesafe-config/parse"
	"time"
)

type Protocol string

const (
	HTTP Protocol = "http"
	GRPC Protocol = "grpc"
)

func (protocol Protocol) Validate() error {
	switch protocol {
	case HTTP, GRPC:
		return nil
	default:
		return fmt.Errorf("invalid protocol: %q. Allowed values are 'http' or 'grpc'", protocol)
	}
}

type LoggerConfig struct {
	File  string `config:"file,"`
	Level string `config:"level,ERROR"`
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

type Config struct {
	Logger     LoggerConfig     `config:"logger"`
	Scheduler  SchedulerConfig  `config:"scheduler"`
	BotService BotServiceConfig `config:"bot-service"`
	Server     ServerConfig     `config:"server"`
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
