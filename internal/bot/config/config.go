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

type Config struct {
	Telegram        TelegramConfig        `config:"telegram"`
	Logger          LoggerConfig          `config:"logger"`
	ScrapperService ScrapperServiceConfig `config:"scrapper-service"`
	Server          ServerConfig          `config:"server"`
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
