package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type AppConfig struct {
	LogLevel    string `mapstructure:"APP_LOG_LEVEL"`
	DebugMode   bool   `mapstructure:"APP_ENABLE_DEBUG"`
	MetricsHost string `mapstructure:"HTTP_APP_METRICS_HOST"`
}

func NewAppConfig() (*AppConfig, error) {
	viper.SetConfigFile(`config.env`)
	viper.SetConfigType("env")

	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	viper.AutomaticEnv()

	cfg := AppConfig{}
	err = viper.Unmarshal(&cfg)
	if err != nil {
		return nil, err
	}

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
