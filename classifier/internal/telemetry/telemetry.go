// Package telemetry provides OpenTelemetry instrumentation for the classifier service.
// It exports Prometheus metrics and provides tracing capabilities.
package telemetry

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "classifier"

// Metrics holds all classifier Prometheus metrics
type Metrics struct {
	// Processing metrics
	DocumentsProcessed *prometheus.CounterVec
	DocumentsFailed    *prometheus.CounterVec
	DocumentsRetried   prometheus.Counter
	ProcessingDuration *prometheus.HistogramVec
	BatchSize          prometheus.Histogram

	// Rule engine metrics
	RuleMatchDuration prometheus.Histogram
	RulesEvaluated    prometheus.Counter
	RulesMatched      prometheus.Counter

	// Backpressure metrics
	QueueDepth    prometheus.Gauge
	ActiveWorkers prometheus.Gauge
	WorkDropped   prometheus.Counter
	ThrottleCount prometheus.Counter

	// Lag metrics (freshness SLO)
	PollerLag         prometheus.Histogram
	ClassificationLag prometheus.Histogram

	// DLQ metrics
	DLQDepth     prometheus.Gauge
	DLQEnqueued  *prometheus.CounterVec
	DLQProcessed prometheus.Counter
	DLQExhausted prometheus.Counter

	// Outbox metrics
	OutboxBacklog    prometheus.Gauge
	OutboxPublished  prometheus.Counter
	OutboxPublishLag prometheus.Histogram

	// Content type distribution (article vs page vs listing vs unknown)
	ContentTypeTotal *prometheus.CounterVec
}

// Provider wraps telemetry providers
type Provider struct {
	Tracer  trace.Tracer
	Metrics *Metrics
}

// NewProvider initializes telemetry with Prometheus metrics
func NewProvider() *Provider {
	metrics := initMetrics()
	tracer := otel.Tracer(serviceName)

	return &Provider{
		Tracer:  tracer,
		Metrics: metrics,
	}
}

// Handler returns the Prometheus HTTP handler for /metrics endpoint
func (p *Provider) Handler() http.Handler {
	return promhttp.Handler()
}

func initMetrics() *Metrics {
	m := &Metrics{}
	initProcessingMetrics(m)
	initRuleEngineMetrics(m)
	initBackpressureMetrics(m)
	initLagMetrics(m)
	initDLQMetrics(m)
	initOutboxMetrics(m)
	initContentTypeMetrics(m)
	return m
}

func initContentTypeMetrics(m *Metrics) {
	m.ContentTypeTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "classifier_content_type_total",
		Help: "Total documents classified by content_type (article, page, listing, unknown)",
	}, []string{"content_type"})
}

func initProcessingMetrics(m *Metrics) {
	m.DocumentsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "classifier_documents_processed_total",
		Help: "Total documents successfully classified",
	}, []string{"source"})

	m.DocumentsFailed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "classifier_documents_failed_total",
		Help: "Total documents that failed classification",
	}, []string{"source", "error_code"})

	m.DocumentsRetried = promauto.NewCounter(prometheus.CounterOpts{
		Name: "classifier_documents_retried_total",
		Help: "Total documents retried from DLQ",
	})

	m.ProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "classifier_processing_duration_seconds",
		Help:    "Time to classify a single document",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
	}, []string{"source"})

	m.BatchSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "classifier_batch_size",
		Help:    "Number of documents per batch",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 200, 500},
	})
}

func initRuleEngineMetrics(m *Metrics) {
	m.RuleMatchDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "classifier_rule_match_duration_seconds",
		Help:    "Time spent in rule matching (Aho-Corasick)",
		Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1},
	})

	m.RulesEvaluated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "classifier_rules_evaluated_total",
		Help: "Total rule evaluations",
	})

	m.RulesMatched = promauto.NewCounter(prometheus.CounterOpts{
		Name: "classifier_rules_matched_total",
		Help: "Total rules that matched",
	})
}

func initBackpressureMetrics(m *Metrics) {
	m.QueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "classifier_queue_depth",
		Help: "Current pending documents in work queue",
	})

	m.ActiveWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "classifier_active_workers",
		Help: "Currently active worker goroutines",
	})

	m.WorkDropped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "classifier_work_dropped_total",
		Help: "Work items dropped due to queue full",
	})

	m.ThrottleCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "classifier_throttle_count_total",
		Help: "Number of times poller was throttled due to backpressure",
	})
}

func initLagMetrics(m *Metrics) {
	m.PollerLag = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "classifier_poller_lag_seconds",
		Help:    "Time between document creation and classification start",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600},
	})

	m.ClassificationLag = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "classifier_classification_lag_seconds",
		Help:    "End-to-end lag from crawl to classified",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600},
	})
}

func initDLQMetrics(m *Metrics) {
	m.DLQDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "classifier_dlq_depth",
		Help: "Current documents in dead-letter queue",
	})

	m.DLQEnqueued = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "classifier_dlq_enqueued_total",
		Help: "Total documents added to DLQ",
	}, []string{"source", "error_code"})

	m.DLQProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "classifier_dlq_processed_total",
		Help: "Total documents successfully processed from DLQ",
	})

	m.DLQExhausted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "classifier_dlq_exhausted_total",
		Help: "Total documents that exhausted all retries",
	})
}

func initOutboxMetrics(m *Metrics) {
	m.OutboxBacklog = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "classifier_outbox_backlog",
		Help: "Current unpublished documents in outbox",
	})

	m.OutboxPublished = promauto.NewCounter(prometheus.CounterOpts{
		Name: "classifier_outbox_published_total",
		Help: "Total documents published from outbox",
	})

	m.OutboxPublishLag = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "classifier_outbox_publish_lag_seconds",
		Help:    "Time between outbox insert and publish",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
	})
}

// RecordClassification records metrics for a single classification
func (p *Provider) RecordClassification(ctx context.Context, source string, success bool, duration time.Duration) {
	if success {
		p.Metrics.DocumentsProcessed.WithLabelValues(source).Inc()
	}
	p.Metrics.ProcessingDuration.WithLabelValues(source).Observe(duration.Seconds())
}

// RecordContentType increments the content_type counter (article, page, listing, or unknown).
func (p *Provider) RecordContentType(ctx context.Context, contentType string) {
	label := contentType
	if label == "" {
		label = "unknown"
	}
	p.Metrics.ContentTypeTotal.WithLabelValues(label).Inc()
}

// RecordClassificationFailure records a failed classification with error code
func (p *Provider) RecordClassificationFailure(ctx context.Context, source, errorCode string) {
	p.Metrics.DocumentsFailed.WithLabelValues(source, errorCode).Inc()
}

// RecordRuleMatch records rule engine metrics
func (p *Provider) RecordRuleMatch(ctx context.Context, duration time.Duration, rulesEvaluated, rulesMatched int) {
	p.Metrics.RuleMatchDuration.Observe(duration.Seconds())
	p.Metrics.RulesEvaluated.Add(float64(rulesEvaluated))
	p.Metrics.RulesMatched.Add(float64(rulesMatched))
}

// RecordPollerLag records the freshness lag
func (p *Provider) RecordPollerLag(ctx context.Context, createdAt time.Time) {
	lag := time.Since(createdAt)
	p.Metrics.PollerLag.Observe(lag.Seconds())
}

// RecordClassificationLag records end-to-end lag
func (p *Provider) RecordClassificationLag(ctx context.Context, crawledAt time.Time) {
	lag := time.Since(crawledAt)
	p.Metrics.ClassificationLag.Observe(lag.Seconds())
}

// RecordDLQEnqueue records a DLQ enqueue event
func (p *Provider) RecordDLQEnqueue(ctx context.Context, source, errorCode string) {
	p.Metrics.DLQEnqueued.WithLabelValues(source, errorCode).Inc()
	p.Metrics.DLQDepth.Inc()
}

// RecordDLQProcessed records a successful DLQ processing
func (p *Provider) RecordDLQProcessed(ctx context.Context) {
	p.Metrics.DLQProcessed.Inc()
	p.Metrics.DLQDepth.Dec()
}

// RecordDLQExhausted records a DLQ entry that exhausted retries
func (p *Provider) RecordDLQExhausted(ctx context.Context) {
	p.Metrics.DLQExhausted.Inc()
	p.Metrics.DLQDepth.Dec()
}

// RecordOutboxPublish records outbox publish metrics
func (p *Provider) RecordOutboxPublish(ctx context.Context, insertedAt time.Time, success bool) {
	lag := time.Since(insertedAt)
	p.Metrics.OutboxPublishLag.Observe(lag.Seconds())
	if success {
		p.Metrics.OutboxPublished.Inc()
		p.Metrics.OutboxBacklog.Dec()
	}
}

// SetQueueDepth sets the current queue depth
func (p *Provider) SetQueueDepth(depth int) {
	p.Metrics.QueueDepth.Set(float64(depth))
}

// SetActiveWorkers sets the current active worker count
func (p *Provider) SetActiveWorkers(count int) {
	p.Metrics.ActiveWorkers.Set(float64(count))
}

// SetDLQDepth sets the current DLQ depth
func (p *Provider) SetDLQDepth(depth int) {
	p.Metrics.DLQDepth.Set(float64(depth))
}

// SetOutboxBacklog sets the current outbox backlog
func (p *Provider) SetOutboxBacklog(backlog int) {
	p.Metrics.OutboxBacklog.Set(float64(backlog))
}

// IncrementWorkDropped increments the dropped work counter
func (p *Provider) IncrementWorkDropped() {
	p.Metrics.WorkDropped.Inc()
}

// IncrementThrottleCount increments the throttle counter
func (p *Provider) IncrementThrottleCount() {
	p.Metrics.ThrottleCount.Inc()
}

// RecordBatchSize records the size of a processed batch
func (p *Provider) RecordBatchSize(size int) {
	p.Metrics.BatchSize.Observe(float64(size))
}

// StartSpan starts a new trace span.
// The caller is responsible for ending the span with span.End().
//
//nolint:spancheck // Caller is responsible for ending the span
func (p *Provider) StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	ctx, span := p.Tracer.Start(ctx, name, trace.WithAttributes(attrs...))
	return ctx, span
}
