package http

import (
	"context"
	"time"
	"web_page_analyzer/internal/adaptors"
	"web_page_analyzer/internal/http/handlers"
	"web_page_analyzer/internal/http/middleware"
	"web_page_analyzer/internal/platform/storage"
	"web_page_analyzer/internal/service"

	"github.com/go-chi/chi/v5"
)

func initRoutes(_ context.Context, r *Router) {
	jobStore := storage.NewInMemoryJobRepository()

	r.httpRouter.Use(middleware.MetricsMiddleware)
	r.httpRouter.Use(middleware.RequestIDLoggerMiddleware(r.log))

	// Health check
	r.httpRouter.Get("/ready", handlers.NewReadyHandler().Handle)

	// API routes
	r.httpRouter.Route("/api/v1", func(apiRouter chi.Router) {
		analyzerService := service.NewAnalyzer(r.log, adaptors.NewWebClient(10*time.Second, r.log))
		jobService := service.NewJobService(analyzerService, jobStore, r.log, 10)
		apiRouter.Post("/analyze", handlers.NewWebPageAnalysisHandler(jobService, r.log).Handle)
		apiRouter.Get("/status/{job_id}", handlers.NewStatusHandler(jobStore, r.log).Handle)
		apiRouter.Get("/result/{job_id}", handlers.NewResultHandler(jobStore, r.log).Handle)
	})
}
