// Package redis provides a Redis pub/sub publisher for alert lifecycle events.
// It wraps infrastructure/redis for connection management and publishes JSON
// payloads to a configured channel.
//
// # Failure semantics
//
// Redis publish failures are propagated to the caller but are not fatal to the
// pipeline. Elasticsearch is the canonical store; Redis is the live notification
// bus. When Publish returns an error the caller (runner) MUST increment the
// alert_crawler.redis.publish_failure_total metric and log at WARN. The ES write
// MUST NOT be rolled back. Consumers that miss a pub/sub notification MUST fall
// back to querying ES directly (NFR-004).
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	infraredis "github.com/jonesrussell/north-cloud/infrastructure/redis"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

// ErrNilContext is returned when Publish is called with a nil context.
var ErrNilContext = errors.New("redis publisher: context must not be nil")

// redisClient is the minimal interface required from the underlying Redis client.
// *redis.Client from infrastructure/redis satisfies this interface.
type redisClient interface {
	Publish(ctx context.Context, channel string, message any) error
	Close() error
}

// Publisher serializes domain.LifecycleEvent values and publishes them to a
// Redis pub/sub channel. Construct via New; do not copy after first use.
type Publisher struct {
	client  redisClient
	channel string
}

// Config holds the parameters required to construct a Publisher.
type Config struct {
	// Address is the Redis server address in "host:port" form.
	Address string
	// Password is the optional Redis authentication password.
	Password string
	// DB is the Redis database index (usually 0).
	DB int
	// Channel is the pub/sub channel name, e.g. "community_alerts:lifecycle".
	Channel string
}

// newWithClient constructs a Publisher from an already-connected redisClient.
// Used in tests to inject a mock without a real Redis connection.
func newWithClient(client redisClient, channel string) *Publisher {
	return &Publisher{client: client, channel: channel}
}

// New connects to Redis using cfg and returns a ready Publisher.
// The caller must call Close when the Publisher is no longer needed.
func New(cfg Config) (*Publisher, error) {
	client, err := infraredis.NewClient(infraredis.Config{
		Address:  cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err != nil {
		return nil, fmt.Errorf("redis publisher: connect: %w", err)
	}

	return &Publisher{
		client:  newPublishAdapter(client),
		channel: cfg.Channel,
	}, nil
}

// Publish serializes event to JSON and publishes it to the configured channel.
// If ctx is nil, ErrNilContext is returned immediately.
// Publish errors are propagated to the caller; the caller is responsible for
// metrics and logging. ES writes must not be rolled back on Publish failure.
func (p *Publisher) Publish(ctx context.Context, event domain.LifecycleEvent) error {
	if ctx == nil {
		return ErrNilContext
	}

	payload, marshalErr := json.Marshal(event)
	if marshalErr != nil {
		return fmt.Errorf("redis publisher: marshal lifecycle event: %w", marshalErr)
	}

	if publishErr := p.client.Publish(ctx, p.channel, payload); publishErr != nil {
		return fmt.Errorf("redis publisher: publish to %s: %w", p.channel, publishErr)
	}

	return nil
}

// Close releases the underlying Redis connection.
func (p *Publisher) Close() error {
	if closeErr := p.client.Close(); closeErr != nil {
		return fmt.Errorf("redis publisher: close: %w", closeErr)
	}

	return nil
}
