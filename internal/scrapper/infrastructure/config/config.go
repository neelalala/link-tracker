package config

import (
	"fmt"
	"github.com/byrnedo/typesafe-config/parse"
)

type Protocol string
type AccessType string

const (
	HTTP Protocol = "http"
	GRPC Protocol = "grpc"

	SQL     AccessType = "SQL"
	Builder AccessType = "BUILDER"
)

func (protocol Protocol) Validate() error {
	switch protocol {
	case HTTP, GRPC:
		return nil
	default:
		return fmt.Errorf("invalid protocol: %q. Allowed values are 'http' or 'grpc'", protocol)
	}
}

func (accessType AccessType) Validate() error {
	switch accessType {
	case Builder, SQL:
		return nil
	default:
		return fmt.Errorf("invalid access type: %q. Allowed values are 'SQL' or 'BUILDER'", accessType)
	}
}

type Config struct {
	UpdatesIntervalSeconds int        `config:"updates-interval-seconds"`
	Environment            string     `config:"environment,local"`
	LogsFile               string     `config:"logs-file,"`
	LogLevel               string     `config:"log-level,error"`
	BotUrl                 string     `config:"bot-url"`
	ScrapperApiPort        uint16     `config:"scrapper-api-port"`
	ApiProtocol            Protocol   `config:"api-protocol"`
	MigrationsDir          string     `config:"migrations-dir"`
	DatabaseUrl            string     `config:"database-url"`
	AccessType             AccessType `config:"access-type,builder"`
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
