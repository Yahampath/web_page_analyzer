package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type AppConfig struct {
	LogLevel    string
	DebugMode   bool
	MetricsHost string
}

func NewAppConfig() (*AppConfig, error) {
	err := godotenv.Load(`config.env`)
	if err != nil {
		return nil, err
	}

	cfg := AppConfig{}
	cfg.LogLevel = os.Getenv("APP_LOG_LEVEL")
	cfg.DebugMode = os.Getenv("APP_ENABLE_DEBUG") == "true"
	cfg.MetricsHost = os.Getenv("HTTP_APP_METRICS_HOST")

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

	if cfg.MetricsHost == "" {
		errMsg = append(errMsg, `metrics host is empty`)
	}

	if len(errMsg) != 0 {
		return fmt.Errorf(`validation failed: %s`, strings.Join(errMsg, "\n"))
	}
	return nil
}
