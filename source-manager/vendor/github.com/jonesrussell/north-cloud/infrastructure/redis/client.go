package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config holds Redis connection configuration.
type Config struct {
	Address  string `default:"localhost:6379" env:"REDIS_ADDRESS"`
	Password string `default:""               env:"REDIS_PASSWORD"`
	DB       int    `default:"0"              env:"REDIS_DB"`
}

// ErrEmptyAddress is returned when Redis address is not configured.
var ErrEmptyAddress = errors.New("redis address is required")

// connectionTimeout is the timeout for verifying Redis connection.
const connectionTimeout = 5 * time.Second

// NewClient creates a new Redis client with the given configuration.
func NewClient(cfg Config) (*redis.Client, error) {
	if cfg.Address == "" {
		return nil, ErrEmptyAddress
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return client, nil
}
