package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// publishAdapter wraps *redis.Client to satisfy the redisClient interface.
// go-redis Publish returns *IntCmd; this adapter extracts the error.
type publishAdapter struct {
	c *redis.Client
}

// newPublishAdapter wraps a *redis.Client so it satisfies redisClient.
func newPublishAdapter(c *redis.Client) redisClient {
	return &publishAdapter{c: c}
}

// Publish delegates to the underlying go-redis Publish and extracts the error.
func (a *publishAdapter) Publish(ctx context.Context, channel string, message any) error {
	return a.c.Publish(ctx, channel, message).Err()
}

// Close delegates to the underlying go-redis client.
func (a *publishAdapter) Close() error {
	return a.c.Close()
}
