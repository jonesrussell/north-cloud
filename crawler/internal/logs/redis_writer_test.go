package logs_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
)

func TestRedisStreamWriter_WriteEntry(t *testing.T) {
	t.Helper()

	// Skip if no Redis available
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	jobID := "test-job-" + time.Now().Format("20060102150405")
	streamKey := "logs:" + jobID

	// Clean up after test
	defer client.Del(context.Background(), streamKey)

	writer := logs.NewRedisStreamWriter(client, "logs", 3600)

	entry := logs.LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Category:  "crawler.lifecycle",
		Message:   "Test message",
		JobID:     jobID,
		ExecID:    "exec-1",
	}

	err := writer.WriteEntry(context.Background(), entry)
	if err != nil {
		t.Fatalf("WriteEntry failed: %v", err)
	}

	// Verify entry in Redis
	entries, readErr := client.XRange(context.Background(), streamKey, "-", "+").Result()
	if readErr != nil {
		t.Fatalf("XRange failed: %v", readErr)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Values["level"] != "info" {
		t.Errorf("level = %v, want %v", entries[0].Values["level"], "info")
	}
	if entries[0].Values["message"] != "Test message" {
		t.Errorf("message = %v, want %v", entries[0].Values["message"], "Test message")
	}
}

func TestRedisStreamWriter_ReadLast(t *testing.T) {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	jobID := "test-job-readlast-" + time.Now().Format("20060102150405")
	streamKey := "logs:" + jobID
	defer client.Del(context.Background(), streamKey)

	writer := logs.NewRedisStreamWriter(client, "logs", 3600)
	ctx := context.Background()

	// Write 5 entries
	messages := []string{"A", "B", "C", "D", "E"}
	for _, msg := range messages {
		entry := logs.LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   "Message " + msg,
			JobID:     jobID,
			ExecID:    "exec-1",
		}
		if err := writer.WriteEntry(ctx, entry); err != nil {
			t.Fatalf("WriteEntry failed: %v", err)
		}
	}

	// Read last 3
	entries, err := writer.ReadLast(ctx, jobID, 3)
	if err != nil {
		t.Fatalf("ReadLast failed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// Should be in chronological order: C, D, E
	if entries[0].Message != "Message C" {
		t.Errorf("entries[0].Message = %q, want %q", entries[0].Message, "Message C")
	}
	if entries[1].Message != "Message D" {
		t.Errorf("entries[1].Message = %q, want %q", entries[1].Message, "Message D")
	}
	if entries[2].Message != "Message E" {
		t.Errorf("entries[2].Message = %q, want %q", entries[2].Message, "Message E")
	}
}

func TestRedisStreamWriter_ReadAll(t *testing.T) {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	jobID := "test-job-readall-" + time.Now().Format("20060102150405")
	streamKey := "logs:" + jobID
	defer client.Del(context.Background(), streamKey)

	writer := logs.NewRedisStreamWriter(client, "logs", 3600)
	ctx := context.Background()

	// Write 3 entries
	entryCount := 3
	for range entryCount {
		entry := logs.LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   "Message",
			JobID:     jobID,
			ExecID:    "exec-1",
		}
		if err := writer.WriteEntry(ctx, entry); err != nil {
			t.Fatalf("WriteEntry failed: %v", err)
		}
	}

	entries, err := writer.ReadAll(ctx, jobID)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestRedisStreamWriter_Delete(t *testing.T) {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	jobID := "test-job-delete-" + time.Now().Format("20060102150405")
	streamKey := "logs:" + jobID

	writer := logs.NewRedisStreamWriter(client, "logs", 3600)
	ctx := context.Background()

	// Write an entry
	entry := logs.LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "To be deleted",
		JobID:     jobID,
		ExecID:    "exec-1",
	}
	if err := writer.WriteEntry(ctx, entry); err != nil {
		t.Fatalf("WriteEntry failed: %v", err)
	}

	// Verify it exists
	exists, _ := client.Exists(ctx, streamKey).Result()
	if exists != 1 {
		t.Fatal("stream should exist before delete")
	}

	// Delete
	if err := writer.Delete(ctx, jobID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	exists, _ = client.Exists(ctx, streamKey).Result()
	if exists != 0 {
		t.Error("stream should not exist after delete")
	}
}
