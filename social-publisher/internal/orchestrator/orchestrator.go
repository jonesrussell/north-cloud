package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
)

var backoffs = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
}

// NextRetryAt returns the time for the next retry given the current attempt count.
// Returns false if max retries have been exhausted.
func NextRetryAt(attempts int) (time.Time, bool) {
	if attempts >= len(backoffs) {
		return time.Time{}, false
	}
	return time.Now().Add(backoffs[attempts]), true
}

// EventPublisher emits delivery lifecycle events.
type EventPublisher interface {
	PublishDeliveryEvent(ctx context.Context, event *domain.DeliveryEvent) error
}

// ContentRepository provides delivery persistence operations needed by the orchestrator.
type ContentRepository interface {
	UpdateDeliveryStatus(ctx context.Context, id string, status domain.DeliveryStatus,
		result *domain.DeliveryResult, errMsg *string) error
	IncrementAttempts(ctx context.Context, id string, nextRetryAt time.Time) error
	MarkDeliveryFailed(ctx context.Context, id string, errMsg string) error
}

// Orchestrator dispatches publish jobs to the appropriate platform adapter.
type Orchestrator struct {
	adapters map[string]domain.PlatformAdapter
	events   EventPublisher
	repo     ContentRepository
}

// NewOrchestrator creates an orchestrator with the given adapters, event publisher, and repository.
func NewOrchestrator(
	adapters map[string]domain.PlatformAdapter,
	events EventPublisher,
	repo ContentRepository,
) *Orchestrator {
	return &Orchestrator{
		adapters: adapters,
		events:   events,
		repo:     repo,
	}
}

// ProcessJob transforms, validates, and publishes content to the given platform.
func (o *Orchestrator) ProcessJob(
	ctx context.Context, platform string, msg *domain.PublishMessage,
) (domain.DeliveryResult, error) {
	adapter, ok := o.adapters[platform]
	if !ok {
		return domain.DeliveryResult{}, fmt.Errorf("unknown platform: %s", platform)
	}

	post, err := adapter.Transform(*msg)
	if err != nil {
		return domain.DeliveryResult{}, err
	}

	if err := adapter.Validate(post); err != nil {
		return domain.DeliveryResult{}, err
	}

	result, err := adapter.Publish(ctx, post)
	if err != nil {
		return domain.DeliveryResult{}, err
	}

	return result, nil
}

// GetAdapter returns the adapter for a platform, if registered.
func (o *Orchestrator) GetAdapter(platform string) (domain.PlatformAdapter, bool) {
	a, ok := o.adapters[platform]
	return a, ok
}
