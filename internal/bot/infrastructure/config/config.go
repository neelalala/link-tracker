package config

import (
	"fmt"
	"github.com/byrnedo/typesafe-config/parse"
)

type Protocol string

const (
	HTTP Protocol = "http"
	GRPC Protocol = "grpc"
)

func (p Protocol) Validate() error {
	switch p {
	case HTTP, GRPC:
		return nil
	default:
		return fmt.Errorf("invalid protocol: %q. Allowed values are 'http' or 'grpc'", p)
	}
}

type Config struct {
	TelegramToken string   `config:"telegram-token"`
	Environment   string   `config:"environment,local"`
	LogsFile      string   `config:"logs-file,"`
	ScrapperUrl   string   `config:"scrapper-url"`
	BotApiPort    uint16   `config:"bot-api-port"`
	ApiProtocol   Protocol `config:"api-protocol"`
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
