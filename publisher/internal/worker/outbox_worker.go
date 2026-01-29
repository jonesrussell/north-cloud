// Package worker provides background workers for the publisher service.
// outbox_worker.go implements the transactional outbox polling worker.
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

	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	"github.com/jonesrussell/north-cloud/publisher/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	defaultPollInterval   = 5 * time.Second
	defaultBatchSize      = 100
	defaultPublishTimeout = 10 * time.Second
	stalePublishingAge    = 5 * time.Minute
	cleanupRetention      = 7 * 24 * time.Hour // Keep published entries for 7 days
	retryBatchDivisor     = 2                  // Retry batch = batchSize / divisor
)

// OutboxWorker polls the outbox and publishes to Redis Pub/Sub
type OutboxWorker struct {
	repo   *database.OutboxRepository
	redis  *redis.Client
	logger infralogger.Logger
	tracer trace.Tracer

	pollInterval   time.Duration
	batchSize      int
	publishTimeout time.Duration

	stopChan chan struct{}
	wg       sync.WaitGroup
	started  bool
	mu       sync.Mutex
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
	redisClient *redis.Client,
	cfg OutboxWorkerConfig,
	logger infralogger.Logger,
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
		redis:          redisClient,
		logger:         logger,
		tracer:         otel.Tracer("outbox-worker"),
		pollInterval:   cfg.PollInterval,
		batchSize:      cfg.BatchSize,
		publishTimeout: cfg.PublishTimeout,
		stopChan:       make(chan struct{}),
	}
}

// Start begins the outbox polling loop
func (w *OutboxWorker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return
	}
	w.started = true
	w.mu.Unlock()

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
	w.mu.Lock()
	if !w.started {
		w.mu.Unlock()
		return
	}
	w.mu.Unlock()

	close(w.stopChan)
	w.wg.Wait()
	w.logger.Info("outbox worker stopped")
}

// IsRunning returns whether the worker is currently running
func (w *OutboxWorker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.started
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
		w.logger.Debug("processing pending entries", infralogger.Int("count", len(pending)))
		w.publishBatch(ctx, pending)
	}

	// Process retryable entries (reduced batch to prioritize new content)
	retryable, err := w.repo.FetchRetryable(ctx, w.batchSize/retryBatchDivisor)
	if err != nil {
		w.logger.Error("failed to fetch retryable outbox entries", infralogger.Error(err))
	} else if len(retryable) > 0 {
		w.logger.Debug("processing retryable entries", infralogger.Int("count", len(retryable)))
		w.publishBatch(ctx, retryable)
	}
}

func (w *OutboxWorker) publishBatch(ctx context.Context, entries []domain.OutboxEntry) {
	for i := range entries {
		w.publishOne(ctx, &entries[i])
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
	markErr := w.repo.MarkPublished(ctx, entry.ID)
	if markErr != nil {
		w.logger.Error("failed to mark outbox entry as published",
			infralogger.String("outbox_id", entry.ID),
			infralogger.Error(markErr))
		// Don't return error - message was published, just DB update failed
		// This is acceptable as the entry will eventually be cleaned up
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

// runRecovery resets stale "publishing" entries back to "pending".
// This handles entries that were claimed but worker crashed before completing.
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

// GetStats returns current worker statistics
func (w *OutboxWorker) GetStats(ctx context.Context) (map[string]any, error) {
	stats, err := w.repo.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"pending":                 stats.Pending,
		"publishing":              stats.Publishing,
		"published":               stats.Published,
		"failed_retryable":        stats.FailedRetryable,
		"failed_exhausted":        stats.FailedExhausted,
		"avg_publish_lag_seconds": stats.AvgPublishLagSeconds,
		"poll_interval":           w.pollInterval.String(),
		"batch_size":              w.batchSize,
		"running":                 w.IsRunning(),
	}, nil
}
