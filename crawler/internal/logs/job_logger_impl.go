package logs

import (
	"context"
	"sync/atomic"
	"time"
)

// HeartbeatInterval is the interval between heartbeat log entries.
const HeartbeatInterval = 15 * time.Second

// MaxLogsPerJob is the maximum number of logs per job execution.
const MaxLogsPerJob = 50000

// jobLoggerImpl is the main implementation of JobLogger.
type jobLoggerImpl struct {
	jobID       string
	executionID string
	verbosity   Verbosity
	captureFunc func(LogEntry)
	throttler   *RateLimiter
	metrics     *LogMetrics
	logCount    atomic.Int64
}

// NewJobLoggerImpl creates a new JobLogger implementation.
// If captureFunc is nil, logs are discarded (useful for testing).
// If maxLogsPerSec <= 0, throttling is disabled.
func NewJobLoggerImpl(
	jobID, executionID string,
	verbosity Verbosity,
	captureFunc func(LogEntry),
	maxLogsPerSec int,
) JobLogger {
	return &jobLoggerImpl{
		jobID:       jobID,
		executionID: executionID,
		verbosity:   verbosity,
		captureFunc: captureFunc,
		throttler:   NewRateLimiter(maxLogsPerSec),
		metrics:     NewLogMetrics(),
	}
}

// Info logs an info-level message.
func (j *jobLoggerImpl) Info(category Category, msg string, fields ...Field) {
	j.log("info", category, msg, fields)
}

// Warn logs a warning-level message.
func (j *jobLoggerImpl) Warn(category Category, msg string, fields ...Field) {
	j.log("warn", category, msg, fields)
}

// Error logs an error-level message.
func (j *jobLoggerImpl) Error(category Category, msg string, fields ...Field) {
	j.log("error", category, msg, fields)
}

// Debug logs a debug-level message (requires debug verbosity).
func (j *jobLoggerImpl) Debug(category Category, msg string, fields ...Field) {
	if !j.verbosity.AllowsLevel("debug") {
		return
	}
	// Apply throttling to debug logs
	if !j.throttler.Allow() {
		j.metrics.IncrementThrottled()
		return
	}
	j.log("debug", category, msg, fields)
}

// log is the internal logging method.
func (j *jobLoggerImpl) log(level string, category Category, msg string, fields []Field) {
	if j.logCount.Load() >= MaxLogsPerJob {
		return
	}

	entry := LogEntry{
		SchemaVersion: CurrentSchemaVersion,
		Timestamp:     time.Now(),
		Level:         level,
		Category:      string(category),
		Message:       msg,
		JobID:         j.jobID,
		ExecID:        j.executionID,
		Fields:        FieldsToMap(fields),
	}

	j.logCount.Add(1)
	j.metrics.IncrementLogsEmitted()

	if j.captureFunc != nil {
		j.captureFunc(entry)
	}
}

// JobStarted logs the job started lifecycle event.
func (j *jobLoggerImpl) JobStarted(sourceID, url string) {
	j.log("info", CategoryLifecycle, "job_started", []Field{
		String("source_id", sourceID),
		URL(url),
	})
}

// JobCompleted logs the job completed lifecycle event.
func (j *jobLoggerImpl) JobCompleted(summary *JobSummary) {
	j.log("info", CategoryLifecycle, "job_completed", []Field{
		Int64("pages_crawled", summary.PagesCrawled),
		Int64("items_extracted", summary.ItemsExtracted),
		Int64("errors_count", summary.ErrorsCount),
		Int64("logs_emitted", summary.LogsEmitted),
		Int64("logs_throttled", summary.LogsThrottled),
	})
}

// JobFailed logs the job failed lifecycle event.
func (j *jobLoggerImpl) JobFailed(err error) {
	j.log("error", CategoryLifecycle, "job_failed", []Field{
		Err(err),
	})
}

// IsDebugEnabled returns true if debug logging is enabled.
func (j *jobLoggerImpl) IsDebugEnabled() bool {
	return j.verbosity.AllowsLevel("debug")
}

// IsTraceEnabled returns true if trace logging is enabled.
func (j *jobLoggerImpl) IsTraceEnabled() bool {
	return j.verbosity.AllowsLevel("trace")
}

// WithFields returns a scoped logger with pre-set fields.
func (j *jobLoggerImpl) WithFields(fields ...Field) JobLogger {
	return &scopedJobLogger{
		parent: j,
		fields: fields,
	}
}

// StartHeartbeat starts the heartbeat goroutine.
func (j *jobLoggerImpl) StartHeartbeat(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(HeartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				j.log("info", CategoryLifecycle, "heartbeat", []Field{
					Int64("logs_emitted", j.metrics.LogsEmitted()),
					Int64("queue_depth", j.metrics.QueueDepth()),
				})
			}
		}
	}()
}

// BuildSummary returns the current metrics as a summary.
func (j *jobLoggerImpl) BuildSummary() *JobSummary {
	return j.metrics.BuildSummary()
}

// Flush flushes pending logs.
func (j *jobLoggerImpl) Flush() error {
	// No buffering in this implementation - logs are captured immediately
	return nil
}

// scopedJobLogger wraps a parent logger with additional fields.
type scopedJobLogger struct {
	parent JobLogger
	fields []Field
}

// mergeFields combines parent fields with call-site fields.
// Call-site fields override parent fields with the same key.
func (s *scopedJobLogger) mergeFields(callFields []Field) []Field {
	capacity := len(s.fields) + len(callFields)
	merged := make([]Field, 0, capacity)
	merged = append(merged, s.fields...)
	merged = append(merged, callFields...)
	return merged
}

func (s *scopedJobLogger) Info(category Category, msg string, fields ...Field) {
	s.parent.Info(category, msg, s.mergeFields(fields)...)
}

func (s *scopedJobLogger) Warn(category Category, msg string, fields ...Field) {
	s.parent.Warn(category, msg, s.mergeFields(fields)...)
}

func (s *scopedJobLogger) Error(category Category, msg string, fields ...Field) {
	s.parent.Error(category, msg, s.mergeFields(fields)...)
}

func (s *scopedJobLogger) Debug(category Category, msg string, fields ...Field) {
	s.parent.Debug(category, msg, s.mergeFields(fields)...)
}

func (s *scopedJobLogger) JobStarted(sourceID, url string) {
	s.parent.JobStarted(sourceID, url)
}

func (s *scopedJobLogger) JobCompleted(summary *JobSummary) {
	s.parent.JobCompleted(summary)
}

func (s *scopedJobLogger) JobFailed(err error) {
	s.parent.JobFailed(err)
}

func (s *scopedJobLogger) IsDebugEnabled() bool {
	return s.parent.IsDebugEnabled()
}

func (s *scopedJobLogger) IsTraceEnabled() bool {
	return s.parent.IsTraceEnabled()
}

func (s *scopedJobLogger) WithFields(fields ...Field) JobLogger {
	return &scopedJobLogger{
		parent: s,
		fields: fields,
	}
}

func (s *scopedJobLogger) StartHeartbeat(ctx context.Context) {
	s.parent.StartHeartbeat(ctx)
}

func (s *scopedJobLogger) BuildSummary() *JobSummary {
	return s.parent.BuildSummary()
}

func (s *scopedJobLogger) Flush() error {
	return s.parent.Flush()
}

// Compile-time interface checks
var (
	_ JobLogger = (*jobLoggerImpl)(nil)
	_ JobLogger = (*scopedJobLogger)(nil)
)
