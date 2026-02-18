// Package queue provides Redis Streams-based job queue functionality.
package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Default connection timeout for Redis operations.
	defaultConnectionTimeout = 2 * time.Second
)

// StreamsClient wraps a Redis client with streams-specific operations.
type StreamsClient struct {
	client *redis.Client
	prefix string
}

// StreamsConfig holds configuration for the Redis Streams client.
type StreamsConfig struct {
	Addr     string
	Password string `json:"-"`
	DB       int
	Prefix   string // Stream key prefix (e.g., "crawler")
}

// NewStreamsClient creates a new Redis Streams client.
func NewStreamsClient(cfg StreamsConfig) (*StreamsClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), defaultConnectionTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "crawler"
	}

	return &StreamsClient{
		client: client,
		prefix: prefix,
	}, nil
}

// NewStreamsClientFromRedis creates a StreamsClient from an existing Redis client.
func NewStreamsClientFromRedis(client *redis.Client, prefix string) *StreamsClient {
	if prefix == "" {
		prefix = "crawler"
	}
	return &StreamsClient{
		client: client,
		prefix: prefix,
	}
}

// StreamName returns the full stream name for a priority level.
func (c *StreamsClient) StreamName(priority Priority) string {
	return fmt.Sprintf("%s:jobs:%s", c.prefix, priority.String())
}

// Close closes the underlying Redis client.
func (c *StreamsClient) Close() error {
	return c.client.Close()
}

// Ping checks if Redis is reachable.
func (c *StreamsClient) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Client returns the underlying Redis client for advanced operations.
func (c *StreamsClient) Client() *redis.Client {
	return c.client
}

// CreateConsumerGroup creates a consumer group for a stream if it doesn't exist.
func (c *StreamsClient) CreateConsumerGroup(ctx context.Context, stream, group string) error {
	// Try to create the group starting from the beginning of the stream
	err := c.client.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}
	return nil
}

// XAdd adds a message to a stream.
func (c *StreamsClient) XAdd(ctx context.Context, stream string, values map[string]any) (string, error) {
	result := c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: values,
	})
	return result.Result()
}

// XReadGroup reads messages from a stream using a consumer group.
func (c *StreamsClient) XReadGroup(
	ctx context.Context, group, consumer string, streams []string, count int64, block time.Duration,
) ([]redis.XStream, error) {
	result := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  streams,
		Count:    count,
		Block:    block,
	})
	return result.Result()
}

// XAck acknowledges messages in a stream.
func (c *StreamsClient) XAck(ctx context.Context, stream, group string, ids ...string) error {
	return c.client.XAck(ctx, stream, group, ids...).Err()
}

// XPending returns pending entries summary for a stream.
func (c *StreamsClient) XPending(ctx context.Context, stream, group string) (*redis.XPending, error) {
	return c.client.XPending(ctx, stream, group).Result()
}

// XPendingExt returns detailed pending entries for a stream.
func (c *StreamsClient) XPendingExt(
	ctx context.Context, stream, group, start, end string, count int64,
) ([]redis.XPendingExt, error) {
	return c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Start:  start,
		End:    end,
		Count:  count,
	}).Result()
}

// XClaim claims pending messages for a consumer.
func (c *StreamsClient) XClaim(ctx context.Context, stream, group, consumer string, minIdle time.Duration, ids ...string) ([]redis.XMessage, error) {
	return c.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   stream,
		Group:    group,
		Consumer: consumer,
		MinIdle:  minIdle,
		Messages: ids,
	}).Result()
}

// XLen returns the length of a stream.
func (c *StreamsClient) XLen(ctx context.Context, stream string) (int64, error) {
	return c.client.XLen(ctx, stream).Result()
}

// XInfoGroups returns information about consumer groups for a stream.
func (c *StreamsClient) XInfoGroups(ctx context.Context, stream string) ([]redis.XInfoGroup, error) {
	return c.client.XInfoGroups(ctx, stream).Result()
}

// XInfoConsumers returns information about consumers in a group.
func (c *StreamsClient) XInfoConsumers(ctx context.Context, stream, group string) ([]redis.XInfoConsumer, error) {
	return c.client.XInfoConsumers(ctx, stream, group).Result()
}

// XTrimMaxLen trims a stream to a maximum length.
func (c *StreamsClient) XTrimMaxLen(ctx context.Context, stream string, maxLen int64) error {
	return c.client.XTrimMaxLen(ctx, stream, maxLen).Err()
}
