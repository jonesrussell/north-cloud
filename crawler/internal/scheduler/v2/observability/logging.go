package observability

import (
	"context"
	"time"

	infralogger "github.com/north-cloud/infrastructure/logger"
)

// Logger provides structured logging for the V2 scheduler.
type Logger struct {
	log infralogger.Logger
}

// NewLogger creates a new scheduler logger.
func NewLogger(log infralogger.Logger) *Logger {
	return &Logger{log: log}
}

// JobScheduled logs a job being scheduled.
func (l *Logger) JobScheduled(ctx context.Context, jobID, scheduleType, priority string, nextRunAt time.Time) {
	l.log.Info("Job scheduled",
		infralogger.String("job_id", jobID),
		infralogger.String("schedule_type", scheduleType),
		infralogger.String("priority", priority),
		infralogger.String("next_run_at", nextRunAt.Format(time.RFC3339)),
	)
}

// JobStarted logs a job starting execution.
func (l *Logger) JobStarted(ctx context.Context, jobID, sourceID, url string) {
	l.log.Info("Job started",
		infralogger.String("job_id", jobID),
		infralogger.String("source_id", sourceID),
		infralogger.String("url", url),
	)
}

// JobCompleted logs a job completing successfully.
func (l *Logger) JobCompleted(ctx context.Context, jobID string, duration time.Duration, itemsCrawled, itemsIndexed int) {
	l.log.Info("Job completed",
		infralogger.String("job_id", jobID),
		infralogger.Duration("duration", duration),
		infralogger.Int("items_crawled", itemsCrawled),
		infralogger.Int("items_indexed", itemsIndexed),
	)
}

// JobFailed logs a job failure.
func (l *Logger) JobFailed(ctx context.Context, jobID string, err error, duration time.Duration) {
	l.log.Error("Job failed",
		infralogger.String("job_id", jobID),
		infralogger.Error(err),
		infralogger.Duration("duration", duration),
	)
}

// JobRetrying logs a job retry attempt.
func (l *Logger) JobRetrying(ctx context.Context, jobID string, attempt, maxRetries int, nextRetryAt time.Time) {
	l.log.Warn("Job retrying",
		infralogger.String("job_id", jobID),
		infralogger.Int("attempt", attempt),
		infralogger.Int("max_retries", maxRetries),
		infralogger.String("next_retry_at", nextRetryAt.Format(time.RFC3339)),
	)
}

// WorkerStarted logs a worker starting.
func (l *Logger) WorkerStarted(workerID string) {
	l.log.Info("Worker started",
		infralogger.String("worker_id", workerID),
	)
}

// WorkerStopped logs a worker stopping.
func (l *Logger) WorkerStopped(workerID string) {
	l.log.Info("Worker stopped",
		infralogger.String("worker_id", workerID),
	)
}

// WorkerPoolResized logs a worker pool resize.
func (l *Logger) WorkerPoolResized(oldSize, newSize int) {
	l.log.Info("Worker pool resized",
		infralogger.Int("old_size", oldSize),
		infralogger.Int("new_size", newSize),
	)
}

// QueueEnqueued logs a job being enqueued.
func (l *Logger) QueueEnqueued(ctx context.Context, jobID, priority, streamName string) {
	l.log.Debug("Job enqueued",
		infralogger.String("job_id", jobID),
		infralogger.String("priority", priority),
		infralogger.String("stream", streamName),
	)
}

// QueueDequeued logs a job being dequeued.
func (l *Logger) QueueDequeued(ctx context.Context, jobID, priority string) {
	l.log.Debug("Job dequeued",
		infralogger.String("job_id", jobID),
		infralogger.String("priority", priority),
	)
}

// TriggerFired logs a trigger being fired.
func (l *Logger) TriggerFired(ctx context.Context, triggerType, pattern string, matchedJobs int) {
	l.log.Info("Trigger fired",
		infralogger.String("trigger_type", triggerType),
		infralogger.String("pattern", pattern),
		infralogger.Int("matched_jobs", matchedJobs),
	)
}

// TriggerWebhookReceived logs a webhook being received.
func (l *Logger) TriggerWebhookReceived(ctx context.Context, path, source string) {
	l.log.Debug("Webhook received",
		infralogger.String("path", path),
		infralogger.String("source", source),
	)
}

// TriggerPubSubReceived logs a Pub/Sub message being received.
func (l *Logger) TriggerPubSubReceived(ctx context.Context, channel string) {
	l.log.Debug("PubSub message received",
		infralogger.String("channel", channel),
	)
}

// LeaderElected logs this instance becoming leader.
func (l *Logger) LeaderElected(instanceID string) {
	l.log.Info("Leader elected",
		infralogger.String("instance_id", instanceID),
	)
}

// LeaderLost logs this instance losing leadership.
func (l *Logger) LeaderLost(instanceID string) {
	l.log.Warn("Leader lost",
		infralogger.String("instance_id", instanceID),
	)
}

// LeaderRenewed logs a successful leader lease renewal.
func (l *Logger) LeaderRenewed(instanceID string, ttl time.Duration) {
	l.log.Debug("Leader renewed",
		infralogger.String("instance_id", instanceID),
		infralogger.Duration("ttl", ttl),
	)
}

// CircuitBreakerOpened logs a circuit breaker opening.
func (l *Logger) CircuitBreakerOpened(domain string, failures int) {
	l.log.Warn("Circuit breaker opened",
		infralogger.String("domain", domain),
		infralogger.Int("consecutive_failures", failures),
	)
}

// CircuitBreakerClosed logs a circuit breaker closing.
func (l *Logger) CircuitBreakerClosed(domain string) {
	l.log.Info("Circuit breaker closed",
		infralogger.String("domain", domain),
	)
}

// CircuitBreakerHalfOpen logs a circuit breaker entering half-open state.
func (l *Logger) CircuitBreakerHalfOpen(domain string) {
	l.log.Info("Circuit breaker half-open",
		infralogger.String("domain", domain),
	)
}

// SchedulerStarted logs the scheduler starting.
func (l *Logger) SchedulerStarted(workerPoolSize int, cronEnabled, eventsEnabled bool) {
	l.log.Info("Scheduler V2 started",
		infralogger.Int("worker_pool_size", workerPoolSize),
		infralogger.Bool("cron_enabled", cronEnabled),
		infralogger.Bool("events_enabled", eventsEnabled),
	)
}

// SchedulerStopped logs the scheduler stopping.
func (l *Logger) SchedulerStopped(graceful bool) {
	l.log.Info("Scheduler V2 stopped",
		infralogger.Bool("graceful", graceful),
	)
}

// SchedulerDraining logs the scheduler entering drain mode.
func (l *Logger) SchedulerDraining(activeJobs int) {
	l.log.Info("Scheduler draining",
		infralogger.Int("active_jobs", activeJobs),
	)
}

// Error logs a general error.
func (l *Logger) Error(msg string, err error, fields ...infralogger.Field) {
	allFields := append([]infralogger.Field{infralogger.Error(err)}, fields...)
	l.log.Error(msg, allFields...)
}

// Warn logs a warning.
func (l *Logger) Warn(msg string, fields ...infralogger.Field) {
	l.log.Warn(msg, fields...)
}

// Info logs an info message.
func (l *Logger) Info(msg string, fields ...infralogger.Field) {
	l.log.Info(msg, fields...)
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, fields ...infralogger.Field) {
	l.log.Debug(msg, fields...)
}
