// Package telemetry provides Prometheus instrumentation for the auth service.
package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all auth Prometheus metrics.
type Metrics struct {
	LoginTotal    *prometheus.CounterVec
	LoginLatency  prometheus.Histogram
	TokensIssued  prometheus.Counter
	InvalidLogins prometheus.Counter
}

// Provider wraps Prometheus metrics for the auth service.
type Provider struct {
	Metrics *Metrics
}

// NewProvider creates a new telemetry provider with registered metrics.
func NewProvider() *Provider {
	m := &Metrics{}
	initLoginMetrics(m)
	return &Provider{Metrics: m}
}

func initLoginMetrics(m *Metrics) {
	m.LoginTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_login_total",
		Help: "Total login attempts by result",
	}, []string{"result"})

	m.LoginLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "auth_login_latency_seconds",
		Help:    "Login request latency in seconds",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
	})

	m.TokensIssued = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_tokens_issued_total",
		Help: "Total JWT tokens successfully issued",
	})

	m.InvalidLogins = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_invalid_logins_total",
		Help: "Total invalid login attempts (wrong credentials)",
	})
}
