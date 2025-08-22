//go:build wireinject
// +build wireinject

package app

import (
	"context"
	"net/http"
	"time"
	"web_page_analyzer/internal/adaptors"
	"web_page_analyzer/internal/application/config"
	domainStorage "web_page_analyzer/internal/domain/storage"
	internalHTTP "web_page_analyzer/internal/http"
	"web_page_analyzer/internal/http/handlers"
	"web_page_analyzer/internal/http/middleware"
	platformStorage "web_page_analyzer/internal/platform/storage"
	"web_page_analyzer/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/wire"
	"github.com/sirupsen/logrus"
)

func InitializeApp(ctx context.Context) (*App, func(), error) {
	panic(wire.Build(
		// App
		NewApp,

		// Logger
		provideLogger,

		// Config
		config.NewAppConfig,
		internalHTTP.NewConfig,

		// Router
		provideRouter,
		chi.NewRouter,

		// services
		service.NewAnalyzer,
		service.NewJobService,

		// adaptors
		adaptors.NewWebClient,

		// stores
		platformStorage.NewInMemoryJobStore,

		// handlers
		handlers.NewReadyHandler,
		handlers.NewWebPageAnalysisHandler,
		handlers.NewStatusHandler,
		handlers.NewResultHandler,

		// servers
		internalHTTP.NewHttpServer,
		internalHTTP.NewMetricsServer,
		internalHTTP.NewPprofServer,

		// Bind interfaces to implementations
		wire.Bind(new(service.WebPageAnalyzer), new(*service.Analyzer)),
		wire.Bind(new(domainStorage.JobStore), new(*platformStorage.InMemoryJobStore)),
		wire.Bind(new(adaptors.WebClient), new(*adaptors.WebClient)),
		wire.Bind(new(http.Handler), new(*chi.Mux)),

		// Provide values
		provideWebClientTimeout,
		provideNumWorkers,
	))
}

func provideLogger(appCfg *config.AppConfig) (*logrus.Logger, error) {
	log := logrus.New()
	logLevel, err := logrus.ParseLevel(appCfg.LogLevel)
	if err != nil {
		return nil, err
	}
	log.SetLevel(logLevel)
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})
	return log, nil
}

func provideRouter(
	log *logrus.Logger,
	webPageAnalysisHandler *handlers.WebPageAnalysisHandler,
	statusHandler *handlers.StatusHandler,
	resultHandler *handlers.ResultHandler,
	readyHandler *handlers.ReadyHandler,
	httpRouter *chi.Mux,
) *chi.Mux {
	httpRouter.Use(middleware.MetricsMiddleware)
	httpRouter.Use(middleware.RequestIDLoggerMiddleware(log))

	httpRouter.Get("/ready", readyHandler.Handle)

	httpRouter.Route("/api/v1", func(apiRouter chi.Router) {
		apiRouter.Post("/analyze", webPageAnalysisHandler.Handle)
		apiRouter.Get("/status/{job_id}", statusHandler.Handle)
		apiRouter.Get("/result/{job_id}", resultHandler.Handle)
	})

	return httpRouter
}

func provideWebClientTimeout() time.Duration {
	return 5 * time.Second
}

func provideNumWorkers(appCfg *config.AppConfig) int {
	return appCfg.Workers
}
