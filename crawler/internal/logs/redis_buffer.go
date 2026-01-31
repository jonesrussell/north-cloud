package logs

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"time"
)

// redisBufferTimeout is the timeout for Redis operations in the buffer.
const redisBufferTimeout = 5 * time.Second

// RedisBuffer implements Buffer using Redis Streams as storage.
type RedisBuffer struct {
	writer    *RedisStreamWriter
	jobID     string
	lineCount atomic.Int64
}

// NewRedisBuffer creates a buffer backed by Redis Streams.
func NewRedisBuffer(writer *RedisStreamWriter, jobID string) *RedisBuffer {
	return &RedisBuffer{
		writer: writer,
		jobID:  jobID,
	}
}

// Write adds an entry to the Redis stream.
func (b *RedisBuffer) Write(entry LogEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), redisBufferTimeout)
	defer cancel()

	// Ensure entry has jobID set
	if entry.JobID == "" {
		entry.JobID = b.jobID
	}

	if err := b.writer.WriteEntry(ctx, entry); err != nil {
		// Best effort - don't fail on write errors
		return
	}
	b.lineCount.Add(1)
}

// ReadSince returns entries after the given time.
func (b *RedisBuffer) ReadSince(since time.Time) []LogEntry {
	ctx, cancel := context.WithTimeout(context.Background(), redisBufferTimeout)
	defer cancel()

	entries, err := b.writer.ReadAll(ctx, b.jobID)
	if err != nil {
		return nil
	}

	// Filter by timestamp
	result := make([]LogEntry, 0)
	for _, e := range entries {
		if e.Timestamp.After(since) {
			result = append(result, e)
		}
	}
	return result
}

// ReadAll returns all entries in the stream.
func (b *RedisBuffer) ReadAll() []LogEntry {
	ctx, cancel := context.WithTimeout(context.Background(), redisBufferTimeout)
	defer cancel()

	entries, err := b.writer.ReadAll(ctx, b.jobID)
	if err != nil {
		return nil
	}
	return entries
}

// ReadLast returns the last n entries in chronological order.
func (b *RedisBuffer) ReadLast(n int) []LogEntry {
	ctx, cancel := context.WithTimeout(context.Background(), redisBufferTimeout)
	defer cancel()

	entries, err := b.writer.ReadLast(ctx, b.jobID, n)
	if err != nil {
		return nil
	}
	return entries
}

// Size returns the current number of entries written (local count).
func (b *RedisBuffer) Size() int {
	return int(b.lineCount.Load())
}

// Clear deletes the stream (used after archival).
func (b *RedisBuffer) Clear() {
	ctx, cancel := context.WithTimeout(context.Background(), redisBufferTimeout)
	defer cancel()

	_ = b.writer.Delete(ctx, b.jobID)
	b.lineCount.Store(0)
}

// Bytes returns all entries as JSON lines (for archiving).
func (b *RedisBuffer) Bytes() []byte {
	entries := b.ReadAll()
	if len(entries) == 0 {
		return nil
	}

	var result []byte
	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		result = append(result, line...)
		result = append(result, '\n')
	}
	return result
}

// LineCount returns total entries written (local count).
func (b *RedisBuffer) LineCount() int {
	return int(b.lineCount.Load())
}
