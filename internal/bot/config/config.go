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

type Config struct {
	TelegramToken   string        `config:"telegram-token"`
	LogsFile        string        `config:"logs-file,"`
	LogLevel        string        `config:"log-level,error"`
	ScrapperUrl     string        `config:"scrapper-url"`
	ApiPort         uint16        `config:"api-port"`
	ScrapperTimeout time.Duration `config:"scrapper-timeout,30s"`
	ApiProtocol     Protocol      `config:"api-protocol"`
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
