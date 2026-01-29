// Package logs provides structured logging for crawler jobs.
package logs

import (
	"context"
)

// JobLogger provides structured logging for job execution.
// All methods are safe for concurrent use.
type JobLogger interface {
	// Core logging methods
	Info(category Category, msg string, fields ...Field)
	Warn(category Category, msg string, fields ...Field)
	Error(category Category, msg string, fields ...Field)
	Debug(category Category, msg string, fields ...Field)

	// Lifecycle events (always logged regardless of verbosity)
	JobStarted(sourceID, url string)
	JobCompleted(summary *JobSummary)
	JobFailed(err error)

	// Verbosity check (for expensive operations)
	IsDebugEnabled() bool
	IsTraceEnabled() bool

	// WithFields returns a scoped logger with pre-set fields.
	WithFields(fields ...Field) JobLogger

	// StartHeartbeat starts the heartbeat goroutine.
	StartHeartbeat(ctx context.Context)

	// BuildSummary returns the current metrics as a summary.
	BuildSummary() *JobSummary

	// Flush pending logs (called at job end)
	Flush() error
}

// JobSummary contains final statistics appended to job logs.
type JobSummary struct {
	// Core metrics
	PagesDiscovered int64 `json:"pages_discovered"`
	PagesCrawled    int64 `json:"pages_crawled"`
	ItemsExtracted  int64 `json:"items_extracted"`
	ErrorsCount     int64 `json:"errors_count"`

	// Duration breakdown
	Duration        int64 `json:"duration_ms"`
	CrawlDuration   int64 `json:"crawl_duration_ms,omitempty"`
	ExtractDuration int64 `json:"extract_duration_ms,omitempty"`
	BackoffDuration int64 `json:"backoff_duration_ms,omitempty"`

	// Network stats
	BytesFetched   int64 `json:"bytes_fetched,omitempty"`
	RequestsTotal  int64 `json:"requests_total,omitempty"`
	RequestsFailed int64 `json:"requests_failed,omitempty"`

	// Queue behavior
	QueueMaxDepth int64 `json:"queue_max_depth,omitempty"`
	QueueEnqueued int64 `json:"queue_enqueued,omitempty"`
	QueueDequeued int64 `json:"queue_dequeued,omitempty"`

	// StatusCodes provides a breakdown of HTTP status codes encountered.
	StatusCodes map[int]int64 `json:"status_codes,omitempty"`

	// TopErrors contains deduplicated errors (max 5).
	TopErrors []ErrorSummary `json:"top_errors,omitempty"`

	// Throttling stats
	LogsEmitted     int64   `json:"logs_emitted"`
	LogsThrottled   int64   `json:"logs_throttled,omitempty"`
	ThrottlePercent float64 `json:"throttle_percent,omitempty"`
}

// ErrorSummary summarizes a repeated error.
type ErrorSummary struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
	LastURL string `json:"last_url,omitempty"`
}
