package http

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"web_page_analyzer/internal/pkg/errors"

	log "github.com/sirupsen/logrus"
)

type PprofServer struct {
	host    string
	timeout time.Duration
	server  *http.Server
	log     *log.Logger
}

func NewPprofServer(host string, timeout time.Duration, log *log.Logger) *PprofServer {
	return &PprofServer{
		server: &http.Server{
			Addr:    host,
			Handler: nil,
		},
		host:    host,
		timeout: timeout,
		log:     log,
	}
}

func (s *PprofServer) Start() error {
	s.log.Info("PPProf server starting on port ", s.host)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *PprofServer) Stop() error {
	if s.server == nil {
		return fmt.Errorf("server is not initialized")
	}
	s.log.Info("shutting down pprof server...")

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return errors.Wrap(err, `failed to shutdown pprof server`)
	}

	s.log.Info("pprof server exiting")
	return nil
}
