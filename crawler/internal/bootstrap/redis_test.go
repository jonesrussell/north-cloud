package bootstrap_test

import (
	"errors"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/bootstrap"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
)

func TestCreateRedisClient_NilConfig(t *testing.T) {
	t.Parallel()

	client, err := bootstrap.CreateRedisClient(nil)
	if client != nil {
		t.Error("expected nil client for nil config")
	}
	if !errors.Is(err, bootstrap.ErrRedisDisabledVar) {
		t.Errorf("expected ErrRedisDisabled, got %v", err)
	}
}

func TestCreateRedisClient_WithConfig(t *testing.T) {
	t.Parallel()

	// This will fail to connect but should not return ErrRedisDisabled
	cfg := &config.RedisConfig{
		Address:  "localhost:59999", // unlikely to have Redis here
		Password: "",
		DB:       0,
	}

	_, err := bootstrap.CreateRedisClient(cfg)
	// We expect either a connection error or success (if Redis happens to be running)
	// but NOT ErrRedisDisabled
	if errors.Is(err, bootstrap.ErrRedisDisabledVar) {
		t.Error("should not return ErrRedisDisabled when config is provided")
	}
}
