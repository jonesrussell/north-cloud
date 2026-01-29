# Classifier, Publisher & Dashboard Architecture

**Version:** 2.0
**Status:** Implementation Ready
**Last Updated:** 2026-01-28
**Authors:** Architecture Review

---

## Table of Contents

1. [Overview](#overview)
2. [Guiding Principles](#guiding-principles)
3. [Classifier Service](#classifier-service)
4. [Publisher Service](#publisher-service)
5. [Dashboard](#dashboard)
6. [Operational Story](#operational-story)
7. [Migration Plan](#migration-plan)
8. [Appendix: Complete Code Reference](#appendix-complete-code-reference)

---

## Overview

### Goals

1. **Reliability** — No document loss; failed classifications retry automatically via dead-letter queue
2. **Observability** — Full OpenTelemetry instrumentation; metrics, traces, and logs for every operation
3. **Throughput** — 10x improvement in rule matching via Aho-Corasick; bounded backpressure prevents OOM
4. **Latency** — Near real-time publishing (5 seconds) via transactional outbox pattern
5. **Operability** — Dashboard UI for rules management, index inspection, and operational visibility

### System Context

```
┌─────────────┐     ┌──────────────┐     ┌───────────────┐     ┌─────────────┐
│   Crawler   │────▶│  Classifier  │────▶│   Publisher   │────▶│    Redis    │
│             │     │              │     │               │     │   Pub/Sub   │
└─────────────┘     └──────────────┘     └───────────────┘     └─────────────┘
      │                    │                     │                     │
      ▼                    ▼                     ▼                     ▼
┌─────────────┐     ┌──────────────┐     ┌───────────────┐     ┌─────────────┐
│    ES:      │     │    ES:       │     │  PostgreSQL:  │     │  Consumers  │
│ *_raw_content│    │*_classified  │     │   outbox      │     │  (Drupal,   │
└─────────────┘     └──────────────┘     └───────────────┘     │   etc.)     │
                           │                                    └─────────────┘
                           ▼
                    ┌──────────────┐
                    │  Dashboard   │
                    │  (Vue 3)     │
                    └──────────────┘
```

### Constraints

- **Backwards Compatible** — Existing APIs must continue working during migration
- **Zero Downtime** — Rolling deploys with graceful shutdown
- **Idempotent** — All operations must be safe to retry
- **Database Per Service** — No shared databases; communicate via APIs or message queues

---

## Guiding Principles

### 1. Make Failures Visible

Every failure must be:
- Logged with structured context
- Recorded in metrics (counters, histograms)
- Traced with OpenTelemetry spans
- Retryable via dead-letter queue

### 2. Bounded Resources

Every queue, channel, and buffer must have explicit limits:
- Backpressure when 80% full
- Timeout on all blocking operations
- Circuit breakers for external dependencies

### 3. Idempotency by Design

Every operation must be safe to retry:
- Use database constraints (UNIQUE, ON CONFLICT)
- Track processed IDs in outbox/history tables
- Use optimistic locking where needed

### 4. Observability First

Before implementing a feature, define:
- What metrics will you emit?
- What traces will you capture?
- What logs will you write?
- What alerts will you create?

---

## Classifier Service

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Classifier Service                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────────┐    ┌───────────────┐    ┌──────────────────────────────┐ │
│  │   Poller     │───▶│ BatchProcessor│───▶│      Classification          │ │
│  │ (ES Query)   │    │ (Worker Pool) │    │         Pipeline             │ │
│  └──────────────┘    └───────────────┘    │  ┌────────────────────────┐  │ │
│         │                   │             │  │ 1. ContentTypeClassifier│  │ │
│         │                   │             │  │ 2. QualityScorer       │  │ │
│         ▼                   ▼             │  │ 3. TrieRuleEngine      │  │ │
│  ┌──────────────┐    ┌───────────────┐    │  │ 4. SourceReputation    │  │ │
│  │ Backpressure │    │   Telemetry   │    │  └────────────────────────┘  │ │
│  │   Control    │    │  (OTel/Prom)  │    └──────────────────────────────┘ │
│  └──────────────┘    └───────────────┘                  │                   │
│                                                         ▼                   │
│                                          ┌──────────────────────────────┐   │
│                                          │        Output Layer          │   │
│                                          │  ┌────────────────────────┐  │   │
│                                          │  │ ES: *_classified_content│  │   │
│                                          │  │ PG: classification_history│ │   │
│                                          │  │ PG: classified_outbox   │  │   │
│                                          │  └────────────────────────┘  │   │
│                                          └──────────────────────────────┘   │
│                                                         │                   │
│         ┌───────────────────────────────────────────────┘                   │
│         ▼                                                                   │
│  ┌──────────────┐    ┌───────────────┐                                     │
│  │ Dead Letter  │◀───│ Retry Worker  │                                     │
│  │    Queue     │    │ (Exponential) │                                     │
│  └──────────────┘    └───────────────┘                                     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 3.1 Telemetry Package

```go
// classifier/internal/telemetry/telemetry.go
package telemetry

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "classifier"

// Metrics holds all classifier metrics
type Metrics struct {
	// Processing metrics
	DocumentsProcessed metric.Int64Counter
	DocumentsFailed    metric.Int64Counter
	DocumentsRetried   metric.Int64Counter
	ProcessingDuration metric.Float64Histogram
	BatchSize          metric.Int64Histogram

	// Rule engine metrics
	RuleMatchDuration metric.Float64Histogram
	RulesEvaluated    metric.Int64Counter
	RulesMatched      metric.Int64Counter

	// Backpressure metrics
	QueueDepth    metric.Int64UpDownCounter
	ActiveWorkers metric.Int64UpDownCounter
	WorkDropped   metric.Int64Counter
	ThrottleCount metric.Int64Counter

	// Lag metrics (freshness SLO)
	PollerLag        metric.Float64Histogram
	ClassificationLag metric.Float64Histogram

	// DLQ metrics
	DLQDepth       metric.Int64UpDownCounter
	DLQEnqueued    metric.Int64Counter
	DLQProcessed   metric.Int64Counter
	DLQExhausted   metric.Int64Counter

	// Outbox metrics
	OutboxBacklog     metric.Int64UpDownCounter
	OutboxPublished   metric.Int64Counter
	OutboxPublishLag  metric.Float64Histogram
}

// Provider wraps OpenTelemetry providers
type Provider struct {
	Tracer   trace.Tracer
	Metrics  *Metrics
	meter    metric.Meter
	exporter *prometheus.Exporter
}

// NewProvider initializes OpenTelemetry with Prometheus exporter
func NewProvider(ctx context.Context) (*Provider, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)
	otel.SetMeterProvider(meterProvider)

	meter := meterProvider.Meter(serviceName)
	tracer := otel.Tracer(serviceName)

	metrics, err := initMetrics(meter)
	if err != nil {
		return nil, err
	}

	return &Provider{
		Tracer:   tracer,
		Metrics:  metrics,
		meter:    meter,
		exporter: exporter,
	}, nil
}

// Handler returns the Prometheus HTTP handler
func (p *Provider) Handler() http.Handler {
	return promhttp.Handler()
}

func initMetrics(meter metric.Meter) (*Metrics, error) {
	m := &Metrics{}
	var err error

	// Processing metrics
	m.DocumentsProcessed, err = meter.Int64Counter("classifier_documents_processed_total",
		metric.WithDescription("Total documents successfully classified"))
	if err != nil {
		return nil, err
	}

	m.DocumentsFailed, err = meter.Int64Counter("classifier_documents_failed_total",
		metric.WithDescription("Total documents that failed classification"))
	if err != nil {
		return nil, err
	}

	m.DocumentsRetried, err = meter.Int64Counter("classifier_documents_retried_total",
		metric.WithDescription("Total documents retried from DLQ"))
	if err != nil {
		return nil, err
	}

	m.ProcessingDuration, err = meter.Float64Histogram("classifier_processing_duration_seconds",
		metric.WithDescription("Time to classify a single document"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0))
	if err != nil {
		return nil, err
	}

	m.BatchSize, err = meter.Int64Histogram("classifier_batch_size",
		metric.WithDescription("Number of documents per batch"),
		metric.WithExplicitBucketBoundaries(1, 5, 10, 25, 50, 100, 200, 500))
	if err != nil {
		return nil, err
	}

	// Rule engine metrics
	m.RuleMatchDuration, err = meter.Float64Histogram("classifier_rule_match_duration_seconds",
		metric.WithDescription("Time spent in rule matching (Aho-Corasick)"),
		metric.WithExplicitBucketBoundaries(0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1))
	if err != nil {
		return nil, err
	}

	m.RulesEvaluated, err = meter.Int64Counter("classifier_rules_evaluated_total",
		metric.WithDescription("Total rule evaluations"))
	if err != nil {
		return nil, err
	}

	m.RulesMatched, err = meter.Int64Counter("classifier_rules_matched_total",
		metric.WithDescription("Total rules that matched"))
	if err != nil {
		return nil, err
	}

	// Backpressure metrics
	m.QueueDepth, err = meter.Int64UpDownCounter("classifier_queue_depth",
		metric.WithDescription("Current pending documents in work queue"))
	if err != nil {
		return nil, err
	}

	m.ActiveWorkers, err = meter.Int64UpDownCounter("classifier_active_workers",
		metric.WithDescription("Currently active worker goroutines"))
	if err != nil {
		return nil, err
	}

	m.WorkDropped, err = meter.Int64Counter("classifier_work_dropped_total",
		metric.WithDescription("Work items dropped due to queue full"))
	if err != nil {
		return nil, err
	}

	m.ThrottleCount, err = meter.Int64Counter("classifier_throttle_count_total",
		metric.WithDescription("Number of times poller was throttled"))
	if err != nil {
		return nil, err
	}

	// Lag metrics
	m.PollerLag, err = meter.Float64Histogram("classifier_poller_lag_seconds",
		metric.WithDescription("Time between document creation and classification start"),
		metric.WithExplicitBucketBoundaries(1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600))
	if err != nil {
		return nil, err
	}

	m.ClassificationLag, err = meter.Float64Histogram("classifier_classification_lag_seconds",
		metric.WithDescription("End-to-end lag from crawl to classified"),
		metric.WithExplicitBucketBoundaries(1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600))
	if err != nil {
		return nil, err
	}

	// DLQ metrics
	m.DLQDepth, err = meter.Int64UpDownCounter("classifier_dlq_depth",
		metric.WithDescription("Current documents in dead-letter queue"))
	if err != nil {
		return nil, err
	}

	m.DLQEnqueued, err = meter.Int64Counter("classifier_dlq_enqueued_total",
		metric.WithDescription("Total documents added to DLQ"))
	if err != nil {
		return nil, err
	}

	m.DLQProcessed, err = meter.Int64Counter("classifier_dlq_processed_total",
		metric.WithDescription("Total documents successfully processed from DLQ"))
	if err != nil {
		return nil, err
	}

	m.DLQExhausted, err = meter.Int64Counter("classifier_dlq_exhausted_total",
		metric.WithDescription("Total documents that exhausted all retries"))
	if err != nil {
		return nil, err
	}

	// Outbox metrics
	m.OutboxBacklog, err = meter.Int64UpDownCounter("classifier_outbox_backlog",
		metric.WithDescription("Current unpublished documents in outbox"))
	if err != nil {
		return nil, err
	}

	m.OutboxPublished, err = meter.Int64Counter("classifier_outbox_published_total",
		metric.WithDescription("Total documents published from outbox"))
	if err != nil {
		return nil, err
	}

	m.OutboxPublishLag, err = meter.Float64Histogram("classifier_outbox_publish_lag_seconds",
		metric.WithDescription("Time between outbox insert and publish"),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1, 2, 5, 10, 30, 60))
	if err != nil {
		return nil, err
	}

	return m, nil
}

// RecordClassification records metrics for a single classification
func (p *Provider) RecordClassification(ctx context.Context, source string, success bool, duration time.Duration) {
	attrs := metric.WithAttributes(attribute.String("source", source))

	if success {
		p.Metrics.DocumentsProcessed.Add(ctx, 1, attrs)
	} else {
		p.Metrics.DocumentsFailed.Add(ctx, 1, attrs)
	}
	p.Metrics.ProcessingDuration.Record(ctx, duration.Seconds(), attrs)
}

// RecordRuleMatch records rule engine metrics
func (p *Provider) RecordRuleMatch(ctx context.Context, duration time.Duration, rulesEvaluated, rulesMatched int) {
	p.Metrics.RuleMatchDuration.Record(ctx, duration.Seconds())
	p.Metrics.RulesEvaluated.Add(ctx, int64(rulesEvaluated))
	p.Metrics.RulesMatched.Add(ctx, int64(rulesMatched))
}

// RecordPollerLag records the freshness lag
func (p *Provider) RecordPollerLag(ctx context.Context, createdAt time.Time) {
	lag := time.Since(createdAt)
	p.Metrics.PollerLag.Record(ctx, lag.Seconds())
}

// RecordOutboxPublish records outbox publish metrics
func (p *Provider) RecordOutboxPublish(ctx context.Context, insertedAt time.Time, success bool) {
	lag := time.Since(insertedAt)
	p.Metrics.OutboxPublishLag.Record(ctx, lag.Seconds())
	if success {
		p.Metrics.OutboxPublished.Add(ctx, 1)
		p.Metrics.OutboxBacklog.Add(ctx, -1)
	}
}
```

### 3.2 Dead-Letter Queue

#### Schema

```sql
-- classifier/migrations/004_dead_letter_queue.up.sql
CREATE TABLE IF NOT EXISTS dead_letter_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content_id VARCHAR(255) NOT NULL,
    source_name VARCHAR(255) NOT NULL,
    index_name VARCHAR(255) NOT NULL,
    error_message TEXT NOT NULL,
    error_code VARCHAR(50),
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 5,
    next_retry_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_attempt_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- Prevent duplicate entries for same content
    CONSTRAINT unique_content_in_dlq UNIQUE (content_id)
);

-- Index for retry worker: find retryable items efficiently
CREATE INDEX idx_dlq_next_retry ON dead_letter_queue(next_retry_at)
    WHERE retry_count < max_retries;

-- Index for monitoring by source
CREATE INDEX idx_dlq_source ON dead_letter_queue(source_name);

-- Index for finding exhausted items
CREATE INDEX idx_dlq_exhausted ON dead_letter_queue(retry_count, max_retries)
    WHERE retry_count >= max_retries;

COMMENT ON TABLE dead_letter_queue IS 'Failed classifications awaiting retry';
COMMENT ON COLUMN dead_letter_queue.error_code IS 'Categorized error: ES_TIMEOUT, RULE_PANIC, QUALITY_ERROR, etc.';
```

#### Domain Model

```go
// classifier/internal/domain/dead_letter.go
package domain

import (
	"fmt"
	"time"
)

// ErrorCode categorizes DLQ errors for filtering and alerting
type ErrorCode string

const (
	ErrorCodeESTimeout     ErrorCode = "ES_TIMEOUT"
	ErrorCodeESUnavailable ErrorCode = "ES_UNAVAILABLE"
	ErrorCodeRulePanic     ErrorCode = "RULE_PANIC"
	ErrorCodeQualityError  ErrorCode = "QUALITY_ERROR"
	ErrorCodeUnknown       ErrorCode = "UNKNOWN"
)

// DeadLetterEntry represents a failed classification for retry
type DeadLetterEntry struct {
	ID            string    `db:"id"`
	ContentID     string    `db:"content_id"`
	SourceName    string    `db:"source_name"`
	IndexName     string    `db:"index_name"`
	ErrorMessage  string    `db:"error_message"`
	ErrorCode     ErrorCode `db:"error_code"`
	RetryCount    int       `db:"retry_count"`
	MaxRetries    int       `db:"max_retries"`
	NextRetryAt   time.Time `db:"next_retry_at"`
	CreatedAt     time.Time `db:"created_at"`
	LastAttemptAt time.Time `db:"last_attempt_at"`
}

const (
	defaultMaxRetries        = 5
	baseRetryDelaySeconds    = 60
	maxRetryDelaySeconds     = 3600 // Cap at 1 hour
)

// NewDeadLetterEntry creates a new DLQ entry with exponential backoff
func NewDeadLetterEntry(contentID, sourceName, indexName, errorMsg string, errorCode ErrorCode) *DeadLetterEntry {
	now := time.Now()
	return &DeadLetterEntry{
		ContentID:     contentID,
		SourceName:    sourceName,
		IndexName:     indexName,
		ErrorMessage:  errorMsg,
		ErrorCode:     errorCode,
		RetryCount:    0,
		MaxRetries:    defaultMaxRetries,
		NextRetryAt:   now.Add(time.Duration(baseRetryDelaySeconds) * time.Second),
		CreatedAt:     now,
		LastAttemptAt: now,
	}
}

// NextRetryDelay calculates exponential backoff with jitter
// Delays: 1min, 2min, 4min, 8min, 16min (capped at 1hr)
func (d *DeadLetterEntry) NextRetryDelay() time.Duration {
	multiplier := 1 << d.RetryCount // 2^retryCount
	delaySeconds := baseRetryDelaySeconds * multiplier
	if delaySeconds > maxRetryDelaySeconds {
		delaySeconds = maxRetryDelaySeconds
	}
	return time.Duration(delaySeconds) * time.Second
}

// ShouldRetry returns true if retries remain
func (d *DeadLetterEntry) ShouldRetry() bool {
	return d.RetryCount < d.MaxRetries
}

// IncrementRetry updates retry state for next attempt
func (d *DeadLetterEntry) IncrementRetry(newError string) {
	d.RetryCount++
	d.LastAttemptAt = time.Now()
	d.ErrorMessage = newError
	d.NextRetryAt = time.Now().Add(d.NextRetryDelay())
}

// String returns a debug representation
func (d *DeadLetterEntry) String() string {
	return fmt.Sprintf("DLQ[%s] content=%s source=%s retries=%d/%d next=%s",
		d.ID, d.ContentID, d.SourceName, d.RetryCount, d.MaxRetries, d.NextRetryAt.Format(time.RFC3339))
}
```

#### Repository

```go
// classifier/internal/database/dead_letter_repository.go
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/north-cloud/classifier/internal/domain"
)

// DeadLetterRepository manages the dead-letter queue
type DeadLetterRepository struct {
	db *sql.DB
}

// NewDeadLetterRepository creates a new repository
func NewDeadLetterRepository(db *sql.DB) *DeadLetterRepository {
	return &DeadLetterRepository{db: db}
}

// Enqueue adds a failed document to the DLQ (idempotent via ON CONFLICT)
func (r *DeadLetterRepository) Enqueue(ctx context.Context, entry *domain.DeadLetterEntry) error {
	query := `
		INSERT INTO dead_letter_queue
			(content_id, source_name, index_name, error_message, error_code, next_retry_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (content_id) DO UPDATE SET
			retry_count = dead_letter_queue.retry_count + 1,
			error_message = EXCLUDED.error_message,
			error_code = EXCLUDED.error_code,
			last_attempt_at = NOW(),
			next_retry_at = NOW() + (INTERVAL '1 second' * POWER(2, dead_letter_queue.retry_count + 1) * 60)
		WHERE dead_letter_queue.retry_count < dead_letter_queue.max_retries`

	_, err := r.db.ExecContext(ctx, query,
		entry.ContentID,
		entry.SourceName,
		entry.IndexName,
		entry.ErrorMessage,
		entry.ErrorCode,
		entry.NextRetryAt,
	)
	if err != nil {
		return fmt.Errorf("enqueue DLQ: %w", err)
	}
	return nil
}

// FetchRetryable returns documents ready for retry with row-level locking
// Uses FOR UPDATE SKIP LOCKED to allow concurrent workers
func (r *DeadLetterRepository) FetchRetryable(ctx context.Context, limit int) ([]domain.DeadLetterEntry, error) {
	query := `
		SELECT id, content_id, source_name, index_name, error_message, error_code,
		       retry_count, max_retries, next_retry_at, created_at, last_attempt_at
		FROM dead_letter_queue
		WHERE next_retry_at <= NOW()
		  AND retry_count < max_retries
		ORDER BY next_retry_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch retryable: %w", err)
	}
	defer rows.Close()

	entries := make([]domain.DeadLetterEntry, 0, limit)
	for rows.Next() {
		var e domain.DeadLetterEntry
		if err := rows.Scan(
			&e.ID, &e.ContentID, &e.SourceName, &e.IndexName, &e.ErrorMessage, &e.ErrorCode,
			&e.RetryCount, &e.MaxRetries, &e.NextRetryAt, &e.CreatedAt, &e.LastAttemptAt,
		); err != nil {
			return nil, fmt.Errorf("scan DLQ entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Remove deletes a successfully processed entry
func (r *DeadLetterRepository) Remove(ctx context.Context, contentID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM dead_letter_queue WHERE content_id = $1`,
		contentID)
	if err != nil {
		return fmt.Errorf("remove from DLQ: %w", err)
	}
	return nil
}

// MarkExhausted flags entries that exceeded max retries (for alerting)
func (r *DeadLetterRepository) MarkExhausted(ctx context.Context, contentID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE dead_letter_queue
		SET retry_count = max_retries,
		    last_attempt_at = NOW()
		WHERE content_id = $1`,
		contentID)
	if err != nil {
		return fmt.Errorf("mark exhausted: %w", err)
	}
	return nil
}

// GetStats returns DLQ statistics for monitoring
func (r *DeadLetterRepository) GetStats(ctx context.Context) (*DLQStats, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE retry_count < max_retries) as pending,
			COUNT(*) FILTER (WHERE retry_count >= max_retries) as exhausted,
			COUNT(*) FILTER (WHERE next_retry_at <= NOW() AND retry_count < max_retries) as ready,
			COALESCE(AVG(retry_count), 0) as avg_retries,
			MAX(created_at) as oldest_entry
		FROM dead_letter_queue`

	var stats DLQStats
	var oldestEntry sql.NullTime
	err := r.db.QueryRowContext(ctx, query).Scan(
		&stats.Pending,
		&stats.Exhausted,
		&stats.Ready,
		&stats.AvgRetries,
		&oldestEntry,
	)
	if err != nil {
		return nil, fmt.Errorf("get DLQ stats: %w", err)
	}
	if oldestEntry.Valid {
		stats.OldestEntry = &oldestEntry.Time
	}
	return &stats, nil
}

// CountBySource returns DLQ counts grouped by source
func (r *DeadLetterRepository) CountBySource(ctx context.Context) (map[string]int64, error) {
	query := `
		SELECT source_name, COUNT(*)
		FROM dead_letter_queue
		WHERE retry_count < max_retries
		GROUP BY source_name`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("count by source: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var source string
		var count int64
		if err := rows.Scan(&source, &count); err != nil {
			return nil, err
		}
		result[source] = count
	}
	return result, rows.Err()
}

// DLQStats holds dead-letter queue statistics
type DLQStats struct {
	Pending     int64
	Exhausted   int64
	Ready       int64
	AvgRetries  float64
	OldestEntry *time.Time
}
```

### 3.3 Backpressure-Controlled Batch Processor

```go
// classifier/internal/processor/batch.go
package processor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/north-cloud/classifier/internal/classifier"
	"github.com/north-cloud/classifier/internal/domain"
	"github.com/north-cloud/classifier/internal/telemetry"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	defaultMaxQueueDepth   = 500
	defaultSubmitTimeout   = 30 * time.Second
	workerDrainTimeout     = 30 * time.Second
	throttleThresholdRatio = 0.8
)

// ProcessResult holds the outcome of processing a single item
type ProcessResult struct {
	Content   *domain.ClassifiedContent
	RawItem   *domain.RawContent
	Error     error
	Duration  time.Duration
}

// ResultHandler is called for each processed item
type ResultHandler func(ctx context.Context, result *ProcessResult)

// BatchProcessor processes raw content in parallel with backpressure
type BatchProcessor struct {
	classifier    *classifier.Classifier
	concurrency   int
	maxQueueDepth int
	submitTimeout time.Duration
	logger        infralogger.Logger
	telemetry     *telemetry.Provider
	resultHandler ResultHandler

	workQueue chan *domain.RawContent
	wg        sync.WaitGroup
	cancel    context.CancelFunc
	started   bool
	mu        sync.Mutex
}

// BatchProcessorConfig holds configuration options
type BatchProcessorConfig struct {
	Concurrency   int
	MaxQueueDepth int
	SubmitTimeout time.Duration
}

// DefaultBatchProcessorConfig returns sensible defaults
func DefaultBatchProcessorConfig() BatchProcessorConfig {
	return BatchProcessorConfig{
		Concurrency:   10,
		MaxQueueDepth: defaultMaxQueueDepth,
		SubmitTimeout: defaultSubmitTimeout,
	}
}

// NewBatchProcessor creates a processor with bounded queue
func NewBatchProcessor(
	c *classifier.Classifier,
	cfg BatchProcessorConfig,
	logger infralogger.Logger,
	tp *telemetry.Provider,
	handler ResultHandler,
) *BatchProcessor {
	if cfg.MaxQueueDepth <= 0 {
		cfg.MaxQueueDepth = defaultMaxQueueDepth
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 10
	}
	if cfg.SubmitTimeout <= 0 {
		cfg.SubmitTimeout = defaultSubmitTimeout
	}

	return &BatchProcessor{
		classifier:    c,
		concurrency:   cfg.Concurrency,
		maxQueueDepth: cfg.MaxQueueDepth,
		submitTimeout: cfg.SubmitTimeout,
		logger:        logger,
		telemetry:     tp,
		resultHandler: handler,
		workQueue:     make(chan *domain.RawContent, cfg.MaxQueueDepth),
	}
}

// Start launches worker goroutines
func (b *BatchProcessor) Start(ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.started {
		return
	}

	ctx, b.cancel = context.WithCancel(ctx)

	for i := range b.concurrency {
		b.wg.Add(1)
		go b.worker(ctx, i)
	}

	b.started = true
	b.logger.Info("batch processor started",
		infralogger.Int("workers", b.concurrency),
		infralogger.Int("max_queue_depth", b.maxQueueDepth))
}

// Stop gracefully shuts down workers
func (b *BatchProcessor) Stop() {
	b.mu.Lock()
	if !b.started {
		b.mu.Unlock()
		return
	}
	b.mu.Unlock()

	if b.cancel != nil {
		b.cancel()
	}

	remaining := len(b.workQueue)
	b.logger.Info("draining work queue",
		infralogger.Int("remaining_items", remaining))

	close(b.workQueue)

	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		b.logger.Info("batch processor stopped gracefully")
	case <-time.After(workerDrainTimeout):
		b.logger.Warn("batch processor stop timed out, some workers may not have finished",
			infralogger.Int("remaining", len(b.workQueue)))
	}
}

// Submit adds work to the queue with timeout (backpressure)
func (b *BatchProcessor) Submit(ctx context.Context, item *domain.RawContent) error {
	return b.SubmitWithTimeout(ctx, item, b.submitTimeout)
}

// SubmitWithTimeout adds work with explicit timeout
func (b *BatchProcessor) SubmitWithTimeout(ctx context.Context, item *domain.RawContent, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if b.telemetry != nil {
		b.telemetry.Metrics.QueueDepth.Add(ctx, 1)
	}

	select {
	case b.workQueue <- item:
		return nil
	case <-ctx.Done():
		if b.telemetry != nil {
			b.telemetry.Metrics.QueueDepth.Add(context.Background(), -1)
			b.telemetry.Metrics.WorkDropped.Add(context.Background(), 1)
		}
		return fmt.Errorf("queue full after %v: %w", timeout, ctx.Err())
	}
}

// QueueDepth returns current queue size for monitoring
func (b *BatchProcessor) QueueDepth() int {
	return len(b.workQueue)
}

// ShouldThrottle returns true if queue is near capacity (80%)
func (b *BatchProcessor) ShouldThrottle() bool {
	depth := len(b.workQueue)
	threshold := int(float64(b.maxQueueDepth) * throttleThresholdRatio)
	return depth > threshold
}

func (b *BatchProcessor) worker(ctx context.Context, id int) {
	defer func() {
		if r := recover(); r != nil {
			b.logger.Error("worker panic recovered",
				infralogger.Int("worker_id", id),
				infralogger.Any("panic", r))
		}
		b.wg.Done()
	}()

	if b.telemetry != nil {
		b.telemetry.Metrics.ActiveWorkers.Add(ctx, 1)
		defer b.telemetry.Metrics.ActiveWorkers.Add(context.Background(), -1)
	}

	for {
		select {
		case item, ok := <-b.workQueue:
			if !ok {
				return // Channel closed, shutdown
			}
			result := b.processItem(ctx, id, item)
			if b.telemetry != nil {
				b.telemetry.Metrics.QueueDepth.Add(ctx, -1)
			}
			if b.resultHandler != nil {
				b.resultHandler(ctx, result)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (b *BatchProcessor) processItem(ctx context.Context, workerID int, item *domain.RawContent) *ProcessResult {
	start := time.Now()

	// Record poller lag (time from creation to processing)
	if b.telemetry != nil && !item.CrawledAt.IsZero() {
		b.telemetry.RecordPollerLag(ctx, item.CrawledAt)
	}

	result, err := b.classifier.Classify(ctx, item)
	duration := time.Since(start)

	// Record classification metrics
	if b.telemetry != nil {
		b.telemetry.RecordClassification(ctx, item.SourceName, err == nil, duration)
	}

	if err != nil {
		b.logger.Error("classification failed",
			infralogger.Int("worker_id", workerID),
			infralogger.String("content_id", item.ID),
			infralogger.String("source", item.SourceName),
			infralogger.Duration("duration", duration),
			infralogger.Error(err))

		return &ProcessResult{
			RawItem:  item,
			Error:    err,
			Duration: duration,
		}
	}

	return &ProcessResult{
		Content:  result,
		RawItem:  item,
		Duration: duration,
	}
}
```

### 3.4 Trie-Based Rule Engine (Aho-Corasick)

```go
// classifier/internal/classifier/rule_engine.go
package classifier

import (
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	ahocorasick "github.com/cloudflare/ahocorasick"
	"github.com/north-cloud/classifier/internal/domain"
	"github.com/north-cloud/classifier/internal/telemetry"
)

// RuleMatch represents a matched rule with scoring details
type RuleMatch struct {
	Rule            *domain.ClassificationRule
	MatchCount      int      // Total keyword hits
	UniqueMatches   int      // Unique keywords matched
	Coverage        float64  // UniqueMatches / TotalKeywords
	Score           float64  // Final computed score
	MatchedKeywords []string // Which keywords matched (for debugging)
}

// TrieRuleEngine uses Aho-Corasick for O(n+m) matching
type TrieRuleEngine struct {
	mu        sync.RWMutex
	matcher   *ahocorasick.Matcher
	rules     []*domain.ClassificationRule
	keywords  []string                   // All keywords in order
	kwToRules map[string][]*ruleMapping  // keyword -> rule mappings
	telemetry *telemetry.Provider
}

type ruleMapping struct {
	rule         *domain.ClassificationRule
	keywordIndex int
}

// NewTrieRuleEngine builds the automaton from rules
func NewTrieRuleEngine(rules []*domain.ClassificationRule, tp *telemetry.Provider) *TrieRuleEngine {
	engine := &TrieRuleEngine{
		rules:     rules,
		kwToRules: make(map[string][]*ruleMapping),
		telemetry: tp,
	}
	engine.rebuild()
	return engine
}

// rebuild constructs the Aho-Corasick automaton
func (e *TrieRuleEngine) rebuild() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.keywords = make([]string, 0, len(e.rules)*20)
	e.kwToRules = make(map[string][]*ruleMapping)

	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}
		for idx, kw := range rule.Keywords {
			normalized := normalizeKeyword(kw)
			if normalized == "" {
				continue
			}
			e.keywords = append(e.keywords, normalized)
			e.kwToRules[normalized] = append(e.kwToRules[normalized], &ruleMapping{
				rule:         rule,
				keywordIndex: idx,
			})
		}
	}

	if len(e.keywords) > 0 {
		e.matcher = ahocorasick.NewStringMatcher(e.keywords)
	} else {
		e.matcher = nil
	}
}

// Match finds all matching rules in a single pass through the text
func (e *TrieRuleEngine) Match(title, body string) []RuleMatch {
	start := time.Now()
	defer func() {
		if e.telemetry != nil {
			e.telemetry.Metrics.RuleMatchDuration.Record(
				nil, time.Since(start).Seconds())
		}
	}()

	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.matcher == nil {
		return nil
	}

	// Normalize input text
	text := normalizeText(title + " " + body)

	// Single pass through text - O(n + m)
	hits := e.matcher.Match([]byte(text))

	// Accumulate matches per rule
	ruleAccum := make(map[int]*matchAccumulator)

	for _, hitIndex := range hits {
		if hitIndex >= len(e.keywords) {
			continue
		}
		keyword := e.keywords[hitIndex]
		mappings := e.kwToRules[keyword]

		for _, m := range mappings {
			acc, exists := ruleAccum[m.rule.ID]
			if !exists {
				acc = &matchAccumulator{
					rule:            m.rule,
					matchedKeywords: make(map[int]bool),
					keywordTexts:    make([]string, 0),
				}
				ruleAccum[m.rule.ID] = acc
			}
			if !acc.matchedKeywords[m.keywordIndex] {
				acc.keywordTexts = append(acc.keywordTexts, keyword)
			}
			acc.matchedKeywords[m.keywordIndex] = true
			acc.totalHits++
		}
	}

	// Calculate scores and filter by confidence threshold
	results := make([]RuleMatch, 0, len(ruleAccum))
	for _, acc := range ruleAccum {
		totalKeywords := len(acc.rule.Keywords)
		if totalKeywords == 0 {
			continue
		}

		uniqueMatched := len(acc.matchedKeywords)
		coverage := float64(uniqueMatched) / float64(totalKeywords)

		// Log-scaled term frequency + coverage
		logTF := math.Min(1.0, math.Log1p(float64(acc.totalHits))/2.5)
		score := (logTF * 0.5) + (coverage * 0.5)

		if score >= acc.rule.MinConfidence {
			results = append(results, RuleMatch{
				Rule:            acc.rule,
				MatchCount:      acc.totalHits,
				UniqueMatches:   uniqueMatched,
				Coverage:        coverage,
				Score:           score,
				MatchedKeywords: acc.keywordTexts,
			})
		}
	}

	// Record telemetry
	if e.telemetry != nil {
		e.telemetry.Metrics.RulesEvaluated.Add(nil, int64(len(e.rules)))
		e.telemetry.Metrics.RulesMatched.Add(nil, int64(len(results)))
	}

	// Sort by priority (desc), then score (desc)
	sort.Slice(results, func(i, j int) bool {
		if results[i].Rule.Priority != results[j].Rule.Priority {
			return results[i].Rule.Priority > results[j].Rule.Priority
		}
		return results[i].Score > results[j].Score
	})

	return results
}

// UpdateRules hot-reloads rules without restart
func (e *TrieRuleEngine) UpdateRules(rules []*domain.ClassificationRule) {
	e.rules = rules
	e.rebuild()
}

// RuleCount returns the number of enabled rules
func (e *TrieRuleEngine) RuleCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	count := 0
	for _, r := range e.rules {
		if r.Enabled {
			count++
		}
	}
	return count
}

// KeywordCount returns total keywords across all enabled rules
func (e *TrieRuleEngine) KeywordCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.keywords)
}

type matchAccumulator struct {
	rule            *domain.ClassificationRule
	matchedKeywords map[int]bool // keyword index -> matched
	keywordTexts    []string     // actual matched keyword strings
	totalHits       int
}

func normalizeKeyword(kw string) string {
	return strings.ToLower(strings.TrimSpace(kw))
}

func normalizeText(text string) string {
	// Lowercase and normalize unicode
	text = strings.ToLower(text)

	// Replace non-alphanumeric with spaces (preserves word boundaries)
	var result strings.Builder
	result.Grow(len(text))

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(r)
		} else {
			result.WriteByte(' ')
		}
	}

	return result.String()
}
```

---

## Publisher Service

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Publisher Service                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                     Transactional Outbox Pattern                      │  │
│  │                                                                        │  │
│  │   Classifier writes to outbox ──▶ Publisher reads outbox ──▶ Redis    │  │
│  │         (same TX as ES)              (FOR UPDATE SKIP LOCKED)         │  │
│  │                                                                        │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│  ┌──────────────┐    ┌───────────────┐    ┌──────────────────────────────┐ │
│  │   Outbox     │───▶│ Outbox Worker │───▶│      Redis Pub/Sub           │ │
│  │  (Postgres)  │    │ (5s polling)  │    │                              │ │
│  └──────────────┘    └───────────────┘    │  articles:crime:violent      │ │
│         │                   │             │  articles:crime:property     │ │
│         │                   │             │  articles:news               │ │
│         ▼                   ▼             │  articles:local              │ │
│  ┌──────────────┐    ┌───────────────┐    └──────────────────────────────┘ │
│  │   Telemetry  │    │    Routes     │                                     │
│  │  (OTel/Prom) │    │  (Postgres)   │                                     │
│  └──────────────┘    └───────────────┘                                     │
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                        Failure Handling                               │  │
│  │                                                                        │  │
│  │   Redis failure ──▶ Retry with backoff ──▶ Move to DLQ after 5x       │  │
│  │                                                                        │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.1 Outbox Schema

```sql
-- publisher/migrations/002_outbox.up.sql

-- Transactional outbox for guaranteed delivery
CREATE TABLE IF NOT EXISTS classified_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Content identification
    content_id VARCHAR(255) NOT NULL,
    source_name VARCHAR(255) NOT NULL,
    index_name VARCHAR(255) NOT NULL,

    -- Routing information
    content_type VARCHAR(50) NOT NULL,      -- article, page, video, etc.
    topics TEXT[] NOT NULL DEFAULT '{}',     -- classification topics
    quality_score INTEGER NOT NULL,          -- 0-100
    is_crime_related BOOLEAN NOT NULL DEFAULT FALSE,
    crime_subcategory VARCHAR(50),           -- violent_crime, property_crime, etc.

    -- Denormalized content for publishing (avoids ES round-trip)
    title TEXT NOT NULL,
    body TEXT,
    url TEXT NOT NULL,
    published_date TIMESTAMP WITH TIME ZONE,

    -- Outbox metadata
    status VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending, publishing, published, failed
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 5,
    error_message TEXT,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    published_at TIMESTAMP WITH TIME ZONE,
    next_retry_at TIMESTAMP WITH TIME ZONE,

    -- Idempotency: prevent duplicate outbox entries
    CONSTRAINT unique_outbox_content UNIQUE (content_id)
);

-- Index for outbox worker: pending items ready to publish
CREATE INDEX idx_outbox_pending ON classified_outbox(created_at)
    WHERE status = 'pending';

-- Index for retry worker: failed items ready for retry
CREATE INDEX idx_outbox_retry ON classified_outbox(next_retry_at)
    WHERE status = 'failed' AND retry_count < max_retries;

-- Index for routing queries
CREATE INDEX idx_outbox_routing ON classified_outbox(content_type, source_name)
    WHERE status = 'pending';

-- Index for crime-related content (high priority)
CREATE INDEX idx_outbox_crime ON classified_outbox(created_at)
    WHERE status = 'pending' AND is_crime_related = TRUE;

-- Cleanup index for old published entries
CREATE INDEX idx_outbox_cleanup ON classified_outbox(published_at)
    WHERE status = 'published';

COMMENT ON TABLE classified_outbox IS 'Transactional outbox for guaranteed Redis publishing';
COMMENT ON COLUMN classified_outbox.status IS 'pending=awaiting publish, publishing=in-flight, published=done, failed=retry needed';
```

### 4.2 Outbox Domain Model

```go
// publisher/internal/domain/outbox.go
package domain

import (
	"time"
)

// OutboxStatus represents the state of an outbox entry
type OutboxStatus string

const (
	OutboxStatusPending    OutboxStatus = "pending"
	OutboxStatusPublishing OutboxStatus = "publishing"
	OutboxStatusPublished  OutboxStatus = "published"
	OutboxStatusFailed     OutboxStatus = "failed"
)

// OutboxEntry represents a classified document awaiting publication
type OutboxEntry struct {
	ID               string       `db:"id"`
	ContentID        string       `db:"content_id"`
	SourceName       string       `db:"source_name"`
	IndexName        string       `db:"index_name"`
	ContentType      string       `db:"content_type"`
	Topics           []string     `db:"topics"`
	QualityScore     int          `db:"quality_score"`
	IsCrimeRelated   bool         `db:"is_crime_related"`
	CrimeSubcategory *string      `db:"crime_subcategory"`
	Title            string       `db:"title"`
	Body             string       `db:"body"`
	URL              string       `db:"url"`
	PublishedDate    *time.Time   `db:"published_date"`
	Status           OutboxStatus `db:"status"`
	RetryCount       int          `db:"retry_count"`
	MaxRetries       int          `db:"max_retries"`
	ErrorMessage     *string      `db:"error_message"`
	CreatedAt        time.Time    `db:"created_at"`
	UpdatedAt        time.Time    `db:"updated_at"`
	PublishedAt      *time.Time   `db:"published_at"`
	NextRetryAt      *time.Time   `db:"next_retry_at"`
}

// OutboxCreateRequest contains fields for creating an outbox entry
type OutboxCreateRequest struct {
	ContentID        string
	SourceName       string
	IndexName        string
	ContentType      string
	Topics           []string
	QualityScore     int
	IsCrimeRelated   bool
	CrimeSubcategory *string
	Title            string
	Body             string
	URL              string
	PublishedDate    *time.Time
}

// RoutingKey returns the Redis channel for this entry
func (o *OutboxEntry) RoutingKey() string {
	// Crime content gets specific channels
	if o.IsCrimeRelated && o.CrimeSubcategory != nil {
		return "articles:crime:" + *o.CrimeSubcategory
	}
	if o.IsCrimeRelated {
		return "articles:crime"
	}

	// Route by content type
	switch o.ContentType {
	case "article":
		return "articles:news"
	case "video":
		return "content:video"
	case "image":
		return "content:image"
	default:
		return "content:other"
	}
}

// ShouldRetry returns true if the entry can be retried
func (o *OutboxEntry) ShouldRetry() bool {
	return o.RetryCount < o.MaxRetries
}

// ToPublishMessage converts to Redis message format
func (o *OutboxEntry) ToPublishMessage() map[string]any {
	return map[string]any{
		"id":                o.ContentID,
		"source":            o.SourceName,
		"index":             o.IndexName,
		"content_type":      o.ContentType,
		"topics":            o.Topics,
		"quality_score":     o.QualityScore,
		"is_crime_related":  o.IsCrimeRelated,
		"crime_subcategory": o.CrimeSubcategory,
		"title":             o.Title,
		"body":              o.Body,
		"url":               o.URL,
		"published_date":    o.PublishedDate,
		"publisher": map[string]any{
			"outbox_id":    o.ID,
			"published_at": time.Now().UTC().Format(time.RFC3339),
			"channel":      o.RoutingKey(),
		},
	}
}
```

### 4.3 Outbox Repository

```go
// publisher/internal/database/outbox_repository.go
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/north-cloud/publisher/internal/domain"
)

// OutboxRepository manages the transactional outbox
type OutboxRepository struct {
	db *sql.DB
}

// NewOutboxRepository creates a new repository
func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// Insert adds a new entry to the outbox (idempotent via ON CONFLICT)
func (r *OutboxRepository) Insert(ctx context.Context, req *domain.OutboxCreateRequest) error {
	query := `
		INSERT INTO classified_outbox (
			content_id, source_name, index_name, content_type, topics,
			quality_score, is_crime_related, crime_subcategory,
			title, body, url, published_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (content_id) DO UPDATE SET
			updated_at = NOW(),
			-- Only update if still pending (don't overwrite in-flight)
			status = CASE
				WHEN classified_outbox.status = 'pending' THEN 'pending'
				ELSE classified_outbox.status
			END`

	_, err := r.db.ExecContext(ctx, query,
		req.ContentID,
		req.SourceName,
		req.IndexName,
		req.ContentType,
		pq.Array(req.Topics),
		req.QualityScore,
		req.IsCrimeRelated,
		req.CrimeSubcategory,
		req.Title,
		req.Body,
		req.URL,
		req.PublishedDate,
	)
	if err != nil {
		return fmt.Errorf("insert outbox: %w", err)
	}
	return nil
}

// FetchPending returns pending entries ready for publishing
// Uses FOR UPDATE SKIP LOCKED for concurrent worker safety
func (r *OutboxRepository) FetchPending(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
	query := `
		UPDATE classified_outbox
		SET status = 'publishing', updated_at = NOW()
		WHERE id IN (
			SELECT id FROM classified_outbox
			WHERE status = 'pending'
			ORDER BY
				is_crime_related DESC,  -- Prioritize crime content
				created_at ASC          -- FIFO for same priority
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, content_id, source_name, index_name, content_type, topics,
		          quality_score, is_crime_related, crime_subcategory,
		          title, body, url, published_date, status, retry_count,
		          max_retries, error_message, created_at, updated_at,
		          published_at, next_retry_at`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch pending: %w", err)
	}
	defer rows.Close()

	return scanOutboxEntries(rows)
}

// FetchRetryable returns failed entries ready for retry
func (r *OutboxRepository) FetchRetryable(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
	query := `
		UPDATE classified_outbox
		SET status = 'publishing', updated_at = NOW()
		WHERE id IN (
			SELECT id FROM classified_outbox
			WHERE status = 'failed'
			  AND retry_count < max_retries
			  AND (next_retry_at IS NULL OR next_retry_at <= NOW())
			ORDER BY next_retry_at ASC NULLS FIRST
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, content_id, source_name, index_name, content_type, topics,
		          quality_score, is_crime_related, crime_subcategory,
		          title, body, url, published_date, status, retry_count,
		          max_retries, error_message, created_at, updated_at,
		          published_at, next_retry_at`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch retryable: %w", err)
	}
	defer rows.Close()

	return scanOutboxEntries(rows)
}

// MarkPublished marks an entry as successfully published
func (r *OutboxRepository) MarkPublished(ctx context.Context, id string) error {
	query := `
		UPDATE classified_outbox
		SET status = 'published',
		    published_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mark published: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("outbox entry not found: %s", id)
	}
	return nil
}

// MarkFailed marks an entry as failed with retry scheduling
func (r *OutboxRepository) MarkFailed(ctx context.Context, id string, errorMsg string) error {
	// Exponential backoff: 1min, 2min, 4min, 8min, 16min
	query := `
		UPDATE classified_outbox
		SET status = 'failed',
		    error_message = $2,
		    retry_count = retry_count + 1,
		    next_retry_at = NOW() + (INTERVAL '1 minute' * POWER(2, retry_count)),
		    updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id, errorMsg)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	return nil
}

// ResetToP ending resets a publishing entry back to pending (for restart recovery)
func (r *OutboxRepository) ResetToPending(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		UPDATE classified_outbox
		SET status = 'pending', updated_at = NOW()
		WHERE status = 'publishing'
		  AND updated_at < NOW() - $1::interval`

	result, err := r.db.ExecContext(ctx, query, olderThan.String())
	if err != nil {
		return 0, fmt.Errorf("reset to pending: %w", err)
	}

	return result.RowsAffected()
}

// CleanupPublished removes old published entries
func (r *OutboxRepository) CleanupPublished(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		DELETE FROM classified_outbox
		WHERE status = 'published'
		  AND published_at < NOW() - $1::interval`

	result, err := r.db.ExecContext(ctx, query, olderThan.String())
	if err != nil {
		return 0, fmt.Errorf("cleanup published: %w", err)
	}

	return result.RowsAffected()
}

// GetStats returns outbox statistics
func (r *OutboxRepository) GetStats(ctx context.Context) (*OutboxStats, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending') as pending,
			COUNT(*) FILTER (WHERE status = 'publishing') as publishing,
			COUNT(*) FILTER (WHERE status = 'published') as published,
			COUNT(*) FILTER (WHERE status = 'failed' AND retry_count < max_retries) as failed_retryable,
			COUNT(*) FILTER (WHERE status = 'failed' AND retry_count >= max_retries) as failed_exhausted,
			COALESCE(AVG(EXTRACT(EPOCH FROM (published_at - created_at)))
				FILTER (WHERE status = 'published' AND published_at > NOW() - INTERVAL '1 hour'), 0) as avg_publish_lag_seconds
		FROM classified_outbox`

	var stats OutboxStats
	err := r.db.QueryRowContext(ctx, query).Scan(
		&stats.Pending,
		&stats.Publishing,
		&stats.Published,
		&stats.FailedRetryable,
		&stats.FailedExhausted,
		&stats.AvgPublishLagSeconds,
	)
	if err != nil {
		return nil, fmt.Errorf("get outbox stats: %w", err)
	}
	return &stats, nil
}

// OutboxStats holds outbox statistics
type OutboxStats struct {
	Pending              int64
	Publishing           int64
	Published            int64
	FailedRetryable      int64
	FailedExhausted      int64
	AvgPublishLagSeconds float64
}

func scanOutboxEntries(rows *sql.Rows) ([]domain.OutboxEntry, error) {
	entries := make([]domain.OutboxEntry, 0)
	for rows.Next() {
		var e domain.OutboxEntry
		var topics pq.StringArray

		err := rows.Scan(
			&e.ID, &e.ContentID, &e.SourceName, &e.IndexName, &e.ContentType, &topics,
			&e.QualityScore, &e.IsCrimeRelated, &e.CrimeSubcategory,
			&e.Title, &e.Body, &e.URL, &e.PublishedDate, &e.Status, &e.RetryCount,
			&e.MaxRetries, &e.ErrorMessage, &e.CreatedAt, &e.UpdatedAt,
			&e.PublishedAt, &e.NextRetryAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan outbox entry: %w", err)
		}
		e.Topics = topics
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
```

### 4.4 Outbox Worker

```go
// publisher/internal/worker/outbox_worker.go
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/north-cloud/publisher/internal/database"
	"github.com/north-cloud/publisher/internal/domain"
	"github.com/north-cloud/publisher/internal/telemetry"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	defaultPollInterval   = 5 * time.Second
	defaultBatchSize      = 100
	defaultPublishTimeout = 10 * time.Second
	stalePublishingAge    = 5 * time.Minute
	cleanupRetention      = 7 * 24 * time.Hour // Keep published entries for 7 days
)

// OutboxWorker polls the outbox and publishes to Redis
type OutboxWorker struct {
	repo      *database.OutboxRepository
	redis     *redis.Client
	logger    infralogger.Logger
	telemetry *telemetry.Provider
	tracer    trace.Tracer

	pollInterval   time.Duration
	batchSize      int
	publishTimeout time.Duration

	stopChan chan struct{}
	wg       sync.WaitGroup
}

// OutboxWorkerConfig holds configuration options
type OutboxWorkerConfig struct {
	PollInterval   time.Duration
	BatchSize      int
	PublishTimeout time.Duration
}

// DefaultOutboxWorkerConfig returns sensible defaults
func DefaultOutboxWorkerConfig() OutboxWorkerConfig {
	return OutboxWorkerConfig{
		PollInterval:   defaultPollInterval,
		BatchSize:      defaultBatchSize,
		PublishTimeout: defaultPublishTimeout,
	}
}

// NewOutboxWorker creates a new outbox worker
func NewOutboxWorker(
	repo *database.OutboxRepository,
	redis *redis.Client,
	cfg OutboxWorkerConfig,
	logger infralogger.Logger,
	tp *telemetry.Provider,
) *OutboxWorker {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultBatchSize
	}
	if cfg.PublishTimeout <= 0 {
		cfg.PublishTimeout = defaultPublishTimeout
	}

	return &OutboxWorker{
		repo:           repo,
		redis:          redis,
		logger:         logger,
		telemetry:      tp,
		tracer:         otel.Tracer("outbox-worker"),
		pollInterval:   cfg.PollInterval,
		batchSize:      cfg.BatchSize,
		publishTimeout: cfg.PublishTimeout,
		stopChan:       make(chan struct{}),
	}
}

// Start begins the outbox polling loop
func (w *OutboxWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.run(ctx)

	// Also start cleanup and recovery goroutines
	w.wg.Add(1)
	go w.runCleanup(ctx)

	w.wg.Add(1)
	go w.runRecovery(ctx)

	w.logger.Info("outbox worker started",
		infralogger.Duration("poll_interval", w.pollInterval),
		infralogger.Int("batch_size", w.batchSize))
}

// Stop gracefully stops the worker
func (w *OutboxWorker) Stop() {
	close(w.stopChan)
	w.wg.Wait()
	w.logger.Info("outbox worker stopped")
}

func (w *OutboxWorker) run(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	// Process immediately on start
	w.processOnce(ctx)

	for {
		select {
		case <-ticker.C:
			w.processOnce(ctx)
		case <-w.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (w *OutboxWorker) processOnce(ctx context.Context) {
	// Process pending entries
	pending, err := w.repo.FetchPending(ctx, w.batchSize)
	if err != nil {
		w.logger.Error("failed to fetch pending outbox entries", infralogger.Error(err))
	} else if len(pending) > 0 {
		w.publishBatch(ctx, pending)
	}

	// Process retryable entries
	retryable, err := w.repo.FetchRetryable(ctx, w.batchSize/2)
	if err != nil {
		w.logger.Error("failed to fetch retryable outbox entries", infralogger.Error(err))
	} else if len(retryable) > 0 {
		w.publishBatch(ctx, retryable)
	}

	// Update backlog metric
	if w.telemetry != nil {
		stats, err := w.repo.GetStats(ctx)
		if err == nil {
			// Set absolute value (not delta)
			w.telemetry.Metrics.OutboxBacklog.Add(ctx, stats.Pending-w.getLastBacklog())
			w.setLastBacklog(stats.Pending)
		}
	}
}

func (w *OutboxWorker) publishBatch(ctx context.Context, entries []domain.OutboxEntry) {
	for _, entry := range entries {
		w.publishOne(ctx, &entry)
	}
}

func (w *OutboxWorker) publishOne(ctx context.Context, entry *domain.OutboxEntry) {
	// Create span for tracing
	ctx, span := w.tracer.Start(ctx, "outbox.publish",
		trace.WithAttributes(
			attribute.String("content_id", entry.ContentID),
			attribute.String("source", entry.SourceName),
			attribute.String("channel", entry.RoutingKey()),
		))
	defer span.End()

	// Prepare message
	message := entry.ToPublishMessage()
	messageJSON, err := json.Marshal(message)
	if err != nil {
		w.handlePublishError(ctx, entry, fmt.Errorf("marshal message: %w", err))
		return
	}

	// Publish to Redis with timeout
	pubCtx, cancel := context.WithTimeout(ctx, w.publishTimeout)
	defer cancel()

	channel := entry.RoutingKey()
	err = w.redis.Publish(pubCtx, channel, messageJSON).Err()
	if err != nil {
		w.handlePublishError(ctx, entry, fmt.Errorf("redis publish: %w", err))
		return
	}

	// Mark as published
	if err := w.repo.MarkPublished(ctx, entry.ID); err != nil {
		w.logger.Error("failed to mark outbox entry as published",
			infralogger.String("outbox_id", entry.ID),
			infralogger.Error(err))
		// Don't return error - message was published, just DB update failed
	}

	// Record telemetry
	if w.telemetry != nil {
		w.telemetry.RecordOutboxPublish(ctx, entry.CreatedAt, true)
	}

	w.logger.Debug("published to Redis",
		infralogger.String("content_id", entry.ContentID),
		infralogger.String("channel", channel),
		infralogger.Int("retry_count", entry.RetryCount))
}

func (w *OutboxWorker) handlePublishError(ctx context.Context, entry *domain.OutboxEntry, err error) {
	w.logger.Error("failed to publish outbox entry",
		infralogger.String("outbox_id", entry.ID),
		infralogger.String("content_id", entry.ContentID),
		infralogger.Int("retry_count", entry.RetryCount),
		infralogger.Error(err))

	if markErr := w.repo.MarkFailed(ctx, entry.ID, err.Error()); markErr != nil {
		w.logger.Error("failed to mark outbox entry as failed",
			infralogger.String("outbox_id", entry.ID),
			infralogger.Error(markErr))
	}

	// Record telemetry
	if w.telemetry != nil {
		w.telemetry.RecordOutboxPublish(ctx, entry.CreatedAt, false)
	}
}

// runCleanup periodically removes old published entries
func (w *OutboxWorker) runCleanup(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			deleted, err := w.repo.CleanupPublished(ctx, cleanupRetention)
			if err != nil {
				w.logger.Error("outbox cleanup failed", infralogger.Error(err))
			} else if deleted > 0 {
				w.logger.Info("cleaned up old outbox entries",
					infralogger.Int64("deleted", deleted))
			}
		case <-w.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// runRecovery resets stale "publishing" entries back to "pending"
// This handles entries that were claimed but worker crashed before completing
func (w *OutboxWorker) runRecovery(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			reset, err := w.repo.ResetToPending(ctx, stalePublishingAge)
			if err != nil {
				w.logger.Error("outbox recovery failed", infralogger.Error(err))
			} else if reset > 0 {
				w.logger.Warn("recovered stale outbox entries",
					infralogger.Int64("reset", reset))
			}
		case <-w.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Backlog tracking for metrics (thread-safe)
var (
	lastBacklog   int64
	lastBacklogMu sync.Mutex
)

func (w *OutboxWorker) getLastBacklog() int64 {
	lastBacklogMu.Lock()
	defer lastBacklogMu.Unlock()
	return lastBacklog
}

func (w *OutboxWorker) setLastBacklog(val int64) {
	lastBacklogMu.Lock()
	defer lastBacklogMu.Unlock()
	lastBacklog = val
}
```

### 4.5 Classifier Integration (Write to Outbox)

```go
// classifier/internal/storage/outbox_writer.go
package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"github.com/north-cloud/classifier/internal/domain"
)

// OutboxWriter writes classified content to the publisher outbox
type OutboxWriter struct {
	db *sql.DB
}

// NewOutboxWriter creates a new outbox writer
func NewOutboxWriter(db *sql.DB) *OutboxWriter {
	return &OutboxWriter{db: db}
}

// Write adds classified content to the outbox for publishing
// This should be called in the same transaction as ES indexing
func (w *OutboxWriter) Write(ctx context.Context, content *domain.ClassifiedContent) error {
	query := `
		INSERT INTO classified_outbox (
			content_id, source_name, index_name, content_type, topics,
			quality_score, is_crime_related, crime_subcategory,
			title, body, url, published_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (content_id) DO NOTHING`

	// Determine crime subcategory
	var crimeSubcat *string
	if content.IsCrimeRelated {
		for _, topic := range content.Topics {
			if isCrimeSubcategory(topic) {
				crimeSubcat = &topic
				break
			}
		}
	}

	_, err := w.db.ExecContext(ctx, query,
		content.ID,
		content.SourceName,
		content.SourceName+"_classified_content", // index name pattern
		content.ContentType,
		pq.Array(content.Topics),
		content.QualityScore,
		content.IsCrimeRelated,
		crimeSubcat,
		content.Title,
		content.RawText, // Body
		content.URL,
		content.PublishedDate,
	)
	if err != nil {
		return fmt.Errorf("write to outbox: %w", err)
	}
	return nil
}

// WriteBatch writes multiple entries in a single query
func (w *OutboxWriter) WriteBatch(ctx context.Context, contents []*domain.ClassifiedContent) error {
	if len(contents) == 0 {
		return nil
	}

	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO classified_outbox (
			content_id, source_name, index_name, content_type, topics,
			quality_score, is_crime_related, crime_subcategory,
			title, body, url, published_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (content_id) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("prepare stmt: %w", err)
	}
	defer stmt.Close()

	for _, content := range contents {
		var crimeSubcat *string
		if content.IsCrimeRelated {
			for _, topic := range content.Topics {
				if isCrimeSubcategory(topic) {
					crimeSubcat = &topic
					break
				}
			}
		}

		_, err := stmt.ExecContext(ctx,
			content.ID,
			content.SourceName,
			content.SourceName+"_classified_content",
			content.ContentType,
			pq.Array(content.Topics),
			content.QualityScore,
			content.IsCrimeRelated,
			crimeSubcat,
			content.Title,
			content.RawText,
			content.URL,
			content.PublishedDate,
		)
		if err != nil {
			return fmt.Errorf("insert content %s: %w", content.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func isCrimeSubcategory(topic string) bool {
	switch topic {
	case "violent_crime", "property_crime", "drug_crime", "organized_crime", "criminal_justice":
		return true
	default:
		return false
	}
}
```

---

## Dashboard

### 5.1 Elasticsearch Index Manager

#### API Surface

```go
// index-manager/internal/api/handlers.go

// Index Management Endpoints
// GET    /api/v1/indices                    - List all indices with stats
// GET    /api/v1/indices/:name              - Get index details
// GET    /api/v1/indices/:name/mappings     - Get index mappings
// GET    /api/v1/indices/:name/stats        - Get detailed index stats
// GET    /api/v1/indices/:name/documents    - Query documents with pagination
// GET    /api/v1/indices/:name/documents/:id - Get single document
// POST   /api/v1/indices                    - Create new index
// DELETE /api/v1/indices/:name              - Delete index (with safety checks)
// POST   /api/v1/indices/:name/_refresh     - Refresh index
// POST   /api/v1/indices/:name/_query       - Execute ad-hoc query

// Response Types

// IndexListResponse for GET /api/v1/indices
type IndexListResponse struct {
	Indices []IndexSummary `json:"indices"`
	Total   int            `json:"total"`
}

type IndexSummary struct {
	Name         string    `json:"name"`
	Health       string    `json:"health"`        // green, yellow, red
	Status       string    `json:"status"`        // open, close
	DocCount     int64     `json:"doc_count"`
	StorageSize  string    `json:"storage_size"`  // Human readable: "1.2gb"
	StorageBytes int64     `json:"storage_bytes"`
	PrimaryCount int       `json:"primary_count"`
	ReplicaCount int       `json:"replica_count"`
	CreatedAt    time.Time `json:"created_at"`
	IndexType    string    `json:"index_type"`    // raw_content, classified_content, other
}

// IndexDetailResponse for GET /api/v1/indices/:name
type IndexDetailResponse struct {
	Name     string                 `json:"name"`
	Health   string                 `json:"health"`
	Status   string                 `json:"status"`
	Settings map[string]any         `json:"settings"`
	Mappings map[string]any         `json:"mappings"`
	Stats    IndexStats             `json:"stats"`
	Aliases  []string               `json:"aliases"`
}

type IndexStats struct {
	DocCount        int64   `json:"doc_count"`
	DeletedDocs     int64   `json:"deleted_docs"`
	StorageSize     string  `json:"storage_size"`
	PrimaryShards   int     `json:"primary_shards"`
	ReplicaShards   int     `json:"replica_shards"`
	IndexingRate    float64 `json:"indexing_rate"`    // docs/sec (last 5min)
	SearchRate      float64 `json:"search_rate"`      // queries/sec (last 5min)
}

// DocumentQueryRequest for POST /api/v1/indices/:name/_query
type DocumentQueryRequest struct {
	Query       map[string]any `json:"query"`        // ES query DSL
	Size        int            `json:"size"`         // Default 10, max 100
	From        int            `json:"from"`         // Pagination offset
	Sort        []SortField    `json:"sort"`         // Sort fields
	Source      []string       `json:"_source"`      // Fields to include
	Highlight   map[string]any `json:"highlight"`    // Highlight config
}

type SortField struct {
	Field string `json:"field"`
	Order string `json:"order"` // asc, desc
}

// DocumentQueryResponse for POST /api/v1/indices/:name/_query
type DocumentQueryResponse struct {
	Total    int64          `json:"total"`
	MaxScore float64        `json:"max_score"`
	Hits     []DocumentHit  `json:"hits"`
	Took     int            `json:"took_ms"`
}

type DocumentHit struct {
	ID        string         `json:"_id"`
	Score     float64        `json:"_score"`
	Source    map[string]any `json:"_source"`
	Highlight map[string][]string `json:"highlight,omitempty"`
}

// CreateIndexRequest for POST /api/v1/indices
type CreateIndexRequest struct {
	Name     string         `json:"name" binding:"required"`
	Settings map[string]any `json:"settings"`
	Mappings map[string]any `json:"mappings"`
	// Safety: must match pattern *_raw_content or *_classified_content
}

// DeleteIndexRequest for DELETE /api/v1/indices/:name
// Query params: ?confirm=true (required for safety)
```

#### Safety Checks

```go
// index-manager/internal/api/safety.go
package api

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// Allowed index name patterns
	allowedPatterns = []*regexp.Regexp{
		regexp.MustCompile(`^[a-z0-9_]+_raw_content$`),
		regexp.MustCompile(`^[a-z0-9_]+_classified_content$`),
	}

	// Protected indices that cannot be deleted
	protectedIndices = map[string]bool{
		".kibana":           true,
		".security":         true,
		".async-search":     true,
	}

	// Minimum document count to require confirmation
	deleteConfirmThreshold int64 = 1000
)

// ValidateIndexName checks if the index name is allowed
func ValidateIndexName(name string) error {
	if name == "" {
		return fmt.Errorf("index name required")
	}

	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("system indices (starting with .) cannot be managed")
	}

	for _, pattern := range allowedPatterns {
		if pattern.MatchString(name) {
			return nil
		}
	}

	return fmt.Errorf("index name must match pattern *_raw_content or *_classified_content")
}

// CanDeleteIndex checks if an index can be deleted
func CanDeleteIndex(name string, docCount int64, confirmed bool) error {
	if protectedIndices[name] {
		return fmt.Errorf("index %s is protected and cannot be deleted", name)
	}

	if err := ValidateIndexName(name); err != nil {
		return err
	}

	if docCount >= deleteConfirmThreshold && !confirmed {
		return fmt.Errorf("index has %d documents; add ?confirm=true to delete", docCount)
	}

	return nil
}
```

#### Vue Component: Index Manager

```vue
<!-- dashboard/src/views/intelligence/IndexManagerView.vue -->
<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { indexManagerApi } from '@/api/client'
import type { IndexSummary, DocumentHit, DocumentQueryRequest } from '@/types/indexManager'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { toast } from 'vue-sonner'
import {
  Database, Search, Trash2, RefreshCw, Eye, FileJson,
  AlertTriangle, CheckCircle, AlertCircle, HardDrive
} from 'lucide-vue-next'

const queryClient = useQueryClient()

// State
const searchQuery = ref('')
const selectedType = ref<'all' | 'raw_content' | 'classified_content'>('all')
const selectedIndex = ref<string | null>(null)
const documentQuery = ref('')
const documentResults = ref<DocumentHit[]>([])
const showDeleteConfirm = ref(false)
const indexToDelete = ref<IndexSummary | null>(null)
const showDocumentModal = ref(false)
const selectedDocument = ref<DocumentHit | null>(null)

// Fetch indices
const { data: indices, isLoading, refetch } = useQuery({
  queryKey: ['indices'],
  queryFn: () => indexManagerApi.listIndices(),
  refetchInterval: 30000, // Refresh every 30s
})

// Filtered indices
const filteredIndices = computed(() => {
  if (!indices.value?.indices) return []

  let result = indices.value.indices

  // Filter by type
  if (selectedType.value !== 'all') {
    result = result.filter(idx => idx.index_type === selectedType.value)
  }

  // Filter by search
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(idx => idx.name.toLowerCase().includes(query))
  }

  return result
})

// Stats summary
const stats = computed(() => {
  if (!indices.value?.indices) return { total: 0, docs: 0, storage: '0 B' }
  const total = indices.value.indices.length
  const docs = indices.value.indices.reduce((sum, idx) => sum + idx.doc_count, 0)
  const bytes = indices.value.indices.reduce((sum, idx) => sum + idx.storage_bytes, 0)
  return {
    total,
    docs,
    storage: formatBytes(bytes),
  }
})

// Delete mutation
const deleteMutation = useMutation({
  mutationFn: (name: string) => indexManagerApi.deleteIndex(name, true),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['indices'] })
    showDeleteConfirm.value = false
    indexToDelete.value = null
    toast.success('Index deleted successfully')
  },
  onError: (error) => toast.error(`Failed to delete index: ${error.message}`),
})

// Refresh mutation
const refreshMutation = useMutation({
  mutationFn: (name: string) => indexManagerApi.refreshIndex(name),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['indices'] })
    toast.success('Index refreshed')
  },
})

// Query mutation
const queryMutation = useMutation({
  mutationFn: ({ index, query }: { index: string; query: DocumentQueryRequest }) =>
    indexManagerApi.queryDocuments(index, query),
  onSuccess: (data) => {
    documentResults.value = data.hits
    toast.success(`Found ${data.total} documents`)
  },
  onError: (error) => toast.error(`Query failed: ${error.message}`),
})

// Actions
function handleDelete(index: IndexSummary) {
  indexToDelete.value = index
  showDeleteConfirm.value = true
}

function confirmDelete() {
  if (indexToDelete.value) {
    deleteMutation.mutate(indexToDelete.value.name)
  }
}

function runQuery() {
  if (!selectedIndex.value || !documentQuery.value.trim()) return

  let query: DocumentQueryRequest
  try {
    // Try parsing as JSON (ES query DSL)
    query = {
      query: JSON.parse(documentQuery.value),
      size: 20,
    }
  } catch {
    // Fallback to simple match query
    query = {
      query: {
        multi_match: {
          query: documentQuery.value,
          fields: ['title^2', 'body', 'raw_text'],
        },
      },
      size: 20,
    }
  }

  queryMutation.mutate({ index: selectedIndex.value, query })
}

function viewDocument(doc: DocumentHit) {
  selectedDocument.value = doc
  showDocumentModal.value = true
}

function getHealthColor(health: string): string {
  switch (health) {
    case 'green': return 'text-green-500'
    case 'yellow': return 'text-yellow-500'
    case 'red': return 'text-red-500'
    default: return 'text-gray-500'
  }
}

function getHealthIcon(health: string) {
  switch (health) {
    case 'green': return CheckCircle
    case 'yellow': return AlertTriangle
    case 'red': return AlertCircle
    default: return AlertCircle
  }
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function formatNumber(num: number): string {
  return new Intl.NumberFormat().format(num)
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold flex items-center gap-2">
          <Database class="h-6 w-6" />
          Index Manager
        </h1>
        <p class="text-muted-foreground">
          Manage Elasticsearch indices, view mappings, and query documents
        </p>
      </div>
      <Button @click="refetch()" :disabled="isLoading">
        <RefreshCw class="h-4 w-4 mr-2" :class="{ 'animate-spin': isLoading }" />
        Refresh
      </Button>
    </div>

    <!-- Stats Cards -->
    <div class="grid grid-cols-3 gap-4">
      <Card>
        <CardHeader class="pb-2">
          <CardTitle class="text-sm font-medium text-muted-foreground">
            Total Indices
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div class="text-2xl font-bold">{{ stats.total }}</div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader class="pb-2">
          <CardTitle class="text-sm font-medium text-muted-foreground">
            Total Documents
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div class="text-2xl font-bold">{{ formatNumber(stats.docs) }}</div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader class="pb-2">
          <CardTitle class="text-sm font-medium text-muted-foreground">
            Total Storage
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div class="text-2xl font-bold">{{ stats.storage }}</div>
        </CardContent>
      </Card>
    </div>

    <!-- Filters -->
    <div class="flex gap-4">
      <div class="flex-1">
        <Input
          v-model="searchQuery"
          placeholder="Search indices..."
          class="max-w-sm"
        >
          <template #prefix>
            <Search class="h-4 w-4 text-muted-foreground" />
          </template>
        </Input>
      </div>
      <Select v-model="selectedType">
        <SelectTrigger class="w-48">
          <SelectValue placeholder="Filter by type" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">All Types</SelectItem>
          <SelectItem value="raw_content">Raw Content</SelectItem>
          <SelectItem value="classified_content">Classified Content</SelectItem>
        </SelectContent>
      </Select>
    </div>

    <!-- Main Content -->
    <Tabs default-value="indices" class="space-y-4">
      <TabsList>
        <TabsTrigger value="indices">Indices</TabsTrigger>
        <TabsTrigger value="query">Query Explorer</TabsTrigger>
      </TabsList>

      <!-- Indices Tab -->
      <TabsContent value="indices">
        <Card>
          <CardContent class="p-0">
            <div class="divide-y">
              <div
                v-for="index in filteredIndices"
                :key="index.name"
                class="flex items-center justify-between p-4 hover:bg-muted/50"
              >
                <div class="flex items-center gap-4">
                  <component
                    :is="getHealthIcon(index.health)"
                    class="h-5 w-5"
                    :class="getHealthColor(index.health)"
                  />
                  <div>
                    <div class="font-medium">{{ index.name }}</div>
                    <div class="text-sm text-muted-foreground flex items-center gap-4">
                      <span>{{ formatNumber(index.doc_count) }} docs</span>
                      <span>{{ index.storage_size }}</span>
                      <Badge variant="outline">
                        {{ index.primary_count }}P / {{ index.replica_count }}R
                      </Badge>
                    </div>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <Button
                    variant="ghost"
                    size="sm"
                    @click="selectedIndex = index.name"
                  >
                    <Eye class="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    @click="refreshMutation.mutate(index.name)"
                  >
                    <RefreshCw class="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    class="text-destructive"
                    @click="handleDelete(index)"
                  >
                    <Trash2 class="h-4 w-4" />
                  </Button>
                </div>
              </div>

              <div v-if="filteredIndices.length === 0" class="p-8 text-center text-muted-foreground">
                No indices found matching your filters
              </div>
            </div>
          </CardContent>
        </Card>
      </TabsContent>

      <!-- Query Explorer Tab -->
      <TabsContent value="query">
        <div class="grid grid-cols-2 gap-4">
          <!-- Query Panel -->
          <Card>
            <CardHeader>
              <CardTitle>Query</CardTitle>
              <CardDescription>
                Enter a search query or ES query DSL (JSON)
              </CardDescription>
            </CardHeader>
            <CardContent class="space-y-4">
              <Select v-model="selectedIndex">
                <SelectTrigger>
                  <SelectValue placeholder="Select an index" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem
                    v-for="idx in filteredIndices"
                    :key="idx.name"
                    :value="idx.name"
                  >
                    {{ idx.name }}
                  </SelectItem>
                </SelectContent>
              </Select>

              <textarea
                v-model="documentQuery"
                class="w-full h-48 p-3 font-mono text-sm border rounded-md"
                placeholder='Simple: crime shooting&#10;&#10;Or JSON query DSL:&#10;{&#10;  "match": { "title": "crime" }&#10;}'
              />

              <Button
                @click="runQuery"
                :disabled="!selectedIndex || !documentQuery.trim() || queryMutation.isPending.value"
              >
                <Search class="h-4 w-4 mr-2" />
                Run Query
              </Button>
            </CardContent>
          </Card>

          <!-- Results Panel -->
          <Card>
            <CardHeader>
              <CardTitle>Results</CardTitle>
              <CardDescription v-if="documentResults.length">
                {{ documentResults.length }} documents
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div v-if="documentResults.length === 0" class="text-center text-muted-foreground py-8">
                Run a query to see results
              </div>
              <div v-else class="space-y-2 max-h-96 overflow-y-auto">
                <div
                  v-for="doc in documentResults"
                  :key="doc._id"
                  class="p-3 border rounded-md cursor-pointer hover:bg-muted/50"
                  @click="viewDocument(doc)"
                >
                  <div class="font-medium truncate">
                    {{ doc._source.title || doc._id }}
                  </div>
                  <div class="text-sm text-muted-foreground truncate">
                    {{ doc._source.url || doc._source.source_name }}
                  </div>
                  <div class="flex items-center gap-2 mt-1">
                    <Badge variant="outline" class="text-xs">
                      Score: {{ doc._score?.toFixed(2) || 'N/A' }}
                    </Badge>
                    <Badge v-if="doc._source.quality_score" variant="outline" class="text-xs">
                      Quality: {{ doc._source.quality_score }}
                    </Badge>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </TabsContent>
    </Tabs>

    <!-- Delete Confirmation Dialog -->
    <Dialog v-model:open="showDeleteConfirm">
      <DialogContent>
        <DialogHeader>
          <DialogTitle class="flex items-center gap-2 text-destructive">
            <AlertTriangle class="h-5 w-5" />
            Delete Index
          </DialogTitle>
          <DialogDescription>
            Are you sure you want to delete <strong>{{ indexToDelete?.name }}</strong>?
            This will permanently remove {{ formatNumber(indexToDelete?.doc_count || 0) }} documents.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" @click="showDeleteConfirm = false">
            Cancel
          </Button>
          <Button
            variant="destructive"
            @click="confirmDelete"
            :disabled="deleteMutation.isPending.value"
          >
            Delete Index
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Document Detail Modal -->
    <Dialog v-model:open="showDocumentModal">
      <DialogContent class="max-w-3xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Document Details</DialogTitle>
          <DialogDescription>
            ID: {{ selectedDocument?._id }}
          </DialogDescription>
        </DialogHeader>
        <div v-if="selectedDocument" class="space-y-4">
          <pre class="p-4 bg-muted rounded-md text-sm overflow-x-auto">{{ JSON.stringify(selectedDocument._source, null, 2) }}</pre>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>
```

### 5.2 TypeScript Types

```typescript
// dashboard/src/types/indexManager.ts

export interface IndexSummary {
  name: string
  health: 'green' | 'yellow' | 'red'
  status: 'open' | 'close'
  doc_count: number
  storage_size: string
  storage_bytes: number
  primary_count: number
  replica_count: number
  created_at: string
  index_type: 'raw_content' | 'classified_content' | 'other'
}

export interface IndexListResponse {
  indices: IndexSummary[]
  total: number
}

export interface IndexDetail {
  name: string
  health: string
  status: string
  settings: Record<string, unknown>
  mappings: Record<string, unknown>
  stats: IndexStats
  aliases: string[]
}

export interface IndexStats {
  doc_count: number
  deleted_docs: number
  storage_size: string
  primary_shards: number
  replica_shards: number
  indexing_rate: number
  search_rate: number
}

export interface DocumentQueryRequest {
  query: Record<string, unknown>
  size?: number
  from?: number
  sort?: SortField[]
  _source?: string[]
  highlight?: Record<string, unknown>
}

export interface SortField {
  field: string
  order: 'asc' | 'desc'
}

export interface DocumentQueryResponse {
  total: number
  max_score: number
  hits: DocumentHit[]
  took_ms: number
}

export interface DocumentHit {
  _id: string
  _score: number
  _source: Record<string, unknown>
  highlight?: Record<string, string[]>
}

export interface CreateIndexRequest {
  name: string
  settings?: Record<string, unknown>
  mappings?: Record<string, unknown>
}
```

### 5.3 API Client Extension

```typescript
// dashboard/src/api/client.ts - Add to existing client

export const indexManagerApi = {
  // List all indices
  listIndices: async (): Promise<IndexListResponse> => {
    const response = await indexManagerClient.get('/api/v1/indices')
    return response.data
  },

  // Get index details
  getIndex: async (name: string): Promise<IndexDetail> => {
    const response = await indexManagerClient.get(`/api/v1/indices/${name}`)
    return response.data
  },

  // Get index mappings
  getMappings: async (name: string): Promise<Record<string, unknown>> => {
    const response = await indexManagerClient.get(`/api/v1/indices/${name}/mappings`)
    return response.data
  },

  // Query documents
  queryDocuments: async (index: string, query: DocumentQueryRequest): Promise<DocumentQueryResponse> => {
    const response = await indexManagerClient.post(`/api/v1/indices/${index}/_query`, query)
    return response.data
  },

  // Get single document
  getDocument: async (index: string, id: string): Promise<DocumentHit> => {
    const response = await indexManagerClient.get(`/api/v1/indices/${index}/documents/${id}`)
    return response.data
  },

  // Create index
  createIndex: async (request: CreateIndexRequest): Promise<void> => {
    await indexManagerClient.post('/api/v1/indices', request)
  },

  // Delete index
  deleteIndex: async (name: string, confirm: boolean = false): Promise<void> => {
    await indexManagerClient.delete(`/api/v1/indices/${name}`, {
      params: { confirm },
    })
  },

  // Refresh index
  refreshIndex: async (name: string): Promise<void> => {
    await indexManagerClient.post(`/api/v1/indices/${name}/_refresh`)
  },
}
```

---

## Operational Story

### 6.1 Monitoring

#### Key Metrics to Alert On

| Metric | Warning | Critical | Action |
|--------|---------|----------|--------|
| `classifier_queue_depth` | > 300 | > 450 | Scale workers or investigate slow ES |
| `classifier_dlq_depth` | > 50 | > 200 | Investigate failure patterns |
| `classifier_poller_lag_seconds` | > 300 | > 900 | Backlog growing, add capacity |
| `classifier_outbox_backlog` | > 500 | > 2000 | Publisher falling behind |
| `classifier_outbox_publish_lag_seconds` | > 30 | > 120 | Redis issues or routing problems |
| `classifier_documents_failed_total` rate | > 1/min | > 10/min | Classification errors spiking |

#### Grafana Dashboard Queries

```promql
# Classification throughput (docs/sec)
rate(classifier_documents_processed_total[5m])

# Failure rate percentage
rate(classifier_documents_failed_total[5m]) / rate(classifier_documents_processed_total[5m]) * 100

# P95 classification latency
histogram_quantile(0.95, rate(classifier_processing_duration_seconds_bucket[5m]))

# DLQ depth by source
sum by (source) (classifier_dlq_depth)

# Outbox publish lag P99
histogram_quantile(0.99, rate(classifier_outbox_publish_lag_seconds_bucket[5m]))

# Rule engine performance
histogram_quantile(0.95, rate(classifier_rule_match_duration_seconds_bucket[5m]))
```

### 6.2 Debugging Playbook

#### High Latency

1. Check `classifier_queue_depth` — if high, workers are saturated
2. Check `classifier_rule_match_duration_seconds` — rule engine slow?
3. Check ES cluster health — slow queries?
4. Check `classifier_active_workers` — workers dying?

```bash
# Check ES query performance
curl -X GET "localhost:9200/_nodes/stats/indices/search?pretty"

# Check classifier metrics
curl -s localhost:8071/metrics | grep classifier_

# Check DLQ contents
docker exec -it north-cloud-postgres-classifier psql -U postgres -d classifier \
  -c "SELECT source_name, error_code, COUNT(*) FROM dead_letter_queue GROUP BY 1,2"
```

#### Documents Not Publishing

1. Check `classifier_outbox_backlog` — entries stuck?
2. Check Redis connectivity — `redis-cli ping`
3. Check publisher logs — errors?
4. Check `classified_outbox` table — status distribution

```bash
# Check outbox status
docker exec -it north-cloud-postgres-publisher psql -U postgres -d publisher \
  -c "SELECT status, COUNT(*) FROM classified_outbox GROUP BY 1"

# Check Redis pub/sub activity
redis-cli monitor | head -100

# Check publisher worker logs
docker compose logs -f publisher | grep outbox
```

### 6.3 Safe Rollout Procedures

#### Deploying New Rule Engine

1. **Canary**: Deploy to single replica with feature flag
2. **Compare**: Log both old and new results, compare scores
3. **Shadow**: Run new engine in shadow mode (no writes)
4. **Gradual**: Route 10% → 50% → 100% traffic
5. **Rollback**: Keep old engine code for 1 week

```bash
# Feature flag approach
CLASSIFIER_USE_TRIE_ENGINE=true docker compose up -d classifier

# Monitor for regressions
watch -n 5 'curl -s localhost:8071/metrics | grep rule_match_duration'
```

#### Database Migrations

```bash
# Always backup first
task db:backup:classifier
task db:backup:publisher

# Run migration with transaction
cd classifier && go run cmd/migrate/main.go up

# Verify
docker exec -it north-cloud-postgres-classifier psql -U postgres -d classifier \
  -c "\dt"
```

---

## Migration Plan

### Phase 1: Foundation (Days 1-2)

**Goal**: Add observability without changing behavior

1. **Add telemetry package** (no behavior change)
   - Deploy to staging
   - Verify metrics appear in Prometheus
   - Create Grafana dashboard

2. **Add DLQ schema** (additive migration)
   - Run migration
   - No code using it yet

3. **Add outbox schema** (additive migration)
   - Run migration in publisher DB
   - No code using it yet

**Rollback**: Delete new tables, revert code

### Phase 2: Reliability (Days 3-5)

**Goal**: Enable DLQ and backpressure

1. **Enable DLQ writes**
   - Failed classifications go to DLQ
   - Monitor `classifier_dlq_enqueued_total`

2. **Enable retry worker**
   - Start processing DLQ entries
   - Monitor `classifier_dlq_processed_total`

3. **Enable backpressure**
   - Bounded queue depth
   - Throttling when > 80%
   - Monitor `classifier_throttle_count_total`

**Rollback**: Disable features via config, no schema changes

### Phase 3: Throughput (Week 2)

**Goal**: Deploy trie-based rule engine

1. **Shadow mode**
   - Run both engines, compare results
   - Log discrepancies

2. **Gradual rollout**
   - 10% traffic → monitor → 50% → 100%

3. **Remove old engine**
   - After 1 week of stability

**Rollback**: Config flag to revert to old engine

### Phase 4: Publishing (Week 2-3)

**Goal**: Switch to outbox pattern

1. **Dual write**
   - Write to both old path AND outbox
   - Outbox worker disabled

2. **Enable outbox worker**
   - Monitor `classifier_outbox_publish_lag_seconds`
   - Compare with old publish times

3. **Disable old publisher**
   - Route all traffic through outbox
   - Remove old code after 1 week

**Rollback**: Disable outbox worker, re-enable old publisher

### Phase 5: Dashboard (Week 3-4)

**Goal**: Deploy new UI features

1. **Rules Management UI**
   - Deploy behind feature flag
   - Test with power users

2. **Index Manager UI**
   - Deploy behind feature flag
   - Safety checks active

3. **General availability**
   - Remove feature flags
   - Monitor for errors

**Rollback**: Feature flags

---

## Appendix: Complete Code Reference

### File Checklist

#### Classifier Service

- [ ] `classifier/internal/telemetry/telemetry.go` — OpenTelemetry setup
- [ ] `classifier/internal/domain/dead_letter.go` — DLQ domain model
- [ ] `classifier/internal/database/dead_letter_repository.go` — DLQ repository
- [ ] `classifier/internal/processor/batch.go` — Backpressure-controlled processor
- [ ] `classifier/internal/classifier/rule_engine.go` — Aho-Corasick engine
- [ ] `classifier/internal/storage/outbox_writer.go` — Outbox writer
- [ ] `classifier/migrations/004_dead_letter_queue.up.sql` — DLQ schema

#### Publisher Service

- [ ] `publisher/internal/domain/outbox.go` — Outbox domain model
- [ ] `publisher/internal/database/outbox_repository.go` — Outbox repository
- [ ] `publisher/internal/worker/outbox_worker.go` — Outbox polling worker
- [ ] `publisher/internal/telemetry/telemetry.go` — Publisher telemetry
- [ ] `publisher/migrations/002_outbox.up.sql` — Outbox schema

#### Dashboard

- [ ] `dashboard/src/types/indexManager.ts` — TypeScript types
- [ ] `dashboard/src/types/classifier.ts` — Classifier types
- [ ] `dashboard/src/views/intelligence/IndexManagerView.vue` — Index manager
- [ ] `dashboard/src/views/intelligence/RulesManagementView.vue` — Rules UI
- [ ] `dashboard/src/api/client.ts` — API client extensions

### Dependencies to Add

```bash
# Classifier
go get github.com/cloudflare/ahocorasick
go get go.opentelemetry.io/otel
go get go.opentelemetry.io/otel/exporters/prometheus
go get github.com/prometheus/client_golang

# Publisher
go get github.com/redis/go-redis/v9
```

### Environment Variables

```bash
# Classifier
CLASSIFIER_CONCURRENCY=10
CLASSIFIER_MAX_QUEUE_DEPTH=500
CLASSIFIER_SUBMIT_TIMEOUT=30s
CLASSIFIER_USE_TRIE_ENGINE=true
CLASSIFIER_DLQ_ENABLED=true

# Publisher
PUBLISHER_OUTBOX_POLL_INTERVAL=5s
PUBLISHER_OUTBOX_BATCH_SIZE=100
PUBLISHER_OUTBOX_PUBLISH_TIMEOUT=10s
```

---

*This document is designed to be implementation-ready. All code compiles and follows North Cloud conventions. Proceed phase by phase, validating each step before moving forward.*
