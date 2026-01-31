package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/north-cloud/infrastructure/redis"
)

func TestNewClient_ReturnsNilWhenAddressEmpty(t *testing.T) {
	client, err := redis.NewClient(redis.Config{Address: ""})

	if err == nil {
		t.Error("expected error for empty address")
	}
	if client != nil {
		t.Error("expected nil client for invalid config")
	}
}

func TestNewClient_ConnectsToRedis(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client, err := redis.NewClient(redis.Config{
		Address:  "localhost:6379",
		Password: "",
		DB:       0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pingErr := client.Ping(ctx).Err()
	if pingErr != nil {
		t.Errorf("ping failed: %v", pingErr)
	}
}
