package redis

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	connectionTimeout = 2 * time.Second
)

// NewClient creates a new Redis client with connection testing
func NewClient(addr, password string) (*redis.Client, error) {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})

	// Test Redis connection with timeout
	pingCtx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	if pingErr := redisClient.Ping(pingCtx).Err(); pingErr != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", pingErr)
	}

	log.Println("Redis connection established")
	return redisClient, nil
}

// CheckConnection tests if Redis is reachable
func CheckConnection(client *redis.Client) (bool, error) {
	if client == nil {
		return false, errors.New("redis client is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	err := client.Ping(ctx).Err()
	if err != nil {
		return false, err
	}

	return true, nil
}
