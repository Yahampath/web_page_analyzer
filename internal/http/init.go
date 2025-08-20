package http

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"web_page_analyzer/internal/application/config"

	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"
)

type Router struct {
	httpRouter *chi.Mux
	log        *log.Logger
}

func Init(ctx context.Context, log *log.Logger, appCfg *config.AppConfig, httpCfg *Config) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	chiRouter := chi.NewRouter()
	router := &Router{
		httpRouter: chiRouter,
		log:        log,
	}

	initRoutes(ctx, router)

	// Create metrics server
	MetricsServer := NewMetricsServer(appCfg.MetricsHost, httpCfg.ShutdownTimeout, log)
	go MetricsServer.Start()

	// Create HTTP server
	httpServer := NewHttpServer(ctx, httpCfg, router.httpRouter, log)
	go httpServer.Start()

	// Create pprof server (uses default http.DefaultServeMux)
	pprofServer := NewPprofServer(":6060", httpCfg.ShutdownTimeout, log)
	go pprofServer.Start()

	<-sigs
	err := httpServer.Stop()
	if err != nil {
		log.Fatal(err)
	}

	err = pprofServer.Stop()
	if err != nil {
		log.Fatal(err)
	}

	err = MetricsServer.Stop()
	if err != nil {
		log.Fatal(err)
	}
}
