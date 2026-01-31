package logs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisReadBatchSize is the max entries to read in a single XREAD call.
const redisReadBatchSize = 1000

// RedisStreamWriter writes log entries to Redis Streams.
type RedisStreamWriter struct {
	client     *redis.Client
	keyPrefix  string
	ttlSeconds int
}

// NewRedisStreamWriter creates a new Redis stream writer.
func NewRedisStreamWriter(client *redis.Client, keyPrefix string, ttlSeconds int) *RedisStreamWriter {
	return &RedisStreamWriter{
		client:     client,
		keyPrefix:  keyPrefix,
		ttlSeconds: ttlSeconds,
	}
}

// Client returns the underlying Redis client for advanced operations.
func (w *RedisStreamWriter) Client() *redis.Client {
	return w.client
}

// KeyPrefix returns the configured key prefix.
func (w *RedisStreamWriter) KeyPrefix() string {
	return w.keyPrefix
}

// StreamKey returns the Redis key for a job's log stream.
func (w *RedisStreamWriter) StreamKey(jobID string) string {
	return fmt.Sprintf("%s:%s", w.keyPrefix, jobID)
}

// WriteEntry writes a log entry to the Redis stream.
func (w *RedisStreamWriter) WriteEntry(ctx context.Context, entry LogEntry) error {
	key := w.StreamKey(entry.JobID)

	// Serialize fields to JSON if present
	var fieldsJSON string
	if len(entry.Fields) > 0 {
		b, err := json.Marshal(entry.Fields)
		if err != nil {
			return fmt.Errorf("marshal fields: %w", err)
		}
		fieldsJSON = string(b)
	}

	args := &redis.XAddArgs{
		Stream: key,
		Values: map[string]any{
			"timestamp": entry.Timestamp.Format(time.RFC3339Nano),
			"level":     entry.Level,
			"category":  entry.Category,
			"message":   entry.Message,
			"job_id":    entry.JobID,
			"exec_id":   entry.ExecID,
			"fields":    fieldsJSON,
		},
	}

	_, err := w.client.XAdd(ctx, args).Result()
	if err != nil {
		return fmt.Errorf("xadd: %w", err)
	}

	// Set TTL on first write (idempotent - Expire resets TTL each time)
	ttl := time.Duration(w.ttlSeconds) * time.Second
	w.client.Expire(ctx, key, ttl)

	return nil
}

// ReadLast reads the last n entries from the stream in chronological order.
func (w *RedisStreamWriter) ReadLast(ctx context.Context, jobID string, n int) ([]LogEntry, error) {
	key := w.StreamKey(jobID)

	// XREVRANGE returns newest first
	messages, err := w.client.XRevRange(ctx, key, "+", "-").Result()
	if err != nil {
		return nil, fmt.Errorf("xrevrange: %w", err)
	}

	// Take only n entries and reverse to chronological order
	count := n
	if len(messages) < count {
		count = len(messages)
	}

	entries := make([]LogEntry, 0, count)
	for i := count - 1; i >= 0; i-- {
		entry := w.parseMessage(messages[i])
		entries = append(entries, entry)
	}

	return entries, nil
}

// ReadAll reads all entries from the stream in chronological order.
func (w *RedisStreamWriter) ReadAll(ctx context.Context, jobID string) ([]LogEntry, error) {
	key := w.StreamKey(jobID)

	messages, err := w.client.XRange(ctx, key, "-", "+").Result()
	if err != nil {
		return nil, fmt.Errorf("xrange: %w", err)
	}

	entries := make([]LogEntry, 0, len(messages))
	for _, msg := range messages {
		entry := w.parseMessage(msg)
		entries = append(entries, entry)
	}

	return entries, nil
}

// ReadSince reads entries after the given stream ID.
func (w *RedisStreamWriter) ReadSince(ctx context.Context, jobID, lastID string) ([]LogEntry, error) {
	key := w.StreamKey(jobID)

	// Use XREAD with the last ID to get new entries
	streams, err := w.client.XRead(ctx, &redis.XReadArgs{
		Streams: []string{key, lastID},
		Count:   redisReadBatchSize,
		Block:   0, // Non-blocking
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil // No new entries
		}
		return nil, fmt.Errorf("xread: %w", err)
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, nil
	}

	entries := make([]LogEntry, 0, len(streams[0].Messages))
	for _, msg := range streams[0].Messages {
		entry := w.parseMessage(msg)
		entries = append(entries, entry)
	}

	return entries, nil
}

// Delete removes the stream for a job.
func (w *RedisStreamWriter) Delete(ctx context.Context, jobID string) error {
	key := w.StreamKey(jobID)
	return w.client.Del(ctx, key).Err()
}

// parseMessage converts a Redis stream message to a LogEntry.
func (w *RedisStreamWriter) parseMessage(msg redis.XMessage) LogEntry {
	entry := LogEntry{}

	if ts, ok := msg.Values["timestamp"].(string); ok {
		t, err := time.Parse(time.RFC3339Nano, ts)
		if err == nil {
			entry.Timestamp = t
		}
	}

	if v, ok := msg.Values["level"].(string); ok {
		entry.Level = v
	}
	if v, ok := msg.Values["category"].(string); ok {
		entry.Category = v
	}
	if v, ok := msg.Values["message"].(string); ok {
		entry.Message = v
	}
	if v, ok := msg.Values["job_id"].(string); ok {
		entry.JobID = v
	}
	if v, ok := msg.Values["exec_id"].(string); ok {
		entry.ExecID = v
	}
	if v, ok := msg.Values["fields"].(string); ok && v != "" {
		var fields map[string]any
		if err := json.Unmarshal([]byte(v), &fields); err == nil {
			entry.Fields = fields
		}
	}

	return entry
}
