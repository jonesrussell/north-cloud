// Package telemetry provides Prometheus instrumentation for the publisher service.
package telemetry

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all publisher Prometheus metrics.
type Metrics struct {
	// Cursor lag: time between the oldest unprocessed document and now.
	CursorLag prometheus.Gauge

	// Routing metrics
	DocumentsPublished *prometheus.CounterVec
	DocumentsSkipped   prometheus.Counter
	RoutingErrors      prometheus.Counter

	// Batch metrics
	BatchSize      prometheus.Histogram
	PollDuration   prometheus.Histogram
	ChannelsPerDoc prometheus.Histogram

	// Dedup metrics
	DedupHits prometheus.Counter
}

// Provider wraps Prometheus metrics for the publisher.
type Provider struct {
	Metrics *Metrics
}

// NewProvider creates a new telemetry provider with registered metrics.
func NewProvider() *Provider {
	m := &Metrics{}
	initCursorMetrics(m)
	initRoutingMetrics(m)
	initBatchMetrics(m)
	initDedupMetrics(m)
	return &Provider{Metrics: m}
}

// Handler returns the Prometheus HTTP handler for /metrics endpoint.
func (p *Provider) Handler() http.Handler {
	return promhttp.Handler()
}

// RecordCursorLag sets the current cursor lag (time since the sort key timestamp).
func (p *Provider) RecordCursorLag(sortTimestamp time.Time) {
	if sortTimestamp.IsZero() {
		return
	}
	p.Metrics.CursorLag.Set(time.Since(sortTimestamp).Seconds())
}

// RecordPublish records a successful publish to a channel.
func (p *Provider) RecordPublish(channel string) {
	p.Metrics.DocumentsPublished.WithLabelValues(channel).Inc()
}

// RecordSkip records a skipped document.
func (p *Provider) RecordSkip() {
	p.Metrics.DocumentsSkipped.Inc()
}

// RecordRoutingError records a routing/publish error.
func (p *Provider) RecordRoutingError() {
	p.Metrics.RoutingErrors.Inc()
}

// RecordBatch records batch-level metrics after a poll cycle.
func (p *Provider) RecordBatch(size int, duration time.Duration) {
	p.Metrics.BatchSize.Observe(float64(size))
	p.Metrics.PollDuration.Observe(duration.Seconds())
}

// RecordChannelsPerDoc records how many channels a single document was published to.
func (p *Provider) RecordChannelsPerDoc(count int) {
	p.Metrics.ChannelsPerDoc.Observe(float64(count))
}

// RecordDedupHit records a deduplication hit (document already published to channel).
func (p *Provider) RecordDedupHit() {
	p.Metrics.DedupHits.Inc()
}

func initCursorMetrics(m *Metrics) {
	m.CursorLag = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "publisher_cursor_lag_seconds",
		Help: "Seconds between the current cursor position and now",
	})
}

func initRoutingMetrics(m *Metrics) {
	m.DocumentsPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "publisher_documents_published_total",
		Help: "Total documents published, by channel",
	}, []string{"channel"})

	m.DocumentsSkipped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "publisher_documents_skipped_total",
		Help: "Total documents skipped (dedup, quality, content type filter)",
	})

	m.RoutingErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "publisher_routing_errors_total",
		Help: "Total routing or publish errors",
	})
}

func initBatchMetrics(m *Metrics) {
	m.BatchSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "publisher_batch_size",
		Help:    "Number of content items per poll batch",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 200, 500},
	})

	m.PollDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "publisher_poll_duration_seconds",
		Help:    "Time to process a single poll cycle",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
	})

	m.ChannelsPerDoc = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "publisher_channels_per_document",
		Help:    "Number of channels a single document is published to",
		Buckets: []float64{0, 1, 2, 3, 5, 8, 10, 15, 20, 30},
	})
}

func initDedupMetrics(m *Metrics) {
	m.DedupHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "publisher_dedup_hits_total",
		Help: "Total deduplication hits (content already published to channel)",
	})
}
