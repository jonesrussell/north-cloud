# Redis Streams Logging Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace in-memory log buffers with Redis Streams for reliable real-time job log streaming.

**Architecture:** Services write logs to Redis Streams via `XADD`. SSE gateway reads via `XREAD BLOCK`. Native replay via stream IDs. TTL handles cleanup.

**Tech Stack:** Go 1.24+, Redis Streams, go-redis/redis/v9, existing SSE infrastructure

---

## Task 1: Add Redis Config Fields

**Files:**
- Modify: `crawler/internal/logs/types.go`

**Step 1: Write the failing test**

Create test file first:

```go
// crawler/internal/logs/types_test.go
package logs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_RedisDefaults(t *testing.T) {
	t.Helper()
	cfg := DefaultConfig()

	assert.False(t, cfg.RedisEnabled, "Redis should be disabled by default")
	assert.Equal(t, "logs", cfg.RedisKeyPrefix, "default key prefix")
	assert.Equal(t, 86400, cfg.RedisTTLSeconds, "default TTL is 24 hours")
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/logs -run TestConfig_RedisDefaults -v`
Expected: FAIL - RedisEnabled field doesn't exist

**Step 3: Add config fields**

```go
// In crawler/internal/logs/types.go, add to Config struct:

type Config struct {
	Enabled           bool   `default:"true" env:"JOB_LOGS_ENABLED"`
	BufferSize        int    `default:"1000" env:"JOB_LOGS_BUFFER_SIZE"`
	SSEEnabled        bool   `default:"true" env:"JOB_LOGS_SSE_ENABLED"`
	ArchiveEnabled    bool   `default:"true" env:"JOB_LOGS_ARCHIVE_ENABLED"`
	RetentionDays     int    `default:"30" env:"JOB_LOGS_RETENTION_DAYS"`
	MinLevel          string `default:"info" env:"JOB_LOGS_MIN_LEVEL"`
	MinioBucket       string `default:"crawler-logs" env:"JOB_LOGS_MINIO_BUCKET"`
	MilestoneInterval int    `default:"50" env:"JOB_LOGS_MILESTONE_INTERVAL"`

	// Redis Streams (optional, replaces in-memory buffer)
	RedisEnabled   bool   `default:"false" env:"JOB_LOGS_REDIS_ENABLED"`
	RedisKeyPrefix string `default:"logs" env:"JOB_LOGS_REDIS_KEY_PREFIX"`
	RedisTTLSeconds int   `default:"86400" env:"JOB_LOGS_REDIS_TTL_SECONDS"`
}

// Update DefaultConfig() to include new fields:
func DefaultConfig() Config {
	return Config{
		Enabled:           true,
		BufferSize:        defaultBufferSize,
		SSEEnabled:        true,
		ArchiveEnabled:    true,
		RetentionDays:     defaultRetentionDays,
		MinLevel:          defaultMinLevel,
		MinioBucket:       defaultMinioBucket,
		MilestoneInterval: defaultMilestoneInterval,
		RedisEnabled:      false,
		RedisKeyPrefix:    "logs",
		RedisTTLSeconds:   86400,
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd crawler && go test ./internal/logs -run TestConfig_RedisDefaults -v`
Expected: PASS

**Step 5: Run linter**

Run: `cd crawler && golangci-lint run ./internal/logs/...`
Expected: No errors

**Step 6: Commit**

```bash
git add crawler/internal/logs/types.go crawler/internal/logs/types_test.go
git commit -m "feat(logs): add Redis Streams config fields"
```

---

## Task 2: Create Redis Stream Writer Interface

**Files:**
- Create: `crawler/internal/logs/redis_writer.go`
- Create: `crawler/internal/logs/redis_writer_test.go`

**Step 1: Write the failing test**

```go
// crawler/internal/logs/redis_writer_test.go
package logs

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	writer := NewRedisStreamWriter(client, "logs", 3600)

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Category:  "crawler.lifecycle",
		Message:   "Test message",
		JobID:     jobID,
		ExecID:    "exec-1",
	}

	err := writer.WriteEntry(context.Background(), entry)
	require.NoError(t, err)

	// Verify entry in Redis
	entries, err := client.XRange(context.Background(), streamKey, "-", "+").Result()
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "info", entries[0].Values["level"])
	assert.Equal(t, "Test message", entries[0].Values["message"])
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

	writer := NewRedisStreamWriter(client, "logs", 3600)
	ctx := context.Background()

	// Write 5 entries
	for i := 0; i < 5; i++ {
		entry := LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   "Message " + string(rune('A'+i)),
			JobID:     jobID,
			ExecID:    "exec-1",
		}
		require.NoError(t, writer.WriteEntry(ctx, entry))
	}

	// Read last 3
	entries, err := writer.ReadLast(ctx, jobID, 3)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
	assert.Equal(t, "Message C", entries[0].Message)
	assert.Equal(t, "Message D", entries[1].Message)
	assert.Equal(t, "Message E", entries[2].Message)
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/logs -run TestRedisStreamWriter -v`
Expected: FAIL - NewRedisStreamWriter undefined

**Step 3: Implement RedisStreamWriter**

```go
// crawler/internal/logs/redis_writer.go
package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

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

// streamKey returns the Redis key for a job's log stream.
func (w *RedisStreamWriter) streamKey(jobID string) string {
	return fmt.Sprintf("%s:%s", w.keyPrefix, jobID)
}

// WriteEntry writes a log entry to the Redis stream.
func (w *RedisStreamWriter) WriteEntry(ctx context.Context, entry LogEntry) error {
	key := w.streamKey(entry.JobID)

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

	result, err := w.client.XAdd(ctx, args).Result()
	if err != nil {
		return fmt.Errorf("xadd: %w", err)
	}

	// Set TTL on first write (idempotent via NX)
	ttl := time.Duration(w.ttlSeconds) * time.Second
	w.client.Expire(ctx, key, ttl)

	_ = result // Stream ID, not needed
	return nil
}

// ReadLast reads the last n entries from the stream.
func (w *RedisStreamWriter) ReadLast(ctx context.Context, jobID string, n int) ([]LogEntry, error) {
	key := w.streamKey(jobID)

	// XREVRANGE returns newest first, so reverse after
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
		entry, err := w.parseMessage(messages[i])
		if err != nil {
			continue // Skip malformed entries
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// ReadAll reads all entries from the stream.
func (w *RedisStreamWriter) ReadAll(ctx context.Context, jobID string) ([]LogEntry, error) {
	key := w.streamKey(jobID)

	messages, err := w.client.XRange(ctx, key, "-", "+").Result()
	if err != nil {
		return nil, fmt.Errorf("xrange: %w", err)
	}

	entries := make([]LogEntry, 0, len(messages))
	for _, msg := range messages {
		entry, err := w.parseMessage(msg)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// ReadSince reads entries since the given stream ID.
func (w *RedisStreamWriter) ReadSince(ctx context.Context, jobID, lastID string) ([]LogEntry, error) {
	key := w.streamKey(jobID)

	// Use XREAD with the last ID to get new entries
	streams, err := w.client.XRead(ctx, &redis.XReadArgs{
		Streams: []string{key, lastID},
		Count:   1000,
		Block:   0, // Non-blocking
	}).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // No new entries
		}
		return nil, fmt.Errorf("xread: %w", err)
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, nil
	}

	entries := make([]LogEntry, 0, len(streams[0].Messages))
	for _, msg := range streams[0].Messages {
		entry, err := w.parseMessage(msg)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// Delete removes the stream for a job.
func (w *RedisStreamWriter) Delete(ctx context.Context, jobID string) error {
	key := w.streamKey(jobID)
	return w.client.Del(ctx, key).Err()
}

// parseMessage converts a Redis stream message to a LogEntry.
func (w *RedisStreamWriter) parseMessage(msg redis.XMessage) (LogEntry, error) {
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

	return entry, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd crawler && go test ./internal/logs -run TestRedisStreamWriter -v`
Expected: PASS (or SKIP if no Redis)

**Step 5: Run linter**

Run: `cd crawler && golangci-lint run ./internal/logs/...`
Expected: No errors

**Step 6: Commit**

```bash
git add crawler/internal/logs/redis_writer.go crawler/internal/logs/redis_writer_test.go
git commit -m "feat(logs): add RedisStreamWriter for log persistence"
```

---

## Task 3: Create Redis-backed Buffer

**Files:**
- Create: `crawler/internal/logs/redis_buffer.go`
- Create: `crawler/internal/logs/redis_buffer_test.go`

**Step 1: Write the failing test**

```go
// crawler/internal/logs/redis_buffer_test.go
package logs

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisBuffer_ImplementsBuffer(t *testing.T) {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	jobID := "test-buffer-" + time.Now().Format("20060102150405")
	writer := NewRedisStreamWriter(client, "logs", 3600)

	var buf Buffer = NewRedisBuffer(writer, jobID)
	assert.NotNil(t, buf)
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

	writer := NewRedisStreamWriter(client, "logs", 3600)
	buf := NewRedisBuffer(writer, jobID)

	// Write entries
	for i := 0; i < 10; i++ {
		buf.Write(LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   "Message " + string(rune('0'+i)),
			JobID:     jobID,
			ExecID:    "exec-1",
		})
	}

	// Read last 5
	entries := buf.ReadLast(5)
	require.Len(t, entries, 5)
	assert.Equal(t, "Message 5", entries[0].Message)
	assert.Equal(t, "Message 9", entries[4].Message)
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

	writer := NewRedisStreamWriter(client, "logs", 3600)
	buf := NewRedisBuffer(writer, jobID)

	for i := 0; i < 7; i++ {
		buf.Write(LogEntry{Level: "info", Message: "msg", JobID: jobID, ExecID: "e1"})
	}

	assert.Equal(t, 7, buf.LineCount())
	assert.Equal(t, 7, buf.Size())
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/logs -run TestRedisBuffer -v`
Expected: FAIL - NewRedisBuffer undefined

**Step 3: Implement RedisBuffer**

```go
// crawler/internal/logs/redis_buffer.go
package logs

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"time"
)

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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := b.writer.WriteEntry(ctx, entry); err != nil {
		// Log error but don't fail - best effort
		return
	}
	b.lineCount.Add(1)
}

// ReadSince returns entries after the given time.
func (b *RedisBuffer) ReadSince(since time.Time) []LogEntry {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entries, err := b.writer.ReadAll(ctx, b.jobID)
	if err != nil {
		return nil
	}
	return entries
}

// ReadLast returns the last n entries in chronological order.
func (b *RedisBuffer) ReadLast(n int) []LogEntry {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entries, err := b.writer.ReadLast(ctx, b.jobID, n)
	if err != nil {
		return nil
	}
	return entries
}

// Size returns the current number of entries.
func (b *RedisBuffer) Size() int {
	return int(b.lineCount.Load())
}

// Clear deletes the stream (used after archival).
func (b *RedisBuffer) Clear() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = b.writer.Delete(ctx, b.jobID)
	b.lineCount.Store(0)
}

// Bytes returns all entries as JSON lines (for archival).
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

// LineCount returns total entries written.
func (b *RedisBuffer) LineCount() int {
	return int(b.lineCount.Load())
}
```

**Step 4: Run test to verify it passes**

Run: `cd crawler && go test ./internal/logs -run TestRedisBuffer -v`
Expected: PASS (or SKIP if no Redis)

**Step 5: Run linter**

Run: `cd crawler && golangci-lint run ./internal/logs/...`
Expected: No errors

**Step 6: Commit**

```bash
git add crawler/internal/logs/redis_buffer.go crawler/internal/logs/redis_buffer_test.go
git commit -m "feat(logs): add RedisBuffer implementing Buffer interface"
```

---

## Task 4: Wire Redis into Log Service

**Files:**
- Modify: `crawler/internal/logs/service.go`
- Modify: `crawler/cmd/httpd/httpd.go`

**Step 1: Write the failing test**

```go
// Add to crawler/internal/logs/service_test.go

func TestNewService_WithRedis(t *testing.T) {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	cfg := DefaultConfig()
	cfg.RedisEnabled = true

	redisWriter := NewRedisStreamWriter(client, cfg.RedisKeyPrefix, cfg.RedisTTLSeconds)

	svc := NewService(cfg, nil, nil, nil, nil, WithRedisWriter(redisWriter))
	assert.NotNil(t, svc)

	// Start capture should use Redis buffer
	jobID := "test-svc-" + time.Now().Format("20060102150405")
	writer, err := svc.StartCapture(context.Background(), jobID, "exec-1", 1)
	require.NoError(t, err)
	defer svc.StopCapture(context.Background(), jobID, "exec-1")

	// Write a log
	writer.WriteEntry(LogEntry{
		Level:   "info",
		Message: "Test from service",
		JobID:   jobID,
		ExecID:  "exec-1",
	})

	// Verify in Redis
	buf := svc.GetLiveBuffer(jobID)
	entries := buf.ReadAll()
	assert.GreaterOrEqual(t, len(entries), 1)
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/logs -run TestNewService_WithRedis -v`
Expected: FAIL - WithRedisWriter undefined

**Step 3: Update service to support Redis**

```go
// Add to crawler/internal/logs/service.go

// ServiceOption configures the log service.
type ServiceOption func(*logService)

// WithRedisWriter enables Redis-backed log buffering.
func WithRedisWriter(writer *RedisStreamWriter) ServiceOption {
	return func(s *logService) {
		s.redisWriter = writer
	}
}

// Update logService struct to include:
type logService struct {
	config        Config
	archiver      Archiver
	publisher     Publisher
	executionRepo database.ExecutionRepositoryInterface
	logger        infralogger.Logger

	activeWriters map[string]*activeWriter
	buffers       map[string]Buffer
	mu            sync.RWMutex

	redisWriter   *RedisStreamWriter // nil if Redis disabled
}

// Update NewService to accept options:
func NewService(
	cfg Config,
	archiver Archiver,
	publisher Publisher,
	executionRepo database.ExecutionRepositoryInterface,
	logger infralogger.Logger,
	opts ...ServiceOption,
) Service {
	s := &logService{
		config:        cfg,
		archiver:      archiver,
		publisher:     publisher,
		executionRepo: executionRepo,
		logger:        logger,
		activeWriters: make(map[string]*activeWriter),
		buffers:       make(map[string]Buffer),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Update StartCapture to use Redis buffer when enabled:
func (s *logService) StartCapture(ctx context.Context, jobID, executionID string, executionNumber int) (Writer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create buffer based on config
	var buffer Buffer
	if s.config.RedisEnabled && s.redisWriter != nil {
		buffer = NewRedisBuffer(s.redisWriter, jobID)
	} else {
		buffer = NewBuffer(s.config.BufferSize)
	}

	s.buffers[jobID] = buffer

	writer := NewWriter(ctx, jobID, executionID, buffer, s.publisher, s.config.MinLevel)

	s.activeWriters[executionID] = &activeWriter{
		writer:          writer,
		jobID:           jobID,
		executionID:     executionID,
		executionNumber: executionNumber,
		startedAt:       time.Now(),
	}

	return writer, nil
}

// Update GetLiveBuffer:
func (s *logService) GetLiveBuffer(jobID string) Buffer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buffers[jobID]
}
```

**Step 4: Run test to verify it passes**

Run: `cd crawler && go test ./internal/logs -run TestNewService_WithRedis -v`
Expected: PASS (or SKIP if no Redis)

**Step 5: Run linter**

Run: `cd crawler && golangci-lint run ./internal/logs/...`
Expected: No errors

**Step 6: Commit**

```bash
git add crawler/internal/logs/service.go crawler/internal/logs/service_test.go
git commit -m "feat(logs): wire Redis buffer into log service"
```

---

## Task 5: Wire Redis Client in httpd.go

**Files:**
- Modify: `crawler/cmd/httpd/httpd.go`
- Modify: `crawler/internal/config/config.go` (if needed)

**Step 1: Add Redis wiring**

Locate the log service initialization section (around lines 422-436) and update:

```go
// In setupJobsAndScheduler or wherever log service is created:

// Create log service components
logsCfg := logs.DefaultConfig()

// Check if Redis logs are enabled
var redisWriter *logs.RedisStreamWriter
if logsCfg.RedisEnabled {
	redisCfg := deps.Config.GetRedisConfig()
	if redisCfg.Enabled {
		redisClient, err := infraredis.NewClient(infraredis.Config{
			Address:  redisCfg.Address,
			Password: redisCfg.Password,
			DB:       redisCfg.DB,
		})
		if err != nil {
			deps.Logger.Warn("Redis not available for job logs, falling back to in-memory",
				infralogger.Error(err))
		} else {
			redisWriter = logs.NewRedisStreamWriter(
				redisClient,
				logsCfg.RedisKeyPrefix,
				logsCfg.RedisTTLSeconds,
			)
			deps.Logger.Info("Job logs Redis persistence enabled",
				infralogger.String("prefix", logsCfg.RedisKeyPrefix))
		}
	}
}

// Optional: MinIO archiver
logArchiver, _ := logs.NewArchiver(
	deps.Config.GetMinIOConfig(),
	logsCfg.MinioBucket,
	deps.Logger,
)

// SSE publisher
logsPublisher := logs.NewPublisher(sseBroker, deps.Logger, logsCfg.SSEEnabled)

// Main service - with optional Redis
var logService logs.Service
if redisWriter != nil {
	logService = logs.NewService(logsCfg, logArchiver, logsPublisher, executionRepo, deps.Logger,
		logs.WithRedisWriter(redisWriter))
} else {
	logService = logs.NewService(logsCfg, logArchiver, logsPublisher, executionRepo, deps.Logger)
}
```

**Step 2: Run build**

Run: `cd crawler && go build -o /dev/null ./cmd/httpd`
Expected: Build succeeds

**Step 3: Run linter**

Run: `cd crawler && golangci-lint run ./cmd/httpd/...`
Expected: No errors

**Step 4: Commit**

```bash
git add crawler/cmd/httpd/httpd.go
git commit -m "feat(logs): wire Redis client for job log persistence"
```

---

## Task 6: Create SSE Gateway v2 Handler

**Files:**
- Create: `crawler/internal/api/logs_stream_v2_handler.go`
- Create: `crawler/internal/api/logs_stream_v2_handler_test.go`
- Modify: `crawler/internal/api/routes.go`

**Step 1: Write the failing test**

```go
// crawler/internal/api/logs_stream_v2_handler_test.go
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/north-cloud/crawler/internal/logs"
)

func TestLogsStreamV2Handler_Stream(t *testing.T) {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	jobID := "test-stream-v2-" + time.Now().Format("20060102150405")
	streamKey := "logs:" + jobID
	defer client.Del(context.Background(), streamKey)

	// Pre-populate some logs
	writer := logs.NewRedisStreamWriter(client, "logs", 3600)
	for i := 0; i < 3; i++ {
		err := writer.WriteEntry(context.Background(), logs.LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   "Pre-existing log " + string(rune('A'+i)),
			JobID:     jobID,
			ExecID:    "exec-1",
		})
		require.NoError(t, err)
	}

	handler := NewLogsStreamV2Handler(writer, nil)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/jobs/:id/logs/stream/v2", handler.Stream)

	req, _ := http.NewRequest("GET", "/jobs/"+jobID+"/logs/stream/v2", nil)
	req.Header.Set("Accept", "text/event-stream")

	w := httptest.NewRecorder()

	// Run in goroutine since SSE blocks
	done := make(chan struct{})
	go func() {
		r.ServeHTTP(w, req)
		close(done)
	}()

	// Wait briefly for initial replay
	time.Sleep(100 * time.Millisecond)

	// Verify we got SSE response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/api -run TestLogsStreamV2Handler -v`
Expected: FAIL - NewLogsStreamV2Handler undefined

**Step 3: Implement v2 handler**

```go
// crawler/internal/api/logs_stream_v2_handler.go
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/north-cloud/crawler/internal/logs"
	"github.com/north-cloud/infrastructure/sse"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	streamV2ReplayCount = 200
	streamV2BlockMs     = 5000
)

// LogsStreamV2Handler handles SSE streaming from Redis Streams.
type LogsStreamV2Handler struct {
	redisWriter *logs.RedisStreamWriter
	logger      infralogger.Logger
}

// NewLogsStreamV2Handler creates a new v2 stream handler.
func NewLogsStreamV2Handler(redisWriter *logs.RedisStreamWriter, logger infralogger.Logger) *LogsStreamV2Handler {
	return &LogsStreamV2Handler{
		redisWriter: redisWriter,
		logger:      logger,
	}
}

// Stream handles GET /api/v1/jobs/:id/logs/stream/v2
func (h *LogsStreamV2Handler) Stream(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job_id required"})
		return
	}

	// Set SSE headers
	sse.SetSSEHeaders(c.Writer)

	ctx := c.Request.Context()

	// Get Last-Event-ID for resume (Redis stream ID format)
	lastEventID := c.GetHeader("Last-Event-ID")
	if lastEventID == "" {
		lastEventID = "0" // Start from beginning
	}

	// Initial replay
	if lastEventID == "0" {
		h.replayLogs(c, jobID)
	}

	// Stream new entries
	h.streamLogs(ctx, c, jobID, lastEventID)
}

// replayLogs sends buffered logs as a replay event.
func (h *LogsStreamV2Handler) replayLogs(c *gin.Context, jobID string) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	entries, err := h.redisWriter.ReadLast(ctx, jobID, streamV2ReplayCount)
	if err != nil {
		h.logError("replay failed", err, jobID)
		return
	}

	if len(entries) == 0 {
		return
	}

	// Convert to SSE LogLineData format
	lines := make([]sse.LogLineData, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, sse.LogLineData{
			JobID:       entry.JobID,
			ExecutionID: entry.ExecID,
			Level:       entry.Level,
			Category:    entry.Category,
			Message:     entry.Message,
			Fields:      entry.Fields,
			Timestamp:   entry.Timestamp.Format(time.RFC3339),
		})
	}

	event := sse.NewLogReplayEvent(lines)
	h.sendEvent(c.Writer, event)
}

// streamLogs continuously streams new log entries.
func (h *LogsStreamV2Handler) streamLogs(ctx context.Context, c *gin.Context, jobID, lastID string) {
	streamKey := fmt.Sprintf("logs:%s", jobID)
	currentID := lastID

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Blocking read for new entries
		readCtx, cancel := context.WithTimeout(ctx, time.Duration(streamV2BlockMs)*time.Millisecond)

		streams, err := h.redisWriter.client.XRead(readCtx, &redis.XReadArgs{
			Streams: []string{streamKey, currentID},
			Count:   100,
			Block:   time.Duration(streamV2BlockMs) * time.Millisecond,
		}).Result()
		cancel()

		if err != nil {
			if err == redis.Nil || err == context.DeadlineExceeded {
				// No new entries, send heartbeat
				h.sendHeartbeat(c.Writer)
				continue
			}
			if ctx.Err() != nil {
				return // Client disconnected
			}
			h.logError("xread failed", err, jobID)
			time.Sleep(time.Second) // Back off on error
			continue
		}

		if len(streams) == 0 || len(streams[0].Messages) == 0 {
			continue
		}

		// Send each entry as an SSE event
		for _, msg := range streams[0].Messages {
			entry := h.parseMessage(msg)
			event := sse.Event{
				Type: sse.EventTypeLogLine,
				ID:   msg.ID, // Use Redis stream ID as SSE event ID
				Data: sse.LogLineData{
					JobID:       entry.JobID,
					ExecutionID: entry.ExecID,
					Level:       entry.Level,
					Category:    entry.Category,
					Message:     entry.Message,
					Fields:      entry.Fields,
					Timestamp:   entry.Timestamp.Format(time.RFC3339),
				},
			}
			h.sendEvent(c.Writer, event)
			currentID = msg.ID
		}
	}
}

// sendEvent writes an SSE event to the response.
func (h *LogsStreamV2Handler) sendEvent(w http.ResponseWriter, event sse.Event) {
	data, err := json.Marshal(event.Data)
	if err != nil {
		return
	}

	if event.ID != "" {
		fmt.Fprintf(w, "id: %s\n", event.ID)
	}
	fmt.Fprintf(w, "event: %s\n", event.Type)
	fmt.Fprintf(w, "data: %s\n\n", data)

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// sendHeartbeat sends a comment line to keep connection alive.
func (h *LogsStreamV2Handler) sendHeartbeat(w http.ResponseWriter) {
	fmt.Fprint(w, ": heartbeat\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// parseMessage converts Redis message to LogEntry.
func (h *LogsStreamV2Handler) parseMessage(msg redis.XMessage) logs.LogEntry {
	entry := logs.LogEntry{}

	if ts, ok := msg.Values["timestamp"].(string); ok {
		t, _ := time.Parse(time.RFC3339Nano, ts)
		entry.Timestamp = t
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

func (h *LogsStreamV2Handler) logError(msg string, err error, jobID string) {
	if h.logger != nil {
		h.logger.Error(msg, infralogger.Error(err), infralogger.String("job_id", jobID))
	}
}
```

**Step 4: Add route**

```go
// In crawler/internal/api/routes.go, add to the jobs group:

// v2 streaming endpoint (Redis-backed)
if logsStreamV2Handler != nil {
	jobs.GET("/:id/logs/stream/v2", logsStreamV2Handler.Stream)
}
```

**Step 5: Run test to verify it passes**

Run: `cd crawler && go test ./internal/api -run TestLogsStreamV2Handler -v`
Expected: PASS (or SKIP if no Redis)

**Step 6: Run linter**

Run: `cd crawler && golangci-lint run ./internal/api/...`
Expected: No errors

**Step 7: Commit**

```bash
git add crawler/internal/api/logs_stream_v2_handler.go crawler/internal/api/logs_stream_v2_handler_test.go crawler/internal/api/routes.go
git commit -m "feat(api): add v2 SSE log stream endpoint backed by Redis Streams"
```

---

## Task 7: Update Dashboard to Use v2 Endpoint

**Files:**
- Modify: `dashboard/src/features/intake/components/JobLogs.vue` (or `JobLogsViewer.vue`)
- Modify: `dashboard/src/features/intake/api/jobs.ts`

**Step 1: Update API endpoint**

```typescript
// In dashboard/src/features/intake/api/jobs.ts or equivalent

export const getJobLogsStreamUrl = (jobId: string): string => {
  const token = localStorage.getItem('dashboard_token')
  // Use v2 endpoint for Redis-backed streaming
  return `/api/crawler/jobs/${jobId}/logs/stream/v2?token=${token}`
}
```

**Step 2: Update Vue component**

```vue
<!-- In JobLogs.vue or JobLogsViewer.vue -->
<script setup lang="ts">
// Update the SSE connection to use v2 endpoint
const startLiveStream = () => {
  const url = getJobLogsStreamUrl(props.jobId)

  eventSource = new EventSource(url)

  // EventSource automatically sends Last-Event-ID on reconnect
  eventSource.onmessage = (event) => {
    // Existing handling unchanged
  }

  eventSource.onerror = (error) => {
    console.error('SSE connection error:', error)
    // EventSource auto-reconnects, but we can add backoff if needed
  }
}
</script>
```

**Step 3: Build frontend**

Run: `cd dashboard && npm run build`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add dashboard/src/features/intake/api/jobs.ts dashboard/src/features/intake/components/JobLogs.vue
git commit -m "feat(dashboard): switch to v2 Redis-backed log streaming endpoint"
```

---

## Task 8: Integration Test

**Files:**
- Create: `crawler/internal/integration/logs_stream_test.go`

**Step 1: Write integration test**

```go
// crawler/internal/integration/logs_stream_test.go
//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/north-cloud/crawler/internal/logs"
)

func TestRedisLogsIntegration(t *testing.T) {
	t.Helper()

	// Connect to Redis
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	require.NoError(t, client.Ping(context.Background()).Err())
	defer client.Close()

	jobID := "integration-test-" + time.Now().Format("20060102150405")
	streamKey := "logs:" + jobID
	defer client.Del(context.Background(), streamKey)

	// Create writer and buffer
	writer := logs.NewRedisStreamWriter(client, "logs", 3600)
	buffer := logs.NewRedisBuffer(writer, jobID)

	// Simulate job execution - write logs
	for i := 0; i < 100; i++ {
		buffer.Write(logs.LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Category:  "crawler.lifecycle",
			Message:   "Processing item",
			JobID:     jobID,
			ExecID:    "exec-1",
			Fields:    map[string]any{"item": i},
		})
	}

	// Verify all logs are in Redis
	entries := buffer.ReadAll()
	assert.Len(t, entries, 100)

	// Verify ReadLast works
	last10 := buffer.ReadLast(10)
	assert.Len(t, last10, 10)

	// Verify LineCount
	assert.Equal(t, 100, buffer.LineCount())

	// Verify Bytes() for archival
	bytes := buffer.Bytes()
	assert.NotEmpty(t, bytes)

	// Verify logs survive "restart" (new buffer same stream)
	buffer2 := logs.NewRedisBuffer(writer, jobID)
	entries2 := buffer2.ReadAll()
	assert.Len(t, entries2, 100)
}
```

**Step 2: Run integration test**

Run: `cd crawler && go test ./internal/integration -tags=integration -run TestRedisLogsIntegration -v`
Expected: PASS

**Step 3: Commit**

```bash
git add crawler/internal/integration/logs_stream_test.go
git commit -m "test: add Redis logs integration test"
```

---

## Task 9: Add Environment Variables to Docker Compose

**Files:**
- Modify: `docker-compose.dev.yml`
- Modify: `.env.example`

**Step 1: Update docker-compose.dev.yml**

Add to crawler service environment:

```yaml
crawler:
  environment:
    # ... existing vars ...
    JOB_LOGS_REDIS_ENABLED: ${JOB_LOGS_REDIS_ENABLED:-true}
    JOB_LOGS_REDIS_KEY_PREFIX: ${JOB_LOGS_REDIS_KEY_PREFIX:-logs}
    JOB_LOGS_REDIS_TTL_SECONDS: ${JOB_LOGS_REDIS_TTL_SECONDS:-86400}
```

**Step 2: Update .env.example**

```bash
# Job Logs Redis (optional, enables persistent log streaming)
JOB_LOGS_REDIS_ENABLED=true
JOB_LOGS_REDIS_KEY_PREFIX=logs
JOB_LOGS_REDIS_TTL_SECONDS=86400
```

**Step 3: Commit**

```bash
git add docker-compose.dev.yml .env.example
git commit -m "chore: add Redis logs environment variables"
```

---

## Task 10: Final Verification and Documentation

**Step 1: Run all tests**

```bash
cd crawler && go test ./... -v
```

**Step 2: Run linter**

```bash
cd crawler && golangci-lint run
```

**Step 3: Start services and manual test**

```bash
task docker:dev:up
# Trigger a job and verify logs stream in real-time
```

**Step 4: Update CLAUDE.md**

Add to the crawler section:

```markdown
### Job Logs
- **Redis Streams**: When `JOB_LOGS_REDIS_ENABLED=true`, logs persist to Redis
- **Endpoint**: `/api/v1/jobs/:id/logs/stream/v2` for Redis-backed streaming
- **Debug**: `redis-cli XRANGE logs:{job_id} - +` to view live logs
```

**Step 5: Final commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with Redis logs configuration"
```

---

## Summary

| Task | Files | Outcome |
|------|-------|---------|
| 1 | `types.go` | Config fields for Redis |
| 2 | `redis_writer.go` | XADD, XREAD, XRANGE operations |
| 3 | `redis_buffer.go` | Buffer interface over Redis |
| 4 | `service.go` | Optional Redis backend |
| 5 | `httpd.go` | Redis client wiring |
| 6 | `logs_stream_v2_handler.go` | SSE gateway from Redis |
| 7 | Dashboard | Switch to v2 endpoint |
| 8 | Integration test | End-to-end verification |
| 9 | Docker/env | Configuration |
| 10 | Verification | Tests, lint, docs |

**Rollback:** Set `JOB_LOGS_REDIS_ENABLED=false` to revert to in-memory buffers.
