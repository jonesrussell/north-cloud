package redis

import (
	"context"
	"encoding/json"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
)

const (
	// ChannelDeliveryStatus is the Redis channel for delivery status events.
	ChannelDeliveryStatus = "social:delivery-status"
	// ChannelDeadLetter is the Redis channel for permanently failed deliveries.
	ChannelDeadLetter = "social:dead-letter"
)

// EventPublisher emits delivery lifecycle events to Redis.
type EventPublisher struct {
	client *goredis.Client
	log    logger.Logger
}

// NewEventPublisher creates a new event publisher.
func NewEventPublisher(client *goredis.Client, log logger.Logger) *EventPublisher {
	return &EventPublisher{client: client, log: log}
}

// PublishDeliveryEvent publishes a delivery status change event.
func (p *EventPublisher) PublishDeliveryEvent(ctx context.Context, event *domain.DeliveryEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling delivery event: %w", err)
	}
	return p.client.Publish(ctx, ChannelDeliveryStatus, data).Err()
}

// DeadLetterMessage wraps a failed delivery with its original content for diagnosis.
type DeadLetterMessage struct {
	Event            domain.DeliveryEvent  `json:"event"`
	Original         domain.PublishMessage `json:"original"`
	ErrorType        string                `json:"error_type"`
	PlatformResponse string                `json:"platform_response,omitempty"`
}

// PublishDeadLetter publishes a message to the dead-letter channel.
func (p *EventPublisher) PublishDeadLetter(ctx context.Context, msg *DeadLetterMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling dead-letter message: %w", err)
	}
	p.log.Warn("Publishing to dead-letter channel",
		logger.String("content_id", msg.Event.ContentID),
		logger.String("platform", msg.Event.Platform),
		logger.String("error_type", msg.ErrorType),
	)
	return p.client.Publish(ctx, ChannelDeadLetter, data).Err()
}
