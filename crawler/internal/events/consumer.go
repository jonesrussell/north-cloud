package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	infraevents "github.com/north-cloud/infrastructure/events"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/redis/go-redis/v9"
)

const (
	blockDuration    = 5 * time.Second
	claimIdleTimeout = 30 * time.Second
	batchSize        = 10
)

// Consumer reads events from Redis Streams.
type Consumer struct {
	client     *redis.Client
	consumerID string
	handler    EventHandler
	log        infralogger.Logger
	shutdownCh chan struct{}
}

// NewConsumer creates a new event consumer.
// Returns nil if client is nil.
func NewConsumer(client *redis.Client, consumerID string, handler EventHandler, log infralogger.Logger) *Consumer {
	if client == nil {
		return nil
	}
	if consumerID == "" {
		consumerID = generateConsumerID()
	}
	return &Consumer{
		client:     client,
		consumerID: consumerID,
		handler:    handler,
		log:        log,
		shutdownCh: make(chan struct{}),
	}
}

// generateConsumerID creates a unique consumer identifier.
func generateConsumerID() string {
	const uuidPrefixLength = 8
	return fmt.Sprintf("crawler-%s", uuid.New().String()[:uuidPrefixLength])
}

// Start begins consuming events from the stream.
func (c *Consumer) Start(ctx context.Context) error {
	if err := c.ensureConsumerGroup(ctx); err != nil {
		return fmt.Errorf("ensure consumer group: %w", err)
	}

	if c.log != nil {
		c.log.Info("Starting event consumer",
			infralogger.String("consumer_id", c.consumerID),
			infralogger.String("group", infraevents.ConsumerGroup),
		)
	}

	go c.consumeLoop(ctx)
	go c.claimAbandonedLoop(ctx)

	return nil
}

// Stop gracefully shuts down the consumer.
func (c *Consumer) Stop() {
	close(c.shutdownCh)
}

func (c *Consumer) consumeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.shutdownCh:
			return
		default:
			c.readAndProcess(ctx)
		}
	}
}

func (c *Consumer) readAndProcess(ctx context.Context) {
	streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    infraevents.ConsumerGroup,
		Consumer: c.consumerID,
		Streams:  []string{infraevents.StreamName, ">"},
		Count:    batchSize,
		Block:    blockDuration,
	}).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return
		}
		if c.log != nil {
			c.log.Error("Failed to read from stream", infralogger.Error(err))
		}
		time.Sleep(time.Second)
		return
	}

	for _, stream := range streams {
		for _, msg := range stream.Messages {
			c.processMessage(ctx, msg)
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context, msg redis.XMessage) {
	eventData, ok := msg.Values["event"].(string)
	if !ok {
		if c.log != nil {
			c.log.Error("Invalid message format", infralogger.String("stream_id", msg.ID))
		}
		c.ackMessage(ctx, msg.ID)
		return
	}

	var event infraevents.SourceEvent
	if err := json.Unmarshal([]byte(eventData), &event); err != nil {
		if c.log != nil {
			c.log.Error("Failed to unmarshal event",
				infralogger.String("stream_id", msg.ID),
				infralogger.Error(err),
			)
		}
		c.ackMessage(ctx, msg.ID)
		return
	}

	var err error
	switch event.EventType {
	case infraevents.SourceCreated:
		err = c.handler.HandleSourceCreated(ctx, event)
	case infraevents.SourceUpdated:
		err = c.handler.HandleSourceUpdated(ctx, event)
	case infraevents.SourceDeleted:
		err = c.handler.HandleSourceDeleted(ctx, event)
	case infraevents.SourceEnabled:
		err = c.handler.HandleSourceEnabled(ctx, event)
	case infraevents.SourceDisabled:
		err = c.handler.HandleSourceDisabled(ctx, event)
	default:
		if c.log != nil {
			c.log.Warn("Unknown event type",
				infralogger.String("event_type", string(event.EventType)),
			)
		}
	}

	if err != nil {
		if c.log != nil {
			c.log.Error("Failed to handle event",
				infralogger.String("event_type", string(event.EventType)),
				infralogger.String("source_id", event.SourceID.String()),
				infralogger.Error(err),
			)
		}
		return // Don't ACK - will be retried
	}

	c.ackMessage(ctx, msg.ID)

	if c.log != nil {
		c.log.Info("Processed event",
			infralogger.String("event_type", string(event.EventType)),
			infralogger.String("source_id", event.SourceID.String()),
			infralogger.String("stream_id", msg.ID),
		)
	}
}

func (c *Consumer) ackMessage(ctx context.Context, streamID string) {
	if err := c.client.XAck(ctx, infraevents.StreamName, infraevents.ConsumerGroup, streamID).Err(); err != nil {
		if c.log != nil {
			c.log.Error("Failed to ACK message",
				infralogger.String("stream_id", streamID),
				infralogger.Error(err),
			)
		}
	}
}

func (c *Consumer) claimAbandonedLoop(ctx context.Context) {
	ticker := time.NewTicker(claimIdleTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.shutdownCh:
			return
		case <-ticker.C:
			c.claimAbandonedMessages(ctx)
		}
	}
}

func (c *Consumer) claimAbandonedMessages(ctx context.Context) {
	messages, _, err := c.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   infraevents.StreamName,
		Group:    infraevents.ConsumerGroup,
		Consumer: c.consumerID,
		MinIdle:  claimIdleTimeout,
		Count:    batchSize,
	}).Result()

	if err != nil {
		if c.log != nil {
			c.log.Error("Failed to auto-claim messages", infralogger.Error(err))
		}
		return
	}

	for _, msg := range messages {
		if c.log != nil {
			c.log.Info("Claimed abandoned message", infralogger.String("stream_id", msg.ID))
		}
		c.processMessage(ctx, msg)
	}
}

func (c *Consumer) ensureConsumerGroup(ctx context.Context) error {
	err := c.client.XGroupCreateMkStream(ctx, infraevents.StreamName, infraevents.ConsumerGroup, "0").Err()
	if err != nil && !isGroupExistsError(err) {
		return err
	}
	return nil
}

func isGroupExistsError(err error) bool {
	return err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists"
}
