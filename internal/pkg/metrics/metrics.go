// internal/metrics/metrics.go
package metrics

import (
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// --- Inbound (server) metrics ---
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_server_requests_total",
			Help: "Total number of HTTP requests processed.",
		},
		[]string{"method", "route", "code"},
	)
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_server_request_duration_seconds",
			Help:    "Latency of HTTP requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)
	HTTPRequestErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_server_request_errors_total",
			Help: "Total number of HTTP requests resulting in client or server errors.",
		},
		[]string{"method", "route", "code"},
	)

	// --- Outbound (client) metrics ---
	HTTPClientRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_client_requests_total",
			Help: "Total number of outbound HTTP requests.",
		},
		[]string{"method", "code"},
	)
	HTTPClientRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_client_request_duration_seconds",
			Help:    "Latency of outbound HTTP requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "code"},
	)
	HTTPClientErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_client_request_errors_total",
			Help: "Total number of outbound HTTP requests that failed or returned error status.",
		},
		[]string{"method", "code"},
	)

	// --- Runtime metrics ---
	CPUCount = promauto.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "process_cpu_count",
			Help: "Number of CPU cores available.",
		},
		func() float64 { return float64(runtime.NumCPU()) },
	)
)

func MetricsRegister() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	// 2) register exactly once
	reg.MustRegister(
		prometheus.NewGoCollector(),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		HTTPRequestsTotal,
		HTTPRequestDuration,
		HTTPRequestErrorsTotal,
		HTTPClientRequestsTotal,
		HTTPClientRequestDuration,
		HTTPClientErrorsTotal,
		CPUCount,
	)

	return reg
}
