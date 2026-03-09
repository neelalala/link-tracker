package config

import (
	"fmt"
	"github.com/byrnedo/typesafe-config/parse"
)

type Config struct {
	UpdateIntervalSeconds int    `yaml:"update-interval-seconds"`
	Environment           string `config:"environment,local"`
	LogsFile              string `config:"logs-file,"`
	BotUrl                string `config:"bot-url"`
	ScrapperApiPort       uint16 `config:"scrapper-api-port"`
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
