// Package telemetry provides Prometheus instrumentation for the search service.
package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all search Prometheus metrics.
type Metrics struct {
	RequestsTotal  *prometheus.CounterVec
	ErrorsTotal    *prometheus.CounterVec
	QueryLatency   prometheus.Histogram
	ResultsPerPage prometheus.Histogram
}

// Provider wraps Prometheus metrics for the search service.
type Provider struct {
	Metrics *Metrics
}

// NewProvider creates a new telemetry provider with registered metrics.
func NewProvider() *Provider {
	m := &Metrics{}
	initRequestMetrics(m)
	initQueryMetrics(m)
	return &Provider{Metrics: m}
}

func initRequestMetrics(m *Metrics) {
	m.RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "search_requests_total",
		Help: "Total search requests by endpoint and status",
	}, []string{"endpoint", "status"})

	m.ErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "search_errors_total",
		Help: "Total search errors by type",
	}, []string{"error_type"})
}

func initQueryMetrics(m *Metrics) {
	m.QueryLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "search_query_latency_seconds",
		Help:    "Elasticsearch query latency in seconds",
		Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	})

	m.ResultsPerPage = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "search_results_per_page",
		Help:    "Number of results returned per search request",
		Buckets: []float64{0, 1, 5, 10, 20, 50, 100},
	})
}
