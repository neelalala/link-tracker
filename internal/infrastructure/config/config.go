package config

import (
	"fmt"
	"github.com/byrnedo/typesafe-config/parse"
)

type Config struct {
	TelegramToken string `config:"telegram-token"`
	Environment   string `config:"environment,local"`
	LogsFile      string `config:"logs-file,"`
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
