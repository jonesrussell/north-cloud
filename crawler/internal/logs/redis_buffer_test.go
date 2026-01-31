package logs_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
)

func TestRedisBuffer_ImplementsBuffer(t *testing.T) {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	jobID := "test-buffer-" + time.Now().Format("20060102150405")
	writer := logs.NewRedisStreamWriter(client, "logs", 3600)

	// Verify it implements Buffer interface
	var buf logs.Buffer = logs.NewRedisBuffer(writer, jobID)
	_ = buf // Compile-time check that RedisBuffer implements Buffer
}

func TestRedisBuffer_WriteAndReadLast(t *testing.T) {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	jobID := "test-buffer-rw-" + time.Now().Format("20060102150405")
	streamKey := "logs:" + jobID
	defer client.Del(context.Background(), streamKey)

	writer := logs.NewRedisStreamWriter(client, "logs", 3600)
	buf := logs.NewRedisBuffer(writer, jobID)

	// Write 10 entries
	for i := range 10 {
		buf.Write(logs.LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   "Message " + string(rune('0'+i)),
			JobID:     jobID,
			ExecID:    "exec-1",
		})
	}

	// Read last 5
	entries := buf.ReadLast(5)
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}
	if entries[0].Message != "Message 5" {
		t.Errorf("entries[0].Message = %q, want %q", entries[0].Message, "Message 5")
	}
	if entries[4].Message != "Message 9" {
		t.Errorf("entries[4].Message = %q, want %q", entries[4].Message, "Message 9")
	}
}

func TestRedisBuffer_ReadAll(t *testing.T) {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	jobID := "test-buffer-all-" + time.Now().Format("20060102150405")
	streamKey := "logs:" + jobID
	defer client.Del(context.Background(), streamKey)

	writer := logs.NewRedisStreamWriter(client, "logs", 3600)
	buf := logs.NewRedisBuffer(writer, jobID)

	// Write 5 entries
	for i := range 5 {
		buf.Write(logs.LogEntry{
			Level:   "info",
			Message: "msg",
			JobID:   jobID,
			ExecID:  "e1",
		})
		_ = i // Avoid unused variable lint
	}

	entries := buf.ReadAll()
	if len(entries) != 5 {
		t.Errorf("expected 5 entries, got %d", len(entries))
	}
}

func TestRedisBuffer_LineCount(t *testing.T) {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	jobID := "test-buffer-count-" + time.Now().Format("20060102150405")
	streamKey := "logs:" + jobID
	defer client.Del(context.Background(), streamKey)

	writer := logs.NewRedisStreamWriter(client, "logs", 3600)
	buf := logs.NewRedisBuffer(writer, jobID)

	for range 7 {
		buf.Write(logs.LogEntry{Level: "info", Message: "msg", JobID: jobID, ExecID: "e1"})
	}

	if buf.LineCount() != 7 {
		t.Errorf("LineCount() = %d, want %d", buf.LineCount(), 7)
	}
	if buf.Size() != 7 {
		t.Errorf("Size() = %d, want %d", buf.Size(), 7)
	}
}

func TestRedisBuffer_Clear(t *testing.T) {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	jobID := "test-buffer-clear-" + time.Now().Format("20060102150405")
	streamKey := "logs:" + jobID
	defer client.Del(context.Background(), streamKey)

	writer := logs.NewRedisStreamWriter(client, "logs", 3600)
	buf := logs.NewRedisBuffer(writer, jobID)

	buf.Write(logs.LogEntry{Level: "info", Message: "msg", JobID: jobID, ExecID: "e1"})

	// Verify entry exists
	entries := buf.ReadAll()
	if len(entries) != 1 {
		t.Fatal("expected 1 entry before clear")
	}

	// Clear
	buf.Clear()

	// Verify cleared
	entries = buf.ReadAll()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after clear, got %d", len(entries))
	}
}

func TestRedisBuffer_Bytes(t *testing.T) {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	jobID := "test-buffer-bytes-" + time.Now().Format("20060102150405")
	streamKey := "logs:" + jobID
	defer client.Del(context.Background(), streamKey)

	writer := logs.NewRedisStreamWriter(client, "logs", 3600)
	buf := logs.NewRedisBuffer(writer, jobID)

	buf.Write(logs.LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "Test message",
		JobID:     jobID,
		ExecID:    "e1",
	})

	bytes := buf.Bytes()
	if len(bytes) == 0 {
		t.Error("Bytes() returned empty")
	}
	// Should contain JSON line
	if string(bytes)[0] != '{' {
		t.Errorf("expected JSON, got: %s", string(bytes[:50]))
	}
}

func TestRedisBuffer_ReadSince(t *testing.T) {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	jobID := "test-buffer-since-" + time.Now().Format("20060102150405")
	streamKey := "logs:" + jobID
	defer client.Del(context.Background(), streamKey)

	writer := logs.NewRedisStreamWriter(client, "logs", 3600)
	buf := logs.NewRedisBuffer(writer, jobID)

	// Write entry at known time
	oldTime := time.Now().Add(-time.Hour)
	buf.Write(logs.LogEntry{Timestamp: oldTime, Level: "info", Message: "old", JobID: jobID, ExecID: "e1"})

	midTime := time.Now().Add(-time.Minute)
	buf.Write(logs.LogEntry{Timestamp: time.Now(), Level: "info", Message: "new", JobID: jobID, ExecID: "e1"})

	// Read since midTime - should only get the new entry
	entries := buf.ReadSince(midTime)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry since midTime, got %d", len(entries))
	}
	if len(entries) > 0 && entries[0].Message != "new" {
		t.Errorf("expected 'new' message, got %q", entries[0].Message)
	}
}
