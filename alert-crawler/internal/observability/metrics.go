// Package observability provides structured-log emission helpers for alert-crawler.
//
// Layer: L1.  Imports: domain (L0), infrastructure/logger, stdlib only.
package observability

import (
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// consecutiveFailureWarnThreshold is the inclusive lower bound at which
// RecordConsecutiveFailures emits at WARN instead of INFO (NFR-005).
const consecutiveFailureWarnThreshold = 6

// Metrics wraps an infralogger.Logger and emits structured log entries for
// every observable event in the alert-crawler pipeline.
// Construct with New; do not copy after first use.
type Metrics struct {
	log infralogger.Logger
}

// New returns a Metrics emitter backed by log.
func New(log infralogger.Logger) *Metrics {
	return &Metrics{log: log}
}

// RecordPoll emits a poll completion entry.
// result is one of: "ok", "not_modified", "error".
func (m *Metrics) RecordPoll(sourceID, result string, duration time.Duration) {
	m.log.Info("poll completed",
		infralogger.String("source_id", sourceID),
		infralogger.String("result", result),
		infralogger.Duration("duration", duration),
	)
}

// RecordCreated emits an entry when a new alert is indexed and published.
func (m *Metrics) RecordCreated(sourceID, category, sev string) {
	m.log.Info("alert created",
		infralogger.String("source_id", sourceID),
		infralogger.String("category", category),
		infralogger.String("severity", sev),
	)
}

// RecordUpdated emits an entry when an existing alert is updated.
func (m *Metrics) RecordUpdated(sourceID, category, sev string) {
	m.log.Info("alert updated",
		infralogger.String("source_id", sourceID),
		infralogger.String("category", category),
		infralogger.String("severity", sev),
	)
}

// RecordRescinded emits an entry when an alert is rescinded.
func (m *Metrics) RecordRescinded(sourceID string) {
	m.log.Info("alert rescinded",
		infralogger.String("source_id", sourceID),
	)
}

// RecordParseFailure emits an entry when a feed item fails to parse.
// kind identifies the failure type (e.g. "required_fields", "pub_date").
func (m *Metrics) RecordParseFailure(sourceID, kind string) {
	m.log.Info("parse failure",
		infralogger.String("source_id", sourceID),
		infralogger.String("kind", kind),
	)
}

// RecordESWriteFailure emits an entry when an Elasticsearch write fails.
// op is one of: "create", "update", "rescind".
func (m *Metrics) RecordESWriteFailure(sourceID, op string) {
	m.log.Warn("elasticsearch write failure",
		infralogger.String("source_id", sourceID),
		infralogger.String("op", op),
	)
}

// RecordRedisPublishFailure emits an entry when a Redis publish fails.
// eventType is the lifecycle event type that failed to publish.
func (m *Metrics) RecordRedisPublishFailure(sourceID, eventType string) {
	m.log.Warn("redis publish failure",
		infralogger.String("source_id", sourceID),
		infralogger.String("event_type", eventType),
	)
}

// RecordConsecutiveFailures emits the running failure count for a source.
// When count reaches consecutiveFailureWarnThreshold or above (NFR-005), the
// entry is emitted at WARN; otherwise it is emitted at INFO.
func (m *Metrics) RecordConsecutiveFailures(sourceID string, count int) {
	fields := []infralogger.Field{
		infralogger.String("source_id", sourceID),
		infralogger.Int("consecutive_failures", count),
	}

	if count >= consecutiveFailureWarnThreshold {
		m.log.Warn("source consecutive failures threshold reached", fields...)
		return
	}

	m.log.Info("source consecutive failures", fields...)
}

// RecordSourceError emits an error-level entry for an unclassified source error.
func (m *Metrics) RecordSourceError(sourceID string, err error) {
	m.log.Error("source error",
		infralogger.String("source_id", sourceID),
		infralogger.Error(err),
	)
}
