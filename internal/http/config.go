package http

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Host              string        `mapstructure:"HTTP_SERVER_HOST"`
	ReadTimeout       time.Duration `mapstructure:"HTTP_APP_READ_TIMEOUT_DURATION"`
	ReadHeaderTimeout time.Duration `mapstructure:"HTTP_APP_READ_HEADER_TIMEOUT_DURATION"`
	WriteTimeout      time.Duration `mapstructure:"HTTP_APP_WRITE_TIMEOUT_DURATION"`
	IdleTimeout       time.Duration `mapstructure:"HTTP_APP_IDLE_TIMEOUT_DURATION"`
	ShutdownTimeout   time.Duration `mapstructure:"HTTP_APP_SHUTDOWN_TIMEOUT_DURATION"`
}

func NewConfig() (*Config, error) {
	viper.SetConfigFile(`config.env`)
	viper.SetConfigType("env")

	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	viper.AutomaticEnv()

	cfg := Config{}
	err = viper.Unmarshal(&cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}