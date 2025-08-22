package app

import (
	"os"
	"os/signal"
	"syscall"
	"web_page_analyzer/internal/http"

	log "github.com/sirupsen/logrus"
)

// App holds the application's dependencies.
type App struct {
	log           *log.Logger
	httpServer    *http.HTTPServer
	metricsServer *http.MetricsServer
	pprofServer   *http.PprofServer
}

// NewApp creates a new application.
func NewApp(
	log *log.Logger,
	httpServer *http.HTTPServer,
	metricsServer *http.MetricsServer,
	pprofServer *http.PprofServer,
) *App {
	return &App{
		log:           log,
		httpServer:    httpServer,
		metricsServer: metricsServer,
		pprofServer:   pprofServer,
	}
}

// Run starts the application and waits for a signal to shut down.
func (a *App) Run() {
	go a.httpServer.Start()
	go a.metricsServer.Start()
	go a.pprofServer.Start()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	a.log.Info("Shutting down application")

	// Shutdown servers
	if err := a.httpServer.Stop(); err != nil {
		a.log.WithError(err).Error("Error shutting down HTTP server")
	}

	if err := a.metricsServer.Stop(); err != nil {
		a.log.WithError(err).Error("Error shutting down metrics server")
	}

	if err := a.pprofServer.Stop(); err != nil {
		a.log.WithError(err).Error("Error shutting down pprof server")
	}

	a.log.Info("Application shut down gracefully")
}
