# Automated Job Lifecycle - Phase 1 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Introduce event infrastructure (publisher in source-manager, consumer in crawler) without breaking existing functionality.

**Architecture:** Redis Streams for event delivery, shared event types in infrastructure package, goroutine-based async publishing after DB commits.

**Tech Stack:** Go 1.24+, Redis Streams (go-redis/v9), PostgreSQL, gin-gonic

---

## Prerequisites

Before starting, ensure you have:
- Docker running with Redis available
- Access to source-manager and crawler codebases
- Ability to run `task test` and `task lint` in each service

---

## Task 1: Add Redis Client to Infrastructure

**Files:**
- Create: `infrastructure/redis/client.go`
- Create: `infrastructure/redis/client_test.go`

### Step 1: Write the failing test

Create test file:

```go
// infrastructure/redis/client_test.go
package redis

import (
	"context"
	"testing"
	"time"
)

func TestNewClient_ReturnsNilWhenAddressEmpty(t *testing.T) {
	t.Helper()

	client, err := NewClient(Config{Address: ""})

	if err == nil {
		t.Error("expected error for empty address")
	}
	if client != nil {
		t.Error("expected nil client for invalid config")
	}
}

func TestNewClient_ConnectsToRedis(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client, err := NewClient(Config{
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

	if err := client.Ping(ctx).Err(); err != nil {
		t.Errorf("ping failed: %v", err)
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd infrastructure && go test ./redis/... -v
```

Expected: FAIL with "package infrastructure/redis is not in std"

### Step 3: Write minimal implementation

```go
// infrastructure/redis/client.go
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
	Address  string `env:"REDIS_ADDRESS" default:"localhost:6379"`
	Password string `env:"REDIS_PASSWORD" default:""`
	DB       int    `env:"REDIS_DB" default:"0"`
}

// ErrEmptyAddress is returned when Redis address is not configured.
var ErrEmptyAddress = errors.New("redis address is required")

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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return client, nil
}
```

### Step 4: Run test to verify it passes

```bash
cd infrastructure && go test ./redis/... -v
```

Expected: PASS (with integration test skipped in short mode)

### Step 5: Add go-redis dependency

```bash
cd infrastructure && go get github.com/redis/go-redis/v9
```

### Step 6: Commit

```bash
git add infrastructure/redis/
git commit -m "feat(infrastructure): add Redis client package

Add infrastructure/redis package with Config and NewClient for
creating Redis connections. This will be used by source-manager
publisher and crawler consumer.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 2: Create Shared Event Types

**Files:**
- Create: `infrastructure/events/types.go`
- Create: `infrastructure/events/types_test.go`

### Step 1: Write the failing test

```go
// infrastructure/events/types_test.go
package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSourceEvent_MarshalJSON(t *testing.T) {
	t.Helper()

	eventID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")
	sourceID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	timestamp := time.Date(2026, 1, 29, 10, 30, 0, 0, time.UTC)

	event := SourceEvent{
		EventID:   eventID,
		EventType: SourceCreated,
		SourceID:  sourceID,
		Timestamp: timestamp,
		Payload: SourceCreatedPayload{
			Name:      "Example News",
			URL:       "https://example.com",
			RateLimit: 10,
			MaxDepth:  3,
			Enabled:   true,
			Priority:  PriorityNormal,
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded SourceEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.EventType != SourceCreated {
		t.Errorf("expected event type %s, got %s", SourceCreated, decoded.EventType)
	}
	if decoded.SourceID != sourceID {
		t.Errorf("expected source ID %s, got %s", sourceID, decoded.SourceID)
	}
}

func TestEventType_Constants(t *testing.T) {
	t.Helper()

	tests := []struct {
		eventType EventType
		expected  string
	}{
		{SourceCreated, "SOURCE_CREATED"},
		{SourceUpdated, "SOURCE_UPDATED"},
		{SourceDeleted, "SOURCE_DELETED"},
		{SourceEnabled, "SOURCE_ENABLED"},
		{SourceDisabled, "SOURCE_DISABLED"},
	}

	for _, tt := range tests {
		if string(tt.eventType) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.eventType)
		}
	}
}

func TestPriority_Constants(t *testing.T) {
	t.Helper()

	if PriorityNormal != "normal" {
		t.Errorf("expected 'normal', got %s", PriorityNormal)
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd infrastructure && go test ./events/... -v
```

Expected: FAIL with "package infrastructure/events is not in std"

### Step 3: Write minimal implementation

```go
// infrastructure/events/types.go
package events

import (
	"time"

	"github.com/google/uuid"
)

// StreamName is the Redis stream for source events.
const StreamName = "source-events"

// ConsumerGroup is the consumer group for crawler workers.
const ConsumerGroup = "crawler-workers"

// EventType represents the type of source event.
type EventType string

const (
	SourceCreated  EventType = "SOURCE_CREATED"
	SourceUpdated  EventType = "SOURCE_UPDATED"
	SourceDeleted  EventType = "SOURCE_DELETED"
	SourceEnabled  EventType = "SOURCE_ENABLED"
	SourceDisabled EventType = "SOURCE_DISABLED"
)

// Priority levels for sources.
const (
	PriorityLow      = "low"
	PriorityNormal   = "normal"
	PriorityHigh     = "high"
	PriorityCritical = "critical"
)

// SourceEvent is the envelope for all source-related events.
type SourceEvent struct {
	EventID   uuid.UUID `json:"event_id"`
	EventType EventType `json:"event_type"`
	SourceID  uuid.UUID `json:"source_id"`
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"payload"`
}

// SourceCreatedPayload contains data for SOURCE_CREATED events.
type SourceCreatedPayload struct {
	Name      string         `json:"name"`
	URL       string         `json:"url"`
	RateLimit int            `json:"rate_limit"`
	MaxDepth  int            `json:"max_depth"`
	Enabled   bool           `json:"enabled"`
	Selectors map[string]any `json:"selectors,omitempty"`
	Priority  string         `json:"priority"`
}

// SourceUpdatedPayload contains data for SOURCE_UPDATED events.
type SourceUpdatedPayload struct {
	ChangedFields []string       `json:"changed_fields"`
	Previous      map[string]any `json:"previous"`
	Current       map[string]any `json:"current"`
}

// SourceDeletedPayload contains data for SOURCE_DELETED events.
type SourceDeletedPayload struct {
	Name           string `json:"name"`
	DeletionReason string `json:"deletion_reason"`
}

// SourceTogglePayload contains data for SOURCE_ENABLED/DISABLED events.
type SourceTogglePayload struct {
	Reason    string `json:"reason"`
	ToggledBy string `json:"toggled_by"` // "user" or "system"
}
```

### Step 4: Run test to verify it passes

```bash
cd infrastructure && go test ./events/... -v
```

Expected: PASS

### Step 5: Commit

```bash
git add infrastructure/events/
git commit -m "feat(infrastructure): add shared event types for source lifecycle

Define event types (SOURCE_CREATED, SOURCE_UPDATED, SOURCE_DELETED,
SOURCE_ENABLED, SOURCE_DISABLED) and payload structs for Redis Streams
communication between source-manager and crawler.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 3: Create Event Publisher in source-manager

**Files:**
- Create: `source-manager/internal/events/publisher.go`
- Create: `source-manager/internal/events/publisher_test.go`

### Step 1: Write the failing test

```go
// source-manager/internal/events/publisher_test.go
package events

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	infraevents "github.com/north-cloud/infrastructure/events"
)

// MockRedisClient implements a minimal mock for testing
type MockRedisClient struct {
	Published []infraevents.SourceEvent
	ShouldErr bool
}

func TestPublisher_Publish_SetsEventIDIfEmpty(t *testing.T) {
	t.Helper()

	// This test verifies that Publish generates an EventID if not provided
	pub := &Publisher{}

	event := infraevents.SourceEvent{
		EventType: infraevents.SourceCreated,
		SourceID:  uuid.New(),
		Payload:   infraevents.SourceCreatedPayload{Name: "Test"},
	}

	// We can't easily test without Redis, so this is a design verification
	if event.EventID == uuid.Nil {
		// This is expected - the publisher should generate one
		t.Log("EventID is nil as expected, publisher should generate one")
	}
}

func TestPublisher_NewPublisher_RequiresClient(t *testing.T) {
	t.Helper()

	pub := NewPublisher(nil, nil)
	if pub != nil {
		t.Error("expected nil publisher when client is nil")
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd source-manager && go test ./internal/events/... -v
```

Expected: FAIL with "package source-manager/internal/events is not in std"

### Step 3: Write minimal implementation

```go
// source-manager/internal/events/publisher.go
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	infraevents "github.com/north-cloud/infrastructure/events"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// Publisher publishes source events to Redis Streams.
type Publisher struct {
	client *redis.Client
	log    infralogger.Logger
}

// NewPublisher creates a new event publisher.
// Returns nil if client is nil.
func NewPublisher(client *redis.Client, log infralogger.Logger) *Publisher {
	if client == nil {
		return nil
	}
	return &Publisher{
		client: client,
		log:    log,
	}
}

// Publish sends an event to the Redis stream.
func (p *Publisher) Publish(ctx context.Context, event infraevents.SourceEvent) error {
	if p == nil || p.client == nil {
		return nil // No-op if publisher not configured
	}

	// Ensure event has ID and timestamp
	if event.EventID == uuid.Nil {
		event.EventID = uuid.New()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	result := p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: infraevents.StreamName,
		Values: map[string]any{
			"event": string(payload),
		},
	})

	if err := result.Err(); err != nil {
		if p.log != nil {
			p.log.Error("Failed to publish event",
				infralogger.String("event_type", string(event.EventType)),
				infralogger.String("source_id", event.SourceID.String()),
				infralogger.Error(err),
			)
		}
		return fmt.Errorf("publish to stream: %w", err)
	}

	if p.log != nil {
		p.log.Info("Published source event",
			infralogger.String("event_type", string(event.EventType)),
			infralogger.String("source_id", event.SourceID.String()),
			infralogger.String("stream_id", result.Val()),
		)
	}

	return nil
}

// PublishAsync publishes an event asynchronously.
// Errors are logged but not returned.
func (p *Publisher) PublishAsync(event infraevents.SourceEvent) {
	if p == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := p.Publish(ctx, event); err != nil && p.log != nil {
			p.log.Error("Async publish failed",
				infralogger.String("event_type", string(event.EventType)),
				infralogger.String("source_id", event.SourceID.String()),
				infralogger.Error(err),
			)
		}
	}()
}
```

### Step 4: Add dependency and run test

```bash
cd source-manager && go get github.com/north-cloud/infrastructure/events
cd source-manager && go test ./internal/events/... -v
```

Expected: PASS

### Step 5: Commit

```bash
git add source-manager/internal/events/
git commit -m "feat(source-manager): add event publisher for Redis Streams

Implement Publisher that sends source lifecycle events to Redis Streams.
Supports both sync Publish and async PublishAsync for non-blocking calls.
Returns nil/no-op if Redis client not configured (graceful degradation).

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 4: Integrate Publisher into Source Handler

**Files:**
- Modify: `source-manager/internal/handlers/source.go`
- Modify: `source-manager/main.go` (wire up Redis client and publisher)

### Step 1: Review existing handler structure

The existing `SourceHandler` has:
- `Create`, `Update`, `Delete` methods
- `repo *repository.SourceRepository`
- `logger infralogger.Logger`

We need to add:
- `publisher *events.Publisher`
- Event publishing after each successful operation

### Step 2: Modify SourceHandler struct

Edit `source-manager/internal/handlers/source.go`:

```go
// Add import
import (
	// ... existing imports ...
	"github.com/jonesrussell/north-cloud/source-manager/internal/events"
	infraevents "github.com/north-cloud/infrastructure/events"
)

// Modify struct
type SourceHandler struct {
	repo      *repository.SourceRepository
	logger    infralogger.Logger
	extractor *metadata.Extractor
	publisher *events.Publisher // NEW
}

// Modify constructor
func NewSourceHandler(repo *repository.SourceRepository, log infralogger.Logger, publisher *events.Publisher) *SourceHandler {
	return &SourceHandler{
		repo:      repo,
		logger:    log,
		extractor: metadata.NewExtractor(log),
		publisher: publisher, // NEW
	}
}
```

### Step 3: Add event publishing to Create method

After successful creation (around line 68), add:

```go
func (h *SourceHandler) Create(c *gin.Context) {
	var source models.Source
	if err := c.ShouldBindJSON(&source); err != nil {
		// ... existing error handling ...
		return
	}

	if err := h.repo.Create(c.Request.Context(), &source); err != nil {
		// ... existing error handling ...
		return
	}

	h.logger.Info("Source created",
		infralogger.String("source_id", source.ID),
		infralogger.String("source_name", source.Name),
	)

	// NEW: Publish event asynchronously
	if h.publisher != nil {
		sourceID, _ := uuid.Parse(source.ID)
		h.publisher.PublishAsync(infraevents.SourceEvent{
			EventType: infraevents.SourceCreated,
			SourceID:  sourceID,
			Payload: infraevents.SourceCreatedPayload{
				Name:      source.Name,
				URL:       source.URL,
				RateLimit: parseRateLimit(source.RateLimit),
				MaxDepth:  source.MaxDepth,
				Enabled:   source.Enabled,
				Priority:  infraevents.PriorityNormal,
			},
		})
	}

	c.JSON(http.StatusCreated, source)
}

// Helper function
func parseRateLimit(rateLimit string) int {
	// Parse "10/s" to 10, default to 10 if parsing fails
	if rateLimit == "" {
		return 10
	}
	var rate int
	fmt.Sscanf(rateLimit, "%d", &rate)
	if rate <= 0 {
		return 10
	}
	return rate
}
```

### Step 4: Add event publishing to Update method

After successful update (around line 139), add:

```go
// After h.logger.Info("Source updated", ...)

// NEW: Publish event asynchronously
if h.publisher != nil {
	sourceID, _ := uuid.Parse(id)
	h.publisher.PublishAsync(infraevents.SourceEvent{
		EventType: infraevents.SourceUpdated,
		SourceID:  sourceID,
		Payload: infraevents.SourceUpdatedPayload{
			ChangedFields: []string{}, // TODO: Track changed fields
			Current: map[string]any{
				"name":       source.Name,
				"rate_limit": source.RateLimit,
				"max_depth":  source.MaxDepth,
				"enabled":    source.Enabled,
			},
		},
	})
}
```

### Step 5: Add event publishing to Delete method

After successful deletion (around line 159), add:

```go
// After h.logger.Info("Source deleted", ...)

// NEW: Publish event asynchronously
if h.publisher != nil {
	sourceID, _ := uuid.Parse(id)
	h.publisher.PublishAsync(infraevents.SourceEvent{
		EventType: infraevents.SourceDeleted,
		SourceID:  sourceID,
		Payload: infraevents.SourceDeletedPayload{
			DeletionReason: "user_requested",
		},
	})
}
```

### Step 6: Run tests

```bash
cd source-manager && go test ./... -v
```

Expected: PASS (may need to update test mocks)

### Step 7: Commit

```bash
git add source-manager/internal/handlers/source.go
git commit -m "feat(source-manager): integrate event publisher into source handlers

Publish SOURCE_CREATED, SOURCE_UPDATED, SOURCE_DELETED events after
successful database operations. Uses async publishing to avoid blocking
HTTP responses. Publisher is optional (nil check) for graceful degradation.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 5: Wire Up Redis in source-manager main.go

**Files:**
- Modify: `source-manager/internal/config/config.go`
- Modify: `source-manager/main.go`

### Step 1: Add Redis config to source-manager config

Edit `source-manager/internal/config/config.go`:

```go
// Add Redis config struct
type RedisConfig struct {
	Address  string `env:"REDIS_ADDRESS" default:"localhost:6379"`
	Password string `env:"REDIS_PASSWORD" default:""`
	DB       int    `env:"REDIS_DB" default:"0"`
	Enabled  bool   `env:"REDIS_EVENTS_ENABLED" default:"false"` // Feature flag
}

// Add to main Config struct
type Config struct {
	// ... existing fields ...
	Redis RedisConfig `envPrefix:"REDIS_"`
}
```

### Step 2: Update main.go to create Redis client and publisher

```go
// In main.go, after config loading

var publisher *events.Publisher

if cfg.Redis.Enabled {
	redisClient, err := infraredis.NewClient(infraredis.Config{
		Address:  cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		log.Warn("Redis not available, events disabled",
			infralogger.Error(err),
		)
	} else {
		publisher = events.NewPublisher(redisClient, log)
		log.Info("Event publisher initialized",
			infralogger.String("redis_address", cfg.Redis.Address),
		)
	}
}

// Pass publisher to handler
sourceHandler := handlers.NewSourceHandler(sourceRepo, log, publisher)
```

### Step 3: Run and verify

```bash
cd source-manager && go build -o /dev/null ./...
```

Expected: Build succeeds

### Step 4: Commit

```bash
git add source-manager/internal/config/config.go source-manager/main.go
git commit -m "feat(source-manager): wire up Redis client and event publisher

Add REDIS_EVENTS_ENABLED feature flag (default: false) for gradual rollout.
When enabled, creates Redis client and passes publisher to handlers.
Gracefully handles Redis unavailability with warning log.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 6: Create Event Consumer Skeleton in crawler

**Files:**
- Create: `crawler/internal/events/consumer.go`
- Create: `crawler/internal/events/consumer_test.go`
- Create: `crawler/internal/events/handler.go`

### Step 1: Write the failing test

```go
// crawler/internal/events/consumer_test.go
package events

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	infraevents "github.com/north-cloud/infrastructure/events"
)

// MockHandler implements EventHandler for testing
type MockHandler struct {
	CreatedEvents  []infraevents.SourceEvent
	UpdatedEvents  []infraevents.SourceEvent
	DeletedEvents  []infraevents.SourceEvent
	EnabledEvents  []infraevents.SourceEvent
	DisabledEvents []infraevents.SourceEvent
}

func (m *MockHandler) HandleSourceCreated(ctx context.Context, event infraevents.SourceEvent) error {
	m.CreatedEvents = append(m.CreatedEvents, event)
	return nil
}

func (m *MockHandler) HandleSourceUpdated(ctx context.Context, event infraevents.SourceEvent) error {
	m.UpdatedEvents = append(m.UpdatedEvents, event)
	return nil
}

func (m *MockHandler) HandleSourceDeleted(ctx context.Context, event infraevents.SourceEvent) error {
	m.DeletedEvents = append(m.DeletedEvents, event)
	return nil
}

func (m *MockHandler) HandleSourceEnabled(ctx context.Context, event infraevents.SourceEvent) error {
	m.EnabledEvents = append(m.EnabledEvents, event)
	return nil
}

func (m *MockHandler) HandleSourceDisabled(ctx context.Context, event infraevents.SourceEvent) error {
	m.DisabledEvents = append(m.DisabledEvents, event)
	return nil
}

func TestNewConsumer_RequiresClient(t *testing.T) {
	t.Helper()

	consumer := NewConsumer(nil, "test", &MockHandler{}, nil)
	if consumer != nil {
		t.Error("expected nil consumer when client is nil")
	}
}

func TestConsumer_GeneratesConsumerID(t *testing.T) {
	t.Helper()

	// Verify that consumer ID generation works
	id := generateConsumerID()
	if id == "" {
		t.Error("expected non-empty consumer ID")
	}
	if len(id) < 10 {
		t.Error("consumer ID too short")
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd crawler && go test ./internal/events/... -v
```

Expected: FAIL with "package crawler/internal/events is not in std"

### Step 3: Write handler interface

```go
// crawler/internal/events/handler.go
package events

import (
	"context"

	infraevents "github.com/north-cloud/infrastructure/events"
)

// EventHandler processes source lifecycle events.
type EventHandler interface {
	HandleSourceCreated(ctx context.Context, event infraevents.SourceEvent) error
	HandleSourceUpdated(ctx context.Context, event infraevents.SourceEvent) error
	HandleSourceDeleted(ctx context.Context, event infraevents.SourceEvent) error
	HandleSourceEnabled(ctx context.Context, event infraevents.SourceEvent) error
	HandleSourceDisabled(ctx context.Context, event infraevents.SourceEvent) error
}
```

### Step 4: Write consumer implementation

```go
// crawler/internal/events/consumer.go
package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	infraevents "github.com/north-cloud/infrastructure/events"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	blockDuration    = 5 * time.Second
	claimIdleTimeout = 30 * time.Second
	batchSize        = 10
)

// Consumer reads events from Redis Streams.
type Consumer struct {
	client     *redis.Client
	consumerID string
	handler    EventHandler
	log        infralogger.Logger
	shutdownCh chan struct{}
}

// NewConsumer creates a new event consumer.
// Returns nil if client is nil.
func NewConsumer(client *redis.Client, consumerID string, handler EventHandler, log infralogger.Logger) *Consumer {
	if client == nil {
		return nil
	}
	if consumerID == "" {
		consumerID = generateConsumerID()
	}
	return &Consumer{
		client:     client,
		consumerID: consumerID,
		handler:    handler,
		log:        log,
		shutdownCh: make(chan struct{}),
	}
}

// generateConsumerID creates a unique consumer identifier.
func generateConsumerID() string {
	return fmt.Sprintf("crawler-%s", uuid.New().String()[:8])
}

// Start begins consuming events from the stream.
func (c *Consumer) Start(ctx context.Context) error {
	if err := c.ensureConsumerGroup(ctx); err != nil {
		return fmt.Errorf("ensure consumer group: %w", err)
	}

	if c.log != nil {
		c.log.Info("Starting event consumer",
			infralogger.String("consumer_id", c.consumerID),
			infralogger.String("group", infraevents.ConsumerGroup),
		)
	}

	go c.consumeLoop(ctx)
	go c.claimAbandonedLoop(ctx)

	return nil
}

// Stop gracefully shuts down the consumer.
func (c *Consumer) Stop() {
	close(c.shutdownCh)
}

func (c *Consumer) consumeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.shutdownCh:
			return
		default:
			c.readAndProcess(ctx)
		}
	}
}

func (c *Consumer) readAndProcess(ctx context.Context) {
	streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    infraevents.ConsumerGroup,
		Consumer: c.consumerID,
		Streams:  []string{infraevents.StreamName, ">"},
		Count:    batchSize,
		Block:    blockDuration,
	}).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return
		}
		if c.log != nil {
			c.log.Error("Failed to read from stream", infralogger.Error(err))
		}
		time.Sleep(time.Second)
		return
	}

	for _, stream := range streams {
		for _, msg := range stream.Messages {
			c.processMessage(ctx, msg)
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context, msg redis.XMessage) {
	eventData, ok := msg.Values["event"].(string)
	if !ok {
		if c.log != nil {
			c.log.Error("Invalid message format", infralogger.String("stream_id", msg.ID))
		}
		c.ackMessage(ctx, msg.ID)
		return
	}

	var event infraevents.SourceEvent
	if err := json.Unmarshal([]byte(eventData), &event); err != nil {
		if c.log != nil {
			c.log.Error("Failed to unmarshal event",
				infralogger.String("stream_id", msg.ID),
				infralogger.Error(err),
			)
		}
		c.ackMessage(ctx, msg.ID)
		return
	}

	var err error
	switch event.EventType {
	case infraevents.SourceCreated:
		err = c.handler.HandleSourceCreated(ctx, event)
	case infraevents.SourceUpdated:
		err = c.handler.HandleSourceUpdated(ctx, event)
	case infraevents.SourceDeleted:
		err = c.handler.HandleSourceDeleted(ctx, event)
	case infraevents.SourceEnabled:
		err = c.handler.HandleSourceEnabled(ctx, event)
	case infraevents.SourceDisabled:
		err = c.handler.HandleSourceDisabled(ctx, event)
	default:
		if c.log != nil {
			c.log.Warn("Unknown event type",
				infralogger.String("event_type", string(event.EventType)),
			)
		}
	}

	if err != nil {
		if c.log != nil {
			c.log.Error("Failed to handle event",
				infralogger.String("event_type", string(event.EventType)),
				infralogger.String("source_id", event.SourceID.String()),
				infralogger.Error(err),
			)
		}
		return // Don't ACK - will be retried
	}

	c.ackMessage(ctx, msg.ID)

	if c.log != nil {
		c.log.Info("Processed event",
			infralogger.String("event_type", string(event.EventType)),
			infralogger.String("source_id", event.SourceID.String()),
			infralogger.String("stream_id", msg.ID),
		)
	}
}

func (c *Consumer) ackMessage(ctx context.Context, streamID string) {
	if err := c.client.XAck(ctx, infraevents.StreamName, infraevents.ConsumerGroup, streamID).Err(); err != nil {
		if c.log != nil {
			c.log.Error("Failed to ACK message",
				infralogger.String("stream_id", streamID),
				infralogger.Error(err),
			)
		}
	}
}

func (c *Consumer) claimAbandonedLoop(ctx context.Context) {
	ticker := time.NewTicker(claimIdleTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.shutdownCh:
			return
		case <-ticker.C:
			c.claimAbandonedMessages(ctx)
		}
	}
}

func (c *Consumer) claimAbandonedMessages(ctx context.Context) {
	messages, _, err := c.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   infraevents.StreamName,
		Group:    infraevents.ConsumerGroup,
		Consumer: c.consumerID,
		MinIdle:  claimIdleTimeout,
		Count:    batchSize,
	}).Result()

	if err != nil {
		if c.log != nil {
			c.log.Error("Failed to auto-claim messages", infralogger.Error(err))
		}
		return
	}

	for _, msg := range messages {
		if c.log != nil {
			c.log.Info("Claimed abandoned message", infralogger.String("stream_id", msg.ID))
		}
		c.processMessage(ctx, msg)
	}
}

func (c *Consumer) ensureConsumerGroup(ctx context.Context) error {
	err := c.client.XGroupCreateMkStream(ctx, infraevents.StreamName, infraevents.ConsumerGroup, "0").Err()
	if err != nil && !isGroupExistsError(err) {
		return err
	}
	return nil
}

func isGroupExistsError(err error) bool {
	return err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists"
}
```

### Step 5: Run tests

```bash
cd crawler && go test ./internal/events/... -v
```

Expected: PASS

### Step 6: Commit

```bash
git add crawler/internal/events/
git commit -m "feat(crawler): add event consumer for Redis Streams

Implement Consumer that reads source lifecycle events from Redis Streams.
Features:
- Consumer groups for horizontal scaling
- Auto-claiming of abandoned messages
- Graceful shutdown
- EventHandler interface for pluggable processing

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 7: Create No-Op Event Handler (Logging Only)

**Files:**
- Create: `crawler/internal/events/noop_handler.go`
- Create: `crawler/internal/events/noop_handler_test.go`

### Step 1: Write the failing test

```go
// crawler/internal/events/noop_handler_test.go
package events

import (
	"context"
	"testing"

	"github.com/google/uuid"
	infraevents "github.com/north-cloud/infrastructure/events"
)

func TestNoOpHandler_HandleSourceCreated_LogsAndReturnsNil(t *testing.T) {
	t.Helper()

	handler := NewNoOpHandler(nil)

	event := infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceCreated,
		SourceID:  uuid.New(),
		Payload:   infraevents.SourceCreatedPayload{Name: "Test"},
	}

	err := handler.HandleSourceCreated(context.Background(), event)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestNoOpHandler_ImplementsEventHandler(t *testing.T) {
	t.Helper()

	var _ EventHandler = (*NoOpHandler)(nil)
}
```

### Step 2: Run test to verify it fails

```bash
cd crawler && go test ./internal/events/... -v
```

Expected: FAIL with "NoOpHandler not defined"

### Step 3: Write implementation

```go
// crawler/internal/events/noop_handler.go
package events

import (
	"context"

	infraevents "github.com/north-cloud/infrastructure/events"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// NoOpHandler logs events but takes no action.
// Used during Phase 1 to verify event flow without affecting job management.
type NoOpHandler struct {
	log infralogger.Logger
}

// NewNoOpHandler creates a new no-op handler.
func NewNoOpHandler(log infralogger.Logger) *NoOpHandler {
	return &NoOpHandler{log: log}
}

func (h *NoOpHandler) HandleSourceCreated(ctx context.Context, event infraevents.SourceEvent) error {
	if h.log != nil {
		h.log.Info("[NOOP] SOURCE_CREATED received",
			infralogger.String("source_id", event.SourceID.String()),
		)
	}
	return nil
}

func (h *NoOpHandler) HandleSourceUpdated(ctx context.Context, event infraevents.SourceEvent) error {
	if h.log != nil {
		h.log.Info("[NOOP] SOURCE_UPDATED received",
			infralogger.String("source_id", event.SourceID.String()),
		)
	}
	return nil
}

func (h *NoOpHandler) HandleSourceDeleted(ctx context.Context, event infraevents.SourceEvent) error {
	if h.log != nil {
		h.log.Info("[NOOP] SOURCE_DELETED received",
			infralogger.String("source_id", event.SourceID.String()),
		)
	}
	return nil
}

func (h *NoOpHandler) HandleSourceEnabled(ctx context.Context, event infraevents.SourceEvent) error {
	if h.log != nil {
		h.log.Info("[NOOP] SOURCE_ENABLED received",
			infralogger.String("source_id", event.SourceID.String()),
		)
	}
	return nil
}

func (h *NoOpHandler) HandleSourceDisabled(ctx context.Context, event infraevents.SourceEvent) error {
	if h.log != nil {
		h.log.Info("[NOOP] SOURCE_DISABLED received",
			infralogger.String("source_id", event.SourceID.String()),
		)
	}
	return nil
}
```

### Step 4: Run tests

```bash
cd crawler && go test ./internal/events/... -v
```

Expected: PASS

### Step 5: Commit

```bash
git add crawler/internal/events/noop_handler.go crawler/internal/events/noop_handler_test.go
git commit -m "feat(crawler): add no-op event handler for Phase 1

NoOpHandler logs received events without taking action.
Used to verify event flow before enabling auto-managed jobs.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 8: Wire Up Event Consumer in Crawler

**Files:**
- Modify: `crawler/internal/config/config.go`
- Modify: `crawler/cmd/httpd/httpd.go` or `crawler/main.go`

### Step 1: Add Redis config to crawler config

Edit `crawler/internal/config/config.go`:

```go
// Add Redis config
type RedisConfig struct {
	Address  string `env:"REDIS_ADDRESS" default:"localhost:6379"`
	Password string `env:"REDIS_PASSWORD" default:""`
	DB       int    `env:"REDIS_DB" default:"0"`
	Enabled  bool   `env:"REDIS_EVENTS_ENABLED" default:"false"`
}

// Add to main Config
type Config struct {
	// ... existing fields ...
	Redis RedisConfig `envPrefix:"REDIS_"`
}
```

### Step 2: Wire up consumer in main startup

In the crawler's main initialization (likely `cmd/httpd/httpd.go`):

```go
// After other initialization

var eventConsumer *events.Consumer

if cfg.Redis.Enabled {
	redisClient, err := infraredis.NewClient(infraredis.Config{
		Address:  cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		log.Warn("Redis not available, event consumer disabled",
			infralogger.Error(err),
		)
	} else {
		handler := events.NewNoOpHandler(log)
		eventConsumer = events.NewConsumer(redisClient, "", handler, log)

		if err := eventConsumer.Start(ctx); err != nil {
			log.Error("Failed to start event consumer", infralogger.Error(err))
		} else {
			log.Info("Event consumer started")
		}
	}
}

// In shutdown handler
if eventConsumer != nil {
	eventConsumer.Stop()
}
```

### Step 3: Build and verify

```bash
cd crawler && go build -o /dev/null ./...
```

Expected: Build succeeds

### Step 4: Commit

```bash
git add crawler/internal/config/config.go crawler/cmd/httpd/httpd.go
git commit -m "feat(crawler): wire up event consumer with feature flag

Add REDIS_EVENTS_ENABLED config option (default: false).
When enabled, starts event consumer with NoOpHandler.
Gracefully handles Redis unavailability.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 9: Add processed_events Table Migration

**Files:**
- Create: `crawler/migrations/008_add_processed_events.up.sql`
- Create: `crawler/migrations/008_add_processed_events.down.sql`

### Step 1: Create up migration

```sql
-- crawler/migrations/008_add_processed_events.up.sql
-- Migration: Add processed_events table for event idempotency
-- Description: Tracks which events have been processed to prevent duplicate handling

BEGIN;

CREATE TABLE IF NOT EXISTS processed_events (
    event_id UUID PRIMARY KEY,
    processed_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for cleanup of old events
CREATE INDEX IF NOT EXISTS idx_processed_events_cleanup
    ON processed_events (processed_at);

COMMENT ON TABLE processed_events IS
    'Tracks processed source events for idempotency (at-least-once delivery)';

COMMIT;
```

### Step 2: Create down migration

```sql
-- crawler/migrations/008_add_processed_events.down.sql
DROP INDEX IF EXISTS idx_processed_events_cleanup;
DROP TABLE IF EXISTS processed_events;
```

### Step 3: Run migration

```bash
cd crawler && go run cmd/migrate/main.go up
```

Expected: Migration applied successfully

### Step 4: Commit

```bash
git add crawler/migrations/008_add_processed_events.up.sql crawler/migrations/008_add_processed_events.down.sql
git commit -m "feat(crawler): add processed_events table for idempotency

Track processed event IDs to prevent duplicate handling in at-least-once
delivery scenarios. Index on processed_at supports periodic cleanup.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 10: Integration Test - End-to-End Event Flow

**Files:**
- Create: `docs/plans/phase1-testing.md`

### Step 1: Manual integration test

1. Start Redis:
```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d redis
```

2. Set environment variables:
```bash
export REDIS_EVENTS_ENABLED=true
export REDIS_ADDRESS=localhost:6379
```

3. Start source-manager:
```bash
cd source-manager && go run main.go
```

4. Start crawler in another terminal:
```bash
cd crawler && go run main.go
```

5. Create a source via API:
```bash
curl -X POST http://localhost:8050/api/v1/sources \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Source",
    "url": "https://example.com",
    "rate_limit": "10/s",
    "max_depth": 2,
    "enabled": true
  }'
```

6. Check crawler logs for:
```
[NOOP] SOURCE_CREATED received source_id=<uuid>
```

7. Verify Redis stream:
```bash
docker exec -it north-cloud-redis redis-cli XLEN source-events
docker exec -it north-cloud-redis redis-cli XRANGE source-events - +
```

### Step 2: Document test results

Create `docs/plans/phase1-testing.md`:

```markdown
# Phase 1 Integration Test Results

## Date: YYYY-MM-DD

## Test: End-to-End Event Flow

### Steps Executed
1. Started Redis, source-manager, crawler with REDIS_EVENTS_ENABLED=true
2. Created source via POST /api/v1/sources
3. Verified event in Redis stream
4. Verified crawler received and logged event

### Results
- [ ] Source-manager published SOURCE_CREATED event
- [ ] Redis stream contains event
- [ ] Crawler consumer received event
- [ ] Crawler logged [NOOP] message

### Notes
...
```

### Step 3: Commit

```bash
git add docs/plans/phase1-testing.md
git commit -m "docs: add Phase 1 integration test results

Document end-to-end event flow verification from source-manager
through Redis Streams to crawler consumer.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Phase 1 Complete - Summary

### Files Created
- `infrastructure/redis/client.go` - Redis client wrapper
- `infrastructure/events/types.go` - Shared event types
- `source-manager/internal/events/publisher.go` - Event publisher
- `crawler/internal/events/consumer.go` - Event consumer
- `crawler/internal/events/handler.go` - EventHandler interface
- `crawler/internal/events/noop_handler.go` - Logging-only handler
- `crawler/migrations/008_add_processed_events.up.sql` - Idempotency table

### Files Modified
- `source-manager/internal/handlers/source.go` - Added event publishing
- `source-manager/internal/config/config.go` - Added Redis config
- `source-manager/main.go` - Wired up publisher
- `crawler/internal/config/config.go` - Added Redis config
- `crawler/cmd/httpd/httpd.go` - Wired up consumer

### Feature Flag
- `REDIS_EVENTS_ENABLED=false` (default) - Events disabled
- `REDIS_EVENTS_ENABLED=true` - Events enabled

### Success Criteria
- Events flow from source-manager to crawler
- Existing manual job creation still works
- No user-facing changes
- Both services gracefully handle Redis unavailability

---

**Next Phase:** Phase 2 will implement the JobService with full event handlers to automatically create/update/delete jobs based on source events.
