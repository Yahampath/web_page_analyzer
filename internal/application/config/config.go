package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type AppConfig struct {
	HttpServerAddr string
	LogLevel       string
	DebugMode      bool
}

func NewAppConfig() (*AppConfig, error) {
	err := godotenv.Load(`config.env`)
	if err != nil {
		return nil, err
	}

	cfg := AppConfig{}
	cfg.HttpServerAddr = os.Getenv("HTTP_SERVER_ADDRESS")
	cfg.LogLevel = os.Getenv("LOG_LEVEL")
	cfg.DebugMode = os.Getenv("DEBUG") == "true"

	err = validate(&cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (cfg *AppConfig) IsDebugMode() bool {
	return cfg.DebugMode
}

func (cfg *AppConfig) GetLogLevel() string {
	return cfg.LogLevel
}

func validate(cfg *AppConfig) error {
	var errMsg []string

	if cfg.HttpServerAddr == "" {
		errMsg = append(errMsg, `http server address is empty`)
	}

	if cfg.LogLevel == "" {
		errMsg = append(errMsg, `log level is empty`)
	}

	if len(errMsg) != 0 {
		return fmt.Errorf(`validation failed: %s`, strings.Join(errMsg, "\n"))
	}
	return nil
}
