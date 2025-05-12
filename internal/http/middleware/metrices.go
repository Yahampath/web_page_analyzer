package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"web_page_analyzer/internal/pkg/metrics"
)

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srw := &metricsStatusRecorder{ResponseWriter: w}
		start := time.Now()

		next.ServeHTTP(srw, r)
		if srw.status == 0 {
			srw.status = http.StatusOK
		}
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = r.URL.Path
		}
		duration := time.Since(start).Seconds()

		codeStr := strconv.Itoa(srw.status)
		metrics.HTTPRequestsTotal.WithLabelValues(r.Method, route, codeStr).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(r.Method, route).Observe(duration)

		// Increment error counter on 4xx or 5xx
		if srw.status >= 400 {
			metrics.HTTPRequestErrorsTotal.WithLabelValues(r.Method, route, codeStr).Inc()
		}
	})
}

type metricsStatusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *metricsStatusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
