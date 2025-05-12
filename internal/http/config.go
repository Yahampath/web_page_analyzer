package http

import (
	"fmt"
	"os"
	"strings"
	"time"
	"github.com/joho/godotenv"
)

type HTTPServerConfig struct {
	Host     string
	Timeouts struct {
		Read         time.Duration
		ReadHeader   time.Duration
		Write        time.Duration
		Idle         time.Duration
		ShutdownWait time.Duration
	}
}

func NewHTTPServerConfig() (*HTTPServerConfig, error) {
	err := godotenv.Load(`config.env`)
	if err != nil {
		return nil, fmt.Errorf("error loading .env file: %w", err)
	}

	var errors []string
	cfg := &HTTPServerConfig{}

	// Parse Host
	cfg.Host = os.Getenv("HTTP_SERVER_HOST")
	if cfg.Host == "" {
		errors = append(errors, "HTTP_SERVER_HOST is required")
	}

	// Helper function to parse duration environment variables
	parseDuration := func(envVar string) (time.Duration, error) {
		value := os.Getenv(envVar)
		if value == "" {
			return 0, fmt.Errorf("%s is required", envVar)
		}
		duration, err := time.ParseDuration(value)
		if err != nil {
			return 0, fmt.Errorf("%s: invalid duration format: %w", envVar, err)
		}
		return duration, nil
	}

	// Parse timeouts
	if dur, err := parseDuration("HTTP_APP_READ_TIMEOUT_DURATION"); err != nil {
		errors = append(errors, err.Error())
	} else {
		cfg.Timeouts.Read = dur
	}

	if dur, err := parseDuration("HTTP_APP_READ_HEADER_TIMEOUT_DURATION"); err != nil {
		errors = append(errors, err.Error())
	} else {
		cfg.Timeouts.ReadHeader = dur
	}

	if dur, err := parseDuration("HTTP_APP_WRITE_TIMEOUT_DURATION"); err != nil {
		errors = append(errors, err.Error())
	} else {
		cfg.Timeouts.Write = dur
	}

	if dur, err := parseDuration("HTTP_APP_IDLE_TIMEOUT_DURATION"); err != nil {
		errors = append(errors, err.Error())
	} else {
		cfg.Timeouts.Idle = dur
	}

	if dur, err := parseDuration("HTTP_APP_SHUTDOWN_TIMEOUT_DURATION"); err != nil {
		errors = append(errors, err.Error())
	} else {
		cfg.Timeouts.ShutdownWait = dur
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("configuration validation failed:\n%s", strings.Join(errors, "\n"))
	}

	return cfg, nil
}