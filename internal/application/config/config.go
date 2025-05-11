package config

import (
	"fmt"
	"os"
	"strings"
	"github.com/joho/godotenv"
)

type AppConfig struct {
	LogLevel       string
	DebugMode      bool
}

func NewAppConfig() (*AppConfig, error) {
	err := godotenv.Load(`config.env`)
	if err != nil {
		return nil, err
	}

	cfg := AppConfig{}
	cfg.LogLevel = os.Getenv("LOG_LEVEL")
	cfg.DebugMode = os.Getenv("ENABLE_DEBUG") == "true"

	err = validate(&cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validate(cfg *AppConfig) error {
	var errMsg []string
	if cfg.LogLevel == "" {
		errMsg = append(errMsg, `log level is empty`)
	}

	if len(errMsg) != 0 {
		return fmt.Errorf(`validation failed: %s`, strings.Join(errMsg, "\n"))
	}
	return nil
}
