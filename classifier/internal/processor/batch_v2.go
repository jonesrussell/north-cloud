// Package processor provides content processing capabilities for the classifier.
// batch_v2.go implements a backpressure-controlled batch processor with bounded queues.
package processor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/telemetry"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	defaultMaxQueueDepth     = 500
	defaultSubmitTimeout     = 30 * time.Second
	workerDrainTimeout       = 30 * time.Second
	throttleThresholdRatio   = 0.8
	defaultWorkerConcurrency = 10
)

// ResultHandlerV2 is called for each processed item
type ResultHandlerV2 func(ctx context.Context, result *ProcessResult)

// BatchProcessorV2 processes raw content in parallel with backpressure control.
// It provides bounded queues, timeout-based submission, and panic recovery.
type BatchProcessorV2 struct {
	classifier    *classifier.Classifier
	concurrency   int
	maxQueueDepth int
	submitTimeout time.Duration
	logger        infralogger.Logger
	telemetry     *telemetry.Provider
	resultHandler ResultHandlerV2

	workQueue chan *domain.RawContent
	wg        sync.WaitGroup
	cancel    context.CancelFunc
	started   bool
	mu        sync.Mutex
}

// BatchProcessorV2Config holds configuration options
type BatchProcessorV2Config struct {
	Concurrency   int
	MaxQueueDepth int
	SubmitTimeout time.Duration
}

// DefaultBatchProcessorV2Config returns sensible defaults
func DefaultBatchProcessorV2Config() BatchProcessorV2Config {
	return BatchProcessorV2Config{
		Concurrency:   defaultWorkerConcurrency,
		MaxQueueDepth: defaultMaxQueueDepth,
		SubmitTimeout: defaultSubmitTimeout,
	}
}

// NewBatchProcessorV2 creates a processor with bounded queue and backpressure
func NewBatchProcessorV2(
	c *classifier.Classifier,
	cfg BatchProcessorV2Config,
	logger infralogger.Logger,
	tp *telemetry.Provider,
	handler ResultHandlerV2,
) *BatchProcessorV2 {
	if cfg.MaxQueueDepth <= 0 {
		cfg.MaxQueueDepth = defaultMaxQueueDepth
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = defaultWorkerConcurrency
	}
	if cfg.SubmitTimeout <= 0 {
		cfg.SubmitTimeout = defaultSubmitTimeout
	}

	return &BatchProcessorV2{
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
func (b *BatchProcessorV2) Start(ctx context.Context) {
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
	b.logger.Info("batch processor v2 started",
		infralogger.Int("workers", b.concurrency),
		infralogger.Int("max_queue_depth", b.maxQueueDepth))
}

// Stop gracefully shuts down workers
func (b *BatchProcessorV2) Stop() {
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
		b.logger.Info("batch processor v2 stopped gracefully")
	case <-time.After(workerDrainTimeout):
		b.logger.Warn("batch processor v2 stop timed out, some workers may not have finished",
			infralogger.Int("remaining", len(b.workQueue)))
	}
}

// Submit adds work to the queue with the configured timeout (backpressure)
func (b *BatchProcessorV2) Submit(ctx context.Context, item *domain.RawContent) error {
	return b.SubmitWithTimeout(ctx, item, b.submitTimeout)
}

// SubmitWithTimeout adds work with explicit timeout
func (b *BatchProcessorV2) SubmitWithTimeout(ctx context.Context, item *domain.RawContent, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if b.telemetry != nil {
		b.telemetry.SetQueueDepth(len(b.workQueue) + 1)
	}

	select {
	case b.workQueue <- item:
		return nil
	case <-ctx.Done():
		if b.telemetry != nil {
			b.telemetry.IncrementWorkDropped()
		}
		return fmt.Errorf("queue full after %v: %w", timeout, ctx.Err())
	}
}

// QueueDepth returns current queue size for monitoring
func (b *BatchProcessorV2) QueueDepth() int {
	return len(b.workQueue)
}

// ShouldThrottle returns true if queue is near capacity (80%)
func (b *BatchProcessorV2) ShouldThrottle() bool {
	depth := len(b.workQueue)
	threshold := int(float64(b.maxQueueDepth) * throttleThresholdRatio)
	return depth > threshold
}

// IsStarted returns whether the processor is running
func (b *BatchProcessorV2) IsStarted() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.started
}

func (b *BatchProcessorV2) worker(ctx context.Context, id int) {
	defer func() {
		if r := recover(); r != nil {
			b.logger.Error("worker panic recovered",
				infralogger.Int("worker_id", id),
				infralogger.Any("panic", r))
		}
		b.wg.Done()
	}()

	if b.telemetry != nil {
		b.telemetry.SetActiveWorkers(b.concurrency)
	}

	for {
		select {
		case item, ok := <-b.workQueue:
			if !ok {
				return // Channel closed, shutdown
			}
			result := b.processItem(ctx, id, item)
			if b.telemetry != nil {
				b.telemetry.SetQueueDepth(len(b.workQueue))
			}
			if b.resultHandler != nil {
				b.resultHandler(ctx, result)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (b *BatchProcessorV2) processItem(ctx context.Context, workerID int, item *domain.RawContent) *ProcessResult {
	start := time.Now()

	// Record poller lag (time from creation to processing)
	if b.telemetry != nil && !item.CrawledAt.IsZero() {
		b.telemetry.RecordPollerLag(ctx, item.CrawledAt)
	}

	result := &ProcessResult{
		Raw: item,
	}

	classificationResult, err := b.classifier.Classify(ctx, item)
	duration := time.Since(start)

	if err != nil {
		result.Error = fmt.Errorf("classification failed: %w", err)

		// Record failure metrics
		if b.telemetry != nil {
			errorCode := string(domain.ClassifyError(err))
			b.telemetry.RecordClassificationFailure(ctx, item.SourceName, errorCode)
		}

		b.logger.Error("classification failed",
			infralogger.Int("worker_id", workerID),
			infralogger.String("content_id", item.ID),
			infralogger.String("source", item.SourceName),
			infralogger.Duration("duration", duration),
			infralogger.Error(err))

		return result
	}

	result.ClassificationResult = classificationResult
	result.ClassifiedContent = b.classifier.BuildClassifiedContent(item, classificationResult)

	// Record success metrics
	if b.telemetry != nil {
		b.telemetry.RecordClassification(ctx, item.SourceName, true, duration)
		if !item.CrawledAt.IsZero() {
			b.telemetry.RecordClassificationLag(ctx, item.CrawledAt)
		}
	}

	return result
}

// GetStats returns statistics about the batch processor
func (b *BatchProcessorV2) GetStats() map[string]any {
	return map[string]any{
		"concurrency":     b.concurrency,
		"max_queue_depth": b.maxQueueDepth,
		"queue_depth":     len(b.workQueue),
		"submit_timeout":  b.submitTimeout.String(),
		"started":         b.started,
		"throttling":      b.ShouldThrottle(),
	}
}

// SetResultHandler sets the callback for processed items
func (b *BatchProcessorV2) SetResultHandler(handler ResultHandlerV2) {
	b.resultHandler = handler
}
