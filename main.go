package main

import (
	"context"
	_ "net/http/pprof"
	"time"
	"web_page_analyzer/internal/application/config"
	"web_page_analyzer/internal/http"

	log "github.com/sirupsen/logrus"
)

func main() {
	logInstance := log.New()
	cfg, err := config.NewAppConfig()
	if err != nil {
		logInstance.WithError(err).Fatal(`Failed to lod config`)
		return
	}

	//log level
	logLevel, err := log.ParseLevel(cfg.LogLevel)
	if err != nil {
		logInstance.WithError(err).Fatal(`Failed to parse log level`)
		return
	}

	logInstance.SetFormatter(&log.JSONFormatter{
		TimestampFormat:   time.RFC3339,
		DisableHTMLEscape: true,
		DisableTimestamp:  false,
	})

	logInstance.SetLevel(logLevel)

	// Get context
	ctx := context.WithoutCancel(context.Background())

	// Init HTTP
	http.Init(ctx, logInstance, cfg)
}
