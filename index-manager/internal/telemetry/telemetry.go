// Package telemetry provides Prometheus instrumentation for the index-manager service.
package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all index-manager Prometheus metrics.
type Metrics struct {
	RequestsTotal    *prometheus.CounterVec
	ErrorsTotal      *prometheus.CounterVec
	RequestLatency   prometheus.Histogram
	IndexCount       prometheus.Gauge
	IndexOperations  *prometheus.CounterVec
	AggregationCount *prometheus.CounterVec
}

// Provider wraps Prometheus metrics for the index-manager service.
type Provider struct {
	Metrics *Metrics
}

// NewProvider creates a new telemetry provider with registered metrics.
func NewProvider() *Provider {
	m := &Metrics{}
	initRequestMetrics(m)
	initIndexMetrics(m)
	return &Provider{Metrics: m}
}

func initRequestMetrics(m *Metrics) {
	m.RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "index_manager_requests_total",
		Help: "Total requests by endpoint and status",
	}, []string{"endpoint", "status"})

	m.ErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "index_manager_errors_total",
		Help: "Total errors by type",
	}, []string{"error_type"})

	m.RequestLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "index_manager_request_latency_seconds",
		Help:    "Request latency in seconds",
		Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	})
}

func initIndexMetrics(m *Metrics) {
	m.IndexCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "index_manager_indexes",
		Help: "Current number of managed Elasticsearch indexes",
	})

	m.IndexOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "index_manager_index_operations_total",
		Help: "Total index operations by type",
	}, []string{"operation"})

	m.AggregationCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "index_manager_aggregation_requests_total",
		Help: "Total aggregation requests by type",
	}, []string{"aggregation_type"})
}
