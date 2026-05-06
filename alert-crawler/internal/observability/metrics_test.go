package observability_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/observability"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// logEntry records one emitted log call for inspection in tests.
type logEntry struct {
	level  string
	msg    string
	fields []infralogger.Field
}

// spyLogger implements infralogger.Logger and captures every call.
type spyLogger struct {
	entries []logEntry
}

func (s *spyLogger) Debug(msg string, fields ...infralogger.Field) {
	s.entries = append(s.entries, logEntry{level: "debug", msg: msg, fields: fields})
}

func (s *spyLogger) Info(msg string, fields ...infralogger.Field) {
	s.entries = append(s.entries, logEntry{level: "info", msg: msg, fields: fields})
}

func (s *spyLogger) Warn(msg string, fields ...infralogger.Field) {
	s.entries = append(s.entries, logEntry{level: "warn", msg: msg, fields: fields})
}

func (s *spyLogger) Error(msg string, fields ...infralogger.Field) {
	s.entries = append(s.entries, logEntry{level: "error", msg: msg, fields: fields})
}

func (s *spyLogger) Fatal(msg string, fields ...infralogger.Field) {
	s.entries = append(s.entries, logEntry{level: "fatal", msg: msg, fields: fields})
}

func (s *spyLogger) With(_ ...infralogger.Field) infralogger.Logger { return s }

func (s *spyLogger) Sync() error { return nil }

func (s *spyLogger) last() logEntry {
	if len(s.entries) == 0 {
		return logEntry{}
	}
	return s.entries[len(s.entries)-1]
}

// newSpyMetrics returns a Metrics backed by a spy logger for test assertions.
func newSpyMetrics(t *testing.T) (*observability.Metrics, *spyLogger) {
	t.Helper()
	spy := &spyLogger{}
	return observability.New(spy), spy
}

func TestRecordPoll_EmitsInfo(t *testing.T) {
	m, spy := newSpyMetrics(t)
	m.RecordPoll("src-01", "ok", 123*time.Millisecond)

	e := spy.last()
	assert.Equal(t, "info", e.level)
	assert.Equal(t, "poll completed", e.msg)
	requireFieldKey(t, e.fields, "source_id")
	requireFieldKey(t, e.fields, "result")
	requireFieldKey(t, e.fields, "duration")
}

func TestRecordPoll_NotModified_EmitsInfo(t *testing.T) {
	m, spy := newSpyMetrics(t)
	m.RecordPoll("src-02", "not_modified", 5*time.Millisecond)

	e := spy.last()
	assert.Equal(t, "info", e.level)
	assert.Equal(t, "poll completed", e.msg)
}

func TestRecordCreated_EmitsInfo(t *testing.T) {
	m, spy := newSpyMetrics(t)
	m.RecordCreated("src-01", "harm_reduction", "high")

	e := spy.last()
	assert.Equal(t, "info", e.level)
	assert.Equal(t, "alert created", e.msg)
	requireFieldKey(t, e.fields, "severity")
	requireFieldKey(t, e.fields, "category")
}

func TestRecordUpdated_EmitsInfo(t *testing.T) {
	m, spy := newSpyMetrics(t)
	m.RecordUpdated("src-01", "harm_reduction", "medium")

	e := spy.last()
	assert.Equal(t, "info", e.level)
	assert.Equal(t, "alert updated", e.msg)
}

func TestRecordRescinded_EmitsInfo(t *testing.T) {
	m, spy := newSpyMetrics(t)
	m.RecordRescinded("src-01")

	e := spy.last()
	assert.Equal(t, "info", e.level)
	assert.Equal(t, "alert rescinded", e.msg)
	requireFieldKey(t, e.fields, "source_id")
}

func TestRecordParseFailure_EmitsInfo(t *testing.T) {
	m, spy := newSpyMetrics(t)
	m.RecordParseFailure("src-01", "item")

	e := spy.last()
	assert.Equal(t, "info", e.level)
	assert.Equal(t, "parse failure", e.msg)
	requireFieldKey(t, e.fields, "kind")
}

func TestRecordESWriteFailure_EmitsWarn(t *testing.T) {
	m, spy := newSpyMetrics(t)
	m.RecordESWriteFailure("src-01", "create")

	e := spy.last()
	assert.Equal(t, "warn", e.level)
	assert.Equal(t, "elasticsearch write failure", e.msg)
	requireFieldKey(t, e.fields, "op")
}

func TestRecordRedisPublishFailure_EmitsWarn(t *testing.T) {
	m, spy := newSpyMetrics(t)
	m.RecordRedisPublishFailure("src-01", "created")

	e := spy.last()
	assert.Equal(t, "warn", e.level)
	assert.Equal(t, "redis publish failure", e.msg)
	requireFieldKey(t, e.fields, "event_type")
}

func TestRecordSourceError_EmitsError(t *testing.T) {
	m, spy := newSpyMetrics(t)
	m.RecordSourceError("src-01", errors.New("boom"))

	e := spy.last()
	assert.Equal(t, "error", e.level)
	assert.Equal(t, "source error", e.msg)
}

func TestRecordConsecutiveFailures_BelowThreshold_EmitsInfo(t *testing.T) {
	m, spy := newSpyMetrics(t)
	m.RecordConsecutiveFailures("src-01", 3)

	e := spy.last()
	assert.Equal(t, "info", e.level, "count below threshold should emit INFO")
	assert.Equal(t, "source consecutive failures", e.msg)
	requireFieldKey(t, e.fields, "consecutive_failures")
}

func TestRecordConsecutiveFailures_AtSix_EmitsWarn(t *testing.T) {
	m, spy := newSpyMetrics(t)
	// 6 == consecutiveFailureWarnThreshold in metrics.go (NFR-005).
	m.RecordConsecutiveFailures("src-01", 6)

	e := spy.last()
	assert.Equal(t, "warn", e.level, "count=6 must emit WARN per NFR-005")
	assert.Equal(t, "source consecutive failures threshold reached", e.msg)
	requireFieldKey(t, e.fields, "consecutive_failures")
}

func TestRecordConsecutiveFailures_AboveThreshold_EmitsWarn(t *testing.T) {
	m, spy := newSpyMetrics(t)
	m.RecordConsecutiveFailures("src-01", 10)

	e := spy.last()
	assert.Equal(t, "warn", e.level, "count above threshold must also emit WARN")
}

func TestRecordConsecutiveFailures_AtOne_EmitsInfo(t *testing.T) {
	m, spy := newSpyMetrics(t)
	m.RecordConsecutiveFailures("src-01", 1)

	e := spy.last()
	assert.Equal(t, "info", e.level)
}

// requireFieldKey asserts that at least one field with the given key exists.
func requireFieldKey(t *testing.T, fields []infralogger.Field, key string) {
	t.Helper()
	for _, f := range fields {
		if f.Key == key {
			return
		}
	}
	require.Failf(t, "field not found", "expected log field %q not found in %v", key, fields)
}
