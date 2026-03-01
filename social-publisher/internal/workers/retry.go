package workers

import (
	"context"
	"time"

	"github.com/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/database"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/orchestrator"
)

// RetryWorker polls the database for deliveries due for retry and reprocesses them.
type RetryWorker struct {
	repo      *database.Repository
	orch      *orchestrator.Orchestrator
	events    orchestrator.EventPublisher
	log       logger.Logger
	interval  time.Duration
	batchSize int
}

// NewRetryWorker creates a new retry worker.
func NewRetryWorker(
	repo *database.Repository,
	orch *orchestrator.Orchestrator,
	events orchestrator.EventPublisher,
	log logger.Logger,
	interval time.Duration,
	batchSize int,
) *RetryWorker {
	return &RetryWorker{
		repo:      repo,
		orch:      orch,
		events:    events,
		log:       log,
		interval:  interval,
		batchSize: batchSize,
	}
}

// Run starts the retry polling loop until ctx is cancelled.
func (w *RetryWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.log.Info("Retry worker started", logger.Duration("interval", w.interval))

	for {
		select {
		case <-ctx.Done():
			w.log.Info("Retry worker shutting down")
			return
		case <-ticker.C:
			w.processRetries(ctx)
		}
	}
}

func (w *RetryWorker) processRetries(ctx context.Context) {
	deliveries, err := w.repo.GetDueRetries(ctx, w.batchSize)
	if err != nil {
		w.log.Error("Failed to fetch due retries", logger.Error(err))
		return
	}

	if len(deliveries) == 0 {
		return
	}

	w.log.Info("Processing retries", logger.Int("count", len(deliveries)))

	for _, delivery := range deliveries {
		w.processRetry(ctx, &delivery)
	}
}

func (w *RetryWorker) processRetry(ctx context.Context, delivery *domain.Delivery) {
	w.emitEvent(ctx, delivery, string(domain.StatusPublishing), nil)

	msg := &domain.PublishMessage{ContentID: delivery.ContentID}

	result, err := w.orch.ProcessJob(ctx, delivery.Platform, msg)
	if err != nil {
		w.handleRetryError(ctx, delivery, err)
		return
	}

	if updateErr := w.repo.UpdateDeliveryStatus(
		ctx, delivery.ID, domain.StatusDelivered, &result, nil,
	); updateErr != nil {
		w.log.Error("Failed to update delivery status", logger.Error(updateErr))
	}
	w.emitEvent(ctx, delivery, string(domain.StatusDelivered), nil)

	w.log.Info("Retry succeeded",
		logger.String("delivery_id", delivery.ID),
		logger.String("platform", delivery.Platform),
		logger.Int("attempts", delivery.Attempts),
	)
}

func (w *RetryWorker) handleRetryError(ctx context.Context, delivery *domain.Delivery, err error) {
	errMsg := err.Error()

	if !isRetryable(err) {
		w.failDelivery(ctx, delivery, errMsg)
		return
	}

	nextRetry, ok := orchestrator.NextRetryAt(delivery.Attempts)
	if !ok {
		w.failDelivery(ctx, delivery, errMsg)
		return
	}

	if rle, isRateLimit := err.(*domain.RateLimitError); isRateLimit {
		nextRetry = time.Now().Add(rle.RetryAfter)
	}

	if incErr := w.repo.IncrementAttempts(ctx, delivery.ID, nextRetry); incErr != nil {
		w.log.Error("Failed to increment retry attempts", logger.Error(incErr))
	}

	w.log.Warn("Retry failed, scheduling next attempt",
		logger.String("delivery_id", delivery.ID),
		logger.String("platform", delivery.Platform),
		logger.Int("attempts", delivery.Attempts+1),
		logger.String("next_retry", nextRetry.Format(time.RFC3339)),
	)
}

func (w *RetryWorker) failDelivery(ctx context.Context, delivery *domain.Delivery, errMsg string) {
	if failErr := w.repo.MarkDeliveryFailed(ctx, delivery.ID, errMsg); failErr != nil {
		w.log.Error("Failed to mark delivery as failed", logger.Error(failErr))
	}
	w.emitEvent(ctx, delivery, string(domain.StatusFailed), &errMsg)
}

func (w *RetryWorker) emitEvent(
	ctx context.Context, delivery *domain.Delivery, status string, errMsg *string,
) {
	if w.events == nil {
		return
	}
	event := &domain.DeliveryEvent{
		ContentID:  delivery.ContentID,
		DeliveryID: delivery.ID,
		Platform:   delivery.Platform,
		Account:    delivery.Account,
		Status:     status,
		Attempts:   delivery.Attempts,
		Timestamp:  time.Now(),
	}
	if errMsg != nil {
		event.Error = *errMsg
	}
	if pubErr := w.events.PublishDeliveryEvent(ctx, event); pubErr != nil {
		w.log.Error("Failed to emit delivery event", logger.Error(pubErr))
	}
}

func isRetryable(err error) bool {
	pubErr, ok := err.(domain.PublishError)
	return ok && pubErr.IsRetryable()
}
