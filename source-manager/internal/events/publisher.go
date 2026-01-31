// Package events provides event publishing for source lifecycle events.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	infraevents "github.com/north-cloud/infrastructure/events"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// asyncPublishTimeout is the context timeout for async publish operations.
const asyncPublishTimeout = 5 * time.Second

// Publisher publishes source events to Redis Streams.
type Publisher struct {
	client *redis.Client
	log    infralogger.Logger
}

// NewPublisher creates a new event publisher.
// Returns nil if client is nil.
func NewPublisher(client *redis.Client, log infralogger.Logger) *Publisher {
	if client == nil {
		return nil
	}
	return &Publisher{
		client: client,
		log:    log,
	}
}

// Publish sends an event to the Redis stream.
func (p *Publisher) Publish(ctx context.Context, event infraevents.SourceEvent) error {
	if p == nil || p.client == nil {
		return nil // No-op if publisher not configured
	}

	// Ensure event has ID and timestamp
	if event.EventID == uuid.Nil {
		event.EventID = uuid.New()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	result := p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: infraevents.StreamName,
		Values: map[string]any{
			"event": string(payload),
		},
	})

	if publishErr := result.Err(); publishErr != nil {
		if p.log != nil {
			p.log.Error("Failed to publish event",
				infralogger.String("event_type", string(event.EventType)),
				infralogger.String("source_id", event.SourceID.String()),
				infralogger.Error(publishErr),
			)
		}
		return fmt.Errorf("publish to stream: %w", publishErr)
	}

	if p.log != nil {
		p.log.Info("Published source event",
			infralogger.String("event_type", string(event.EventType)),
			infralogger.String("source_id", event.SourceID.String()),
			infralogger.String("stream_id", result.Val()),
		)
	}

	return nil
}

// PublishAsync publishes an event asynchronously.
// Errors are logged but not returned.
func (p *Publisher) PublishAsync(event infraevents.SourceEvent) {
	if p == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), asyncPublishTimeout)
		defer cancel()

		if err := p.Publish(ctx, event); err != nil && p.log != nil {
			p.log.Error("Async publish failed",
				infralogger.String("event_type", string(event.EventType)),
				infralogger.String("source_id", event.SourceID.String()),
				infralogger.Error(err),
			)
		}
	}()
}
