package logs_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

func TestNewService_WithRedis(t *testing.T) {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	cfg := logs.DefaultConfig()
	cfg.RedisEnabled = true

	redisWriter := logs.NewRedisStreamWriter(client, cfg.RedisKeyPrefix, cfg.RedisTTLSeconds)

	publisher := logs.NewNoopPublisher()
	logger := infralogger.NewNop()
	svc := logs.NewService(cfg, nil, publisher, nil, logger, logs.WithRedisWriter(redisWriter))
	if svc == nil {
		t.Fatal("NewService returned nil")
	}

	// Start capture should use Redis buffer
	jobID := "test-svc-" + time.Now().Format("20060102150405")
	streamKey := "logs:" + jobID
	defer client.Del(context.Background(), streamKey)

	writer, err := svc.StartCapture(context.Background(), jobID, "exec-1", 1)
	if err != nil {
		t.Fatalf("StartCapture failed: %v", err)
	}

	// Write a log
	writer.WriteEntry(logs.LogEntry{
		Level:   "info",
		Message: "Test from service",
		JobID:   jobID,
		ExecID:  "exec-1",
	})

	// Verify in Redis
	buf := svc.GetLiveBuffer(jobID)
	if buf == nil {
		t.Fatal("GetLiveBuffer returned nil")
	}

	entries := buf.ReadAll()
	if len(entries) < 1 {
		t.Errorf("expected at least 1 entry, got %d", len(entries))
	}

	// Clean up
	_ = writer.Close()
}

func TestNewService_WithoutRedis(t *testing.T) {
	t.Helper()

	cfg := logs.DefaultConfig()
	cfg.RedisEnabled = false

	publisher := logs.NewNoopPublisher()
	logger := infralogger.NewNop()
	svc := logs.NewService(cfg, nil, publisher, nil, logger)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}

	// Start capture should use in-memory buffer
	jobID := "test-svc-memory-" + time.Now().Format("20060102150405")

	writer, err := svc.StartCapture(context.Background(), jobID, "exec-1", 1)
	if err != nil {
		t.Fatalf("StartCapture failed: %v", err)
	}

	// Write a log
	writer.WriteEntry(logs.LogEntry{
		Level:   "info",
		Message: "Test from service",
		JobID:   jobID,
		ExecID:  "exec-1",
	})

	// Verify buffer exists
	buf := svc.GetLiveBuffer(jobID)
	if buf == nil {
		t.Fatal("GetLiveBuffer returned nil")
	}

	entries := buf.ReadAll()
	if len(entries) < 1 {
		t.Errorf("expected at least 1 entry, got %d", len(entries))
	}

	// Clean up
	_ = writer.Close()
}
