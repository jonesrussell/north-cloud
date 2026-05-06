// Package redis exports internal symbols for use by the redis_test package.
// This file is compiled only during testing.
package redis

// NewWithClient constructs a Publisher from an injected client and channel.
// Used in tests to bypass New() without a real Redis connection.
func NewWithClient(client RedisClient, channel string) *Publisher {
	return newWithClient(client, channel)
}

// RedisClient is the exported alias of the internal redisClient interface so
// that test packages can implement it without importing go-redis directly.
type RedisClient = redisClient
