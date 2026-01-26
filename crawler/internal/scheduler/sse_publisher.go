package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/sse"
)

// Progress tracking constants.
const (
	progressItemThreshold    = 10
	progressIntervalDuration = 5 * time.Second
)

// SSEPublisher wraps an SSE broker for scheduler events.
type SSEPublisher struct {
	broker   sse.Broker
	logger   infralogger.Logger
	disabled atomic.Bool
}

// NewSSEPublisher creates a new SSE publisher for scheduler events.
func NewSSEPublisher(broker sse.Broker, logger infralogger.Logger) *SSEPublisher {
	return &SSEPublisher{
		broker: broker,
		logger: logger,
	}
}

// Disable disables event publishing (useful for testing).
func (p *SSEPublisher) Disable() {
	p.disabled.Store(true)
}

// Enable enables event publishing.
func (p *SSEPublisher) Enable() {
	p.disabled.Store(false)
}

// IsEnabled returns true if publishing is enabled.
func (p *SSEPublisher) IsEnabled() bool {
	return !p.disabled.Load()
}

// PublishJobStatus publishes a job status change event.
func (p *SSEPublisher) PublishJobStatus(ctx context.Context, job *domain.Job, details *sse.JobStatusDetails) {
	if p.disabled.Load() {
		return
	}

	event := sse.NewJobStatusEvent(job.ID, job.Status, details)

	if err := p.broker.Publish(ctx, event); err != nil {
		p.logger.Warn("Failed to publish job status event",
			infralogger.Error(err),
			infralogger.String("job_id", job.ID),
			infralogger.String("status", job.Status),
		)
	}
}

// PublishJobProgress publishes job progress during execution.
func (p *SSEPublisher) PublishJobProgress(ctx context.Context, job *domain.Job, execution *domain.JobExecution, itemsCrawled, itemsIndexed int) {
	if p.disabled.Load() {
		return
	}

	event := sse.NewJobProgressEvent(job.ID, execution.ID, itemsCrawled, itemsIndexed)

	if err := p.broker.Publish(ctx, event); err != nil {
		// Don't log at warning level - progress events are best-effort
		p.logger.Debug("Failed to publish job progress event",
			infralogger.Error(err),
			infralogger.String("job_id", job.ID),
		)
	}
}

// PublishJobCompleted publishes a job completion event.
func (p *SSEPublisher) PublishJobCompleted(ctx context.Context, job *domain.Job, execution *domain.JobExecution) {
	if p.disabled.Load() {
		return
	}

	status := "completed"
	if execution.Status == "failed" {
		status = "failed"
	}

	durationMs := int64(0)
	if execution.DurationMs != nil {
		durationMs = *execution.DurationMs
	}

	var errorMessage *string
	if execution.ErrorMessage != nil {
		errorMessage = execution.ErrorMessage
	}

	event := sse.NewJobCompletedEvent(job.ID, execution.ID, status, durationMs, execution.ItemsIndexed, errorMessage)

	if err := p.broker.Publish(ctx, event); err != nil {
		p.logger.Warn("Failed to publish job completed event",
			infralogger.Error(err),
			infralogger.String("job_id", job.ID),
		)
	}
}

// ProgressTracker tracks job progress and emits events at configured intervals.
type ProgressTracker struct {
	publisher     *SSEPublisher
	job           *domain.Job
	execution     *domain.JobExecution
	ctx           context.Context
	lastEmitTime  time.Time
	lastItemCount int
	mu            sync.Mutex
}

// NewProgressTracker creates a new progress tracker for a job execution.
func NewProgressTracker(ctx context.Context, publisher *SSEPublisher, job *domain.Job, execution *domain.JobExecution) *ProgressTracker {
	return &ProgressTracker{
		publisher:     publisher,
		job:           job,
		execution:     execution,
		ctx:           ctx,
		lastEmitTime:  time.Now(),
		lastItemCount: 0,
	}
}

// Update updates progress and emits an event if thresholds are met.
// Returns true if an event was emitted.
func (pt *ProgressTracker) Update(itemsCrawled, itemsIndexed int) bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.publisher == nil || pt.publisher.disabled.Load() {
		return false
	}

	itemDelta := itemsCrawled - pt.lastItemCount
	timeSinceLastEmit := time.Since(pt.lastEmitTime)

	// Emit if either threshold is met
	shouldEmit := itemDelta >= progressItemThreshold || timeSinceLastEmit >= progressIntervalDuration

	if shouldEmit {
		pt.publisher.PublishJobProgress(pt.ctx, pt.job, pt.execution, itemsCrawled, itemsIndexed)
		pt.lastEmitTime = time.Now()
		pt.lastItemCount = itemsCrawled
		return true
	}

	return false
}

// Flush emits final progress regardless of thresholds.
func (pt *ProgressTracker) Flush(itemsCrawled, itemsIndexed int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.publisher == nil || pt.publisher.disabled.Load() {
		return
	}

	// Only flush if there's actual progress since last emit
	if itemsCrawled > pt.lastItemCount {
		pt.publisher.PublishJobProgress(pt.ctx, pt.job, pt.execution, itemsCrawled, itemsIndexed)
	}
}
