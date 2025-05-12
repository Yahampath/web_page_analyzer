package http

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"web_page_analyzer/internal/pkg/errors"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"web_page_analyzer/internal/pkg/metrics"
	log "github.com/sirupsen/logrus"
)

type MetricsServer struct {
	host    string
	timeout time.Duration
	server  *http.Server
	log     *log.Logger
}

func NewMetricsServer(host string, timeout time.Duration, log *log.Logger) *MetricsServer {
	reg := metrics.MetricsRegister()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	return &MetricsServer{
		server: &http.Server{
			Addr:    host,
			Handler: mux,
		},
		host:    host,
		timeout: timeout,
		log:     log,
	}
}

func (m *MetricsServer) Start() error {
	m.log.Info("metrics server starting on port ", m.host)
	if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (m *MetricsServer) Stop() error {
	if m.server == nil {
		return fmt.Errorf("server is not initialized")
	}
	m.log.Info("shutting down metrics server...")

	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	if err := m.server.Shutdown(ctx); err != nil {
		return errors.Wrap(err, `failed to shutdown metrics server`)
	}

	m.log.Info("metrics server exiting")
	return nil
}
