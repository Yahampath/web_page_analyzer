package http

import (
	"context"
	"time"
	"web_page_analyzer/internal/adaptors"
	"web_page_analyzer/internal/http/handlers"
	"web_page_analyzer/internal/http/middleware"
	"web_page_analyzer/internal/service"
)

func initRoutes(_ context.Context, r *Router) {
	r.httpRouter.Use(middleware.MetricsMiddleware)
	r.httpRouter.Use(middleware.RequestIDLoggerMiddleware(r.log))
	// Routes
	r.httpRouter.Get("/ready", handlers.NewReadyHandler().Handle)
	r.httpRouter.Post("/analyze", handlers.NewWebPageAnalysisHandler(service.NewAnalyzer(r.log, adaptors.NewWebClient(5*time.Second, r.log)), r.log).Handle)
}
