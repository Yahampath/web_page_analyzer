package http

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
)

type HTTPServer struct{
	config *HTTPServerConfig
	server *http.Server
	log *logrus.Logger
}


func NewHttpServer(ctx context.Context, config *HTTPServerConfig, router *chi.Mux, log *logrus.Logger) *HTTPServer {
	return &HTTPServer{
		config: config,
		server: &http.Server{
			Addr:    config.Host,
			Handler: router,
			ReadTimeout: config.Timeouts.Read,
			ReadHeaderTimeout: config.Timeouts.ReadHeader,
			WriteTimeout: config.Timeouts.Write,
			IdleTimeout: config.Timeouts.Idle,
		},
		log: log,	
	}
}

func (s *HTTPServer) Start() error {
	s.log.Info("Starting HTTP server on: ", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *HTTPServer) Stop() error {
	if s.server == nil {
		return fmt.Errorf("server is not initialized")
	}
	s.log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), s.config.Timeouts.ShutdownWait)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	s.log.Info("Server exiting")
	return nil
}


