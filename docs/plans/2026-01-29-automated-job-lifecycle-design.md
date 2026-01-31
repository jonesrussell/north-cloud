# Automated Job Lifecycle Architecture

**Date:** 2026-01-29
**Status:** Draft
**Author:** Systems Architecture Review

## Overview

This document describes the architecture for automating crawl job management in the North Cloud platform. The goal is to eliminate manual job creation/management as the system scales beyond 300+ sources.

**Problem Statement:**
- Over 300 sources imported, with more added daily
- Manually creating and managing crawl jobs is not feasible
- Need an industry-standard, scalable, automated scheduling solution

**Solution:** Event-driven job lifecycle management using Redis Streams, with dynamic scheduling based on source metadata.

---

## 1. Event Model

### 1.1 Redis Streams Architecture

```
┌─────────────────┐      XADD         ┌─────────────────────────────────┐
│  source-manager │ ─────────────────►│  Redis Stream                   │
│                 │                   │  "source-events"                │
└─────────────────┘                   │                                 │
                                      │  Consumer Group: "crawler-workers"│
                                      └─────────────────────────────────┘
                                                      │
                                         XREADGROUP + XACK
                                                      │
                    ┌─────────────────────────────────┼─────────────────────────────────┐
                    ▼                                 ▼                                 ▼
            ┌───────────────┐                ┌───────────────┐                ┌───────────────┐
            │  crawler-1    │                │  crawler-2    │                │  crawler-N    │
            │  (consumer)   │                │  (consumer)   │                │  (consumer)   │
            └───────────────┘                └───────────────┘                └───────────────┘
```

**Stream name:** `source-events`
**Consumer group:** `crawler-workers`
**Consumer naming:** `crawler-{instance-id}` (e.g., `crawler-abc123`)

### 1.2 Event Types and JSON Schemas

```json
// Base envelope for all events
{
  "event_id": "uuid",           // Idempotency key
  "event_type": "string",       // SOURCE_CREATED | SOURCE_UPDATED | SOURCE_DELETED | SOURCE_ENABLED | SOURCE_DISABLED
  "source_id": "uuid",          // The source this event pertains to
  "timestamp": "RFC3339",       // When the event occurred
  "payload": {}                 // Event-specific data
}
```

**SOURCE_CREATED**
```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440001",
  "event_type": "SOURCE_CREATED",
  "source_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-01-29T10:30:00Z",
  "payload": {
    "name": "Example News",
    "url": "https://example.com",
    "rate_limit": 10,
    "max_depth": 3,
    "enabled": true,
    "selectors": {
      "title": "h1.article-title",
      "body": "div.article-body"
    },
    "priority": "normal"
  }
}
```

**SOURCE_UPDATED**
```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440002",
  "event_type": "SOURCE_UPDATED",
  "source_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-01-29T11:00:00Z",
  "payload": {
    "changed_fields": ["rate_limit", "max_depth"],
    "previous": {
      "rate_limit": 10,
      "max_depth": 3
    },
    "current": {
      "rate_limit": 5,
      "max_depth": 2
    }
  }
}
```

**SOURCE_DELETED**
```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440003",
  "event_type": "SOURCE_DELETED",
  "source_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-01-29T12:00:00Z",
  "payload": {
    "name": "Example News",
    "deletion_reason": "user_requested"
  }
}
```

**SOURCE_ENABLED / SOURCE_DISABLED**
```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440004",
  "event_type": "SOURCE_DISABLED",
  "source_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-01-29T13:00:00Z",
  "payload": {
    "reason": "rate_limit_exceeded",
    "disabled_by": "system"
  }
}
```

### 1.3 Publisher Implementation (source-manager)

**New package structure:**
```
source-manager/
├── internal/
│   ├── events/
│   │   ├── publisher.go      // Redis stream publisher
│   │   ├── types.go          // Event structs
│   │   └── middleware.go     // Handler middleware for auto-publish
│   ├── handler/
│   │   └── source_handler.go // Existing, modified to publish events
```

**events/types.go**
```go
package events

import (
    "time"

    "github.com/google/uuid"
)

type EventType string

const (
    SourceCreated  EventType = "SOURCE_CREATED"
    SourceUpdated  EventType = "SOURCE_UPDATED"
    SourceDeleted  EventType = "SOURCE_DELETED"
    SourceEnabled  EventType = "SOURCE_ENABLED"
    SourceDisabled EventType = "SOURCE_DISABLED"
)

const StreamName = "source-events"

type SourceEvent struct {
    EventID   uuid.UUID   `json:"event_id"`
    EventType EventType   `json:"event_type"`
    SourceID  uuid.UUID   `json:"source_id"`
    Timestamp time.Time   `json:"timestamp"`
    Payload   any         `json:"payload"`
}

type SourceCreatedPayload struct {
    Name      string         `json:"name"`
    URL       string         `json:"url"`
    RateLimit int            `json:"rate_limit"`
    MaxDepth  int            `json:"max_depth"`
    Enabled   bool           `json:"enabled"`
    Selectors map[string]any `json:"selectors"`
    Priority  string         `json:"priority"`
}

type SourceUpdatedPayload struct {
    ChangedFields []string       `json:"changed_fields"`
    Previous      map[string]any `json:"previous"`
    Current       map[string]any `json:"current"`
}

type SourceDeletedPayload struct {
    Name           string `json:"name"`
    DeletionReason string `json:"deletion_reason"`
}

type SourceTogglePayload struct {
    Reason     string `json:"reason"`
    ToggledBy  string `json:"toggled_by"` // "user" or "system"
}
```

**events/publisher.go**
```go
package events

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/redis/go-redis/v9"
    infralogger "github.com/north-cloud/infrastructure/logger"
)

type Publisher struct {
    client *redis.Client
    log    *infralogger.Logger
}

func NewPublisher(client *redis.Client, log *infralogger.Logger) *Publisher {
    return &Publisher{client: client, log: log}
}

func (p *Publisher) Publish(ctx context.Context, event SourceEvent) error {
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

    // XADD to Redis Stream
    // Using "*" for auto-generated stream ID
    result := p.client.XAdd(ctx, &redis.XAddArgs{
        Stream: StreamName,
        Values: map[string]any{
            "event": string(payload),
        },
    })

    if err := result.Err(); err != nil {
        p.log.Error("Failed to publish event",
            infralogger.String("event_type", string(event.EventType)),
            infralogger.String("source_id", event.SourceID.String()),
            infralogger.Error(err),
        )
        return fmt.Errorf("publish to stream: %w", err)
    }

    p.log.Info("Published source event",
        infralogger.String("event_type", string(event.EventType)),
        infralogger.String("source_id", event.SourceID.String()),
        infralogger.String("stream_id", result.Val()),
    )

    return nil
}

// PublishSourceCreated is a convenience method
func (p *Publisher) PublishSourceCreated(ctx context.Context, source Source) error {
    return p.Publish(ctx, SourceEvent{
        EventType: SourceCreated,
        SourceID:  source.ID,
        Payload: SourceCreatedPayload{
            Name:      source.Name,
            URL:       source.URL,
            RateLimit: source.RateLimit,
            MaxDepth:  source.MaxDepth,
            Enabled:   source.Enabled,
            Selectors: source.Selectors,
            Priority:  source.Priority,
        },
    })
}

// Similar methods for Update, Delete, Enable, Disable...
```

**Integration in source handler (pseudocode):**
```go
func (h *SourceHandler) CreateSource(c *gin.Context) {
    // ... validation, create in DB ...

    source, err := h.repo.Create(ctx, req)
    if err != nil {
        // handle error
        return
    }

    // Publish event AFTER successful DB commit
    // Use goroutine to not block response, but log failures
    go func() {
        pubCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        if err := h.publisher.PublishSourceCreated(pubCtx, source); err != nil {
            h.log.Error("Failed to publish SOURCE_CREATED event",
                infralogger.String("source_id", source.ID.String()),
                infralogger.Error(err),
            )
            // Event will be recovered via reconciliation (see resilience section)
        }
    }()

    c.JSON(http.StatusCreated, source)
}
```

---

## 2. Automated Job Lifecycle

### 2.1 Crawler Internal Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              CRAWLER SERVICE                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐    │
│  │  EventConsumer   │────►│   JobService     │────►│  JobRepository   │    │
│  │                  │     │                  │     │                  │    │
│  │  - XREADGROUP    │     │  - CreateJob     │     │  - Upsert        │    │
│  │  - XACK          │     │  - UpdateJob     │     │  - FindBySource  │    │
│  │  - Error handling│     │  - PauseJob      │     │  - Delete        │    │
│  │  - Retry logic   │     │  - ResumeJob     │     │  - UpdateStatus  │    │
│  └──────────────────┘     │  - DeleteJob     │     └──────────────────┘    │
│           │               │  - ComputeSchedule│              │              │
│           │               └──────────────────┘              │              │
│           │                        │                         │              │
│           ▼                        ▼                         ▼              │
│  ┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐    │
│  │  Redis Stream    │     │ScheduleComputer  │     │   PostgreSQL     │    │
│  │  (source-events) │     │                  │     │   (crawler DB)   │    │
│  └──────────────────┘     │  - Rate-based    │     └──────────────────┘    │
│                           │  - Priority-based│                              │
│                           │  - Backoff logic │                              │
│  ┌──────────────────┐     └──────────────────┘                              │
│  │IntervalScheduler │                                                       │
│  │  (existing)      │◄─────── Jobs table populated by EventConsumer         │
│  │  - Poll due jobs │                                                       │
│  │  - Execute crawls│                                                       │
│  └──────────────────┘                                                       │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 New Package Structure

```
crawler/
├── internal/
│   ├── events/
│   │   ├── consumer.go       // Redis stream consumer
│   │   ├── types.go          // Shared event types (or import from shared pkg)
│   │   └── handler.go        // Event type dispatch
│   ├── job/
│   │   ├── service.go        // JobService (business logic)
│   │   ├── repository.go     // JobRepository (existing, extended)
│   │   └── scheduler.go      // ScheduleComputer
│   ├── scheduler/
│   │   └── interval_scheduler.go  // Existing, minimal changes
```

### 2.3 Event Consumer Implementation

**events/consumer.go**
```go
package events

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
    infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
    StreamName    = "source-events"
    ConsumerGroup = "crawler-workers"

    // Processing constants
    blockDuration     = 5 * time.Second
    maxRetries        = 3
    claimIdleTimeout  = 30 * time.Second
)

type Consumer struct {
    client       *redis.Client
    consumerID   string
    handler      EventHandler
    log          *infralogger.Logger
    shutdownCh   chan struct{}
}

type EventHandler interface {
    HandleSourceCreated(ctx context.Context, event SourceEvent) error
    HandleSourceUpdated(ctx context.Context, event SourceEvent) error
    HandleSourceDeleted(ctx context.Context, event SourceEvent) error
    HandleSourceEnabled(ctx context.Context, event SourceEvent) error
    HandleSourceDisabled(ctx context.Context, event SourceEvent) error
}

func NewConsumer(client *redis.Client, consumerID string, handler EventHandler, log *infralogger.Logger) *Consumer {
    return &Consumer{
        client:     client,
        consumerID: consumerID,
        handler:    handler,
        log:        log,
        shutdownCh: make(chan struct{}),
    }
}

func (c *Consumer) Start(ctx context.Context) error {
    // Ensure consumer group exists
    if err := c.ensureConsumerGroup(ctx); err != nil {
        return fmt.Errorf("ensure consumer group: %w", err)
    }

    c.log.Info("Starting event consumer",
        infralogger.String("consumer_id", c.consumerID),
        infralogger.String("group", ConsumerGroup),
    )

    // Main consumption loop
    go c.consumeLoop(ctx)

    // Claim abandoned messages periodically
    go c.claimAbandonedLoop(ctx)

    return nil
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
    // XREADGROUP: Read new messages for this consumer
    streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
        Group:    ConsumerGroup,
        Consumer: c.consumerID,
        Streams:  []string{StreamName, ">"},  // ">" means only new messages
        Count:    10,
        Block:    blockDuration,
    }).Result()

    if err != nil {
        if errors.Is(err, redis.Nil) {
            return // No new messages, normal
        }
        c.log.Error("Failed to read from stream", infralogger.Error(err))
        time.Sleep(time.Second) // Back off on error
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
        c.log.Error("Invalid message format", infralogger.String("stream_id", msg.ID))
        c.ackMessage(ctx, msg.ID) // Ack to prevent reprocessing
        return
    }

    var event SourceEvent
    if err := json.Unmarshal([]byte(eventData), &event); err != nil {
        c.log.Error("Failed to unmarshal event",
            infralogger.String("stream_id", msg.ID),
            infralogger.Error(err),
        )
        c.ackMessage(ctx, msg.ID)
        return
    }

    // Dispatch to appropriate handler
    var err error
    switch event.EventType {
    case SourceCreated:
        err = c.handler.HandleSourceCreated(ctx, event)
    case SourceUpdated:
        err = c.handler.HandleSourceUpdated(ctx, event)
    case SourceDeleted:
        err = c.handler.HandleSourceDeleted(ctx, event)
    case SourceEnabled:
        err = c.handler.HandleSourceEnabled(ctx, event)
    case SourceDisabled:
        err = c.handler.HandleSourceDisabled(ctx, event)
    default:
        c.log.Warn("Unknown event type",
            infralogger.String("event_type", string(event.EventType)),
        )
    }

    if err != nil {
        c.log.Error("Failed to handle event",
            infralogger.String("event_type", string(event.EventType)),
            infralogger.String("source_id", event.SourceID.String()),
            infralogger.Error(err),
        )
        // Don't ACK - message will be retried or claimed by another consumer
        return
    }

    // Successfully processed - acknowledge
    c.ackMessage(ctx, msg.ID)

    c.log.Info("Processed event",
        infralogger.String("event_type", string(event.EventType)),
        infralogger.String("source_id", event.SourceID.String()),
        infralogger.String("stream_id", msg.ID),
    )
}

func (c *Consumer) ackMessage(ctx context.Context, streamID string) {
    if err := c.client.XAck(ctx, StreamName, ConsumerGroup, streamID).Err(); err != nil {
        c.log.Error("Failed to ACK message",
            infralogger.String("stream_id", streamID),
            infralogger.Error(err),
        )
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
    // XAUTOCLAIM: Claim messages that have been pending too long
    messages, _, err := c.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
        Stream:   StreamName,
        Group:    ConsumerGroup,
        Consumer: c.consumerID,
        MinIdle:  claimIdleTimeout,
        Count:    10,
    }).Result()

    if err != nil {
        c.log.Error("Failed to auto-claim messages", infralogger.Error(err))
        return
    }

    for _, msg := range messages {
        c.log.Info("Claimed abandoned message", infralogger.String("stream_id", msg.ID))
        c.processMessage(ctx, msg)
    }
}

func (c *Consumer) ensureConsumerGroup(ctx context.Context) error {
    // Create group starting from the beginning of the stream
    // MKSTREAM creates the stream if it doesn't exist
    err := c.client.XGroupCreateMkStream(ctx, StreamName, ConsumerGroup, "0").Err()
    if err != nil && !isGroupExistsError(err) {
        return err
    }
    return nil
}

func isGroupExistsError(err error) bool {
    return err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists"
}

func (c *Consumer) Shutdown() {
    close(c.shutdownCh)
}
```

### 2.4 Job Service Implementation

**job/service.go**
```go
package job

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/north-cloud/crawler/internal/events"
    infralogger "github.com/north-cloud/infrastructure/logger"
)

type Service struct {
    repo              Repository
    scheduleComputer  *ScheduleComputer
    sourceClient      SourceClient // HTTP client to source-manager
    log               *infralogger.Logger
}

// Repository interface for job persistence
type Repository interface {
    FindBySourceID(ctx context.Context, sourceID uuid.UUID) (*Job, error)
    Upsert(ctx context.Context, job *Job) error
    Delete(ctx context.Context, sourceID uuid.UUID) error
    UpdateStatus(ctx context.Context, sourceID uuid.UUID, status JobStatus) error
    RecordProcessedEvent(ctx context.Context, eventID uuid.UUID) error
    IsEventProcessed(ctx context.Context, eventID uuid.UUID) (bool, error)
}

// SourceClient fetches full source config
type SourceClient interface {
    GetSource(ctx context.Context, sourceID uuid.UUID) (*Source, error)
}

func NewService(repo Repository, scheduleComputer *ScheduleComputer, sourceClient SourceClient, log *infralogger.Logger) *Service {
    return &Service{
        repo:             repo,
        scheduleComputer: scheduleComputer,
        sourceClient:     sourceClient,
        log:              log,
    }
}

// HandleSourceCreated implements EventHandler
func (s *Service) HandleSourceCreated(ctx context.Context, event events.SourceEvent) error {
    // Idempotency check
    processed, err := s.repo.IsEventProcessed(ctx, event.EventID)
    if err != nil {
        return fmt.Errorf("check event processed: %w", err)
    }
    if processed {
        s.log.Debug("Event already processed, skipping",
            infralogger.String("event_id", event.EventID.String()),
        )
        return nil
    }

    payload, ok := event.Payload.(events.SourceCreatedPayload)
    if !ok {
        return fmt.Errorf("invalid payload type for SOURCE_CREATED")
    }

    // Skip disabled sources
    if !payload.Enabled {
        s.log.Info("Source created but disabled, skipping job creation",
            infralogger.String("source_id", event.SourceID.String()),
        )
        return s.repo.RecordProcessedEvent(ctx, event.EventID)
    }

    // Compute schedule based on source metadata
    schedule := s.scheduleComputer.ComputeSchedule(ScheduleInput{
        RateLimit: payload.RateLimit,
        MaxDepth:  payload.MaxDepth,
        Priority:  payload.Priority,
    })

    job := &Job{
        ID:               uuid.New(),
        SourceID:         event.SourceID,
        SourceName:       payload.Name,
        URL:              payload.URL,
        IntervalMinutes:  schedule.IntervalMinutes,
        IntervalType:     schedule.IntervalType,
        NextRunAt:        time.Now().Add(schedule.InitialDelay),
        Status:           JobStatusPending,
        AutoManaged:      true,  // Flag indicating this job is event-driven
        Priority:         schedule.Priority,
        MaxConcurrent:    schedule.MaxConcurrent,
        CreatedAt:        time.Now(),
        UpdatedAt:        time.Now(),
    }

    if err := s.repo.Upsert(ctx, job); err != nil {
        return fmt.Errorf("upsert job: %w", err)
    }

    // Record event as processed for idempotency
    if err := s.repo.RecordProcessedEvent(ctx, event.EventID); err != nil {
        s.log.Warn("Failed to record processed event",
            infralogger.String("event_id", event.EventID.String()),
            infralogger.Error(err),
        )
    }

    s.log.Info("Created auto-managed job",
        infralogger.String("source_id", event.SourceID.String()),
        infralogger.String("source_name", payload.Name),
        infralogger.Int("interval_minutes", schedule.IntervalMinutes),
    )

    return nil
}

// HandleSourceUpdated implements EventHandler
func (s *Service) HandleSourceUpdated(ctx context.Context, event events.SourceEvent) error {
    // Idempotency check
    processed, err := s.repo.IsEventProcessed(ctx, event.EventID)
    if err != nil {
        return fmt.Errorf("check event processed: %w", err)
    }
    if processed {
        return nil
    }

    payload, ok := event.Payload.(events.SourceUpdatedPayload)
    if !ok {
        return fmt.Errorf("invalid payload type for SOURCE_UPDATED")
    }

    // Fetch existing job
    job, err := s.repo.FindBySourceID(ctx, event.SourceID)
    if err != nil {
        return fmt.Errorf("find job: %w", err)
    }

    // If no job exists and source is now enabled, create one
    if job == nil {
        // Fetch full source to check enabled status
        source, err := s.sourceClient.GetSource(ctx, event.SourceID)
        if err != nil {
            return fmt.Errorf("fetch source: %w", err)
        }
        if source.Enabled {
            // Treat as creation
            return s.HandleSourceCreated(ctx, events.SourceEvent{
                EventID:   event.EventID,
                EventType: events.SourceCreated,
                SourceID:  event.SourceID,
                Timestamp: event.Timestamp,
                Payload: events.SourceCreatedPayload{
                    Name:      source.Name,
                    URL:       source.URL,
                    RateLimit: source.RateLimit,
                    MaxDepth:  source.MaxDepth,
                    Enabled:   source.Enabled,
                    Priority:  source.Priority,
                },
            })
        }
        return s.repo.RecordProcessedEvent(ctx, event.EventID)
    }

    // Check if schedule-affecting fields changed
    scheduleFields := []string{"rate_limit", "max_depth", "priority"}
    needsReschedule := false
    for _, field := range payload.ChangedFields {
        for _, sf := range scheduleFields {
            if field == sf {
                needsReschedule = true
                break
            }
        }
    }

    if needsReschedule {
        // Fetch full source for recomputation
        source, err := s.sourceClient.GetSource(ctx, event.SourceID)
        if err != nil {
            return fmt.Errorf("fetch source for reschedule: %w", err)
        }

        schedule := s.scheduleComputer.ComputeSchedule(ScheduleInput{
            RateLimit: source.RateLimit,
            MaxDepth:  source.MaxDepth,
            Priority:  source.Priority,
        })

        job.IntervalMinutes = schedule.IntervalMinutes
        job.IntervalType = schedule.IntervalType
        job.Priority = schedule.Priority
        job.MaxConcurrent = schedule.MaxConcurrent
        job.UpdatedAt = time.Now()

        if err := s.repo.Upsert(ctx, job); err != nil {
            return fmt.Errorf("update job schedule: %w", err)
        }

        s.log.Info("Updated job schedule",
            infralogger.String("source_id", event.SourceID.String()),
            infralogger.Int("new_interval", schedule.IntervalMinutes),
        )
    }

    return s.repo.RecordProcessedEvent(ctx, event.EventID)
}

// HandleSourceDeleted implements EventHandler
func (s *Service) HandleSourceDeleted(ctx context.Context, event events.SourceEvent) error {
    processed, err := s.repo.IsEventProcessed(ctx, event.EventID)
    if err != nil {
        return fmt.Errorf("check event processed: %w", err)
    }
    if processed {
        return nil
    }

    if err := s.repo.Delete(ctx, event.SourceID); err != nil {
        return fmt.Errorf("delete job: %w", err)
    }

    s.log.Info("Deleted job for deleted source",
        infralogger.String("source_id", event.SourceID.String()),
    )

    return s.repo.RecordProcessedEvent(ctx, event.EventID)
}

// HandleSourceDisabled implements EventHandler
func (s *Service) HandleSourceDisabled(ctx context.Context, event events.SourceEvent) error {
    processed, err := s.repo.IsEventProcessed(ctx, event.EventID)
    if err != nil {
        return fmt.Errorf("check event processed: %w", err)
    }
    if processed {
        return nil
    }

    if err := s.repo.UpdateStatus(ctx, event.SourceID, JobStatusPaused); err != nil {
        return fmt.Errorf("pause job: %w", err)
    }

    s.log.Info("Paused job for disabled source",
        infralogger.String("source_id", event.SourceID.String()),
    )

    return s.repo.RecordProcessedEvent(ctx, event.EventID)
}

// HandleSourceEnabled implements EventHandler
func (s *Service) HandleSourceEnabled(ctx context.Context, event events.SourceEvent) error {
    processed, err := s.repo.IsEventProcessed(ctx, event.EventID)
    if err != nil {
        return fmt.Errorf("check event processed: %w", err)
    }
    if processed {
        return nil
    }

    job, err := s.repo.FindBySourceID(ctx, event.SourceID)
    if err != nil {
        return fmt.Errorf("find job: %w", err)
    }

    // If no job exists, create one
    if job == nil {
        source, err := s.sourceClient.GetSource(ctx, event.SourceID)
        if err != nil {
            return fmt.Errorf("fetch source: %w", err)
        }

        return s.HandleSourceCreated(ctx, events.SourceEvent{
            EventID:   event.EventID,
            EventType: events.SourceCreated,
            SourceID:  event.SourceID,
            Timestamp: event.Timestamp,
            Payload: events.SourceCreatedPayload{
                Name:      source.Name,
                URL:       source.URL,
                RateLimit: source.RateLimit,
                MaxDepth:  source.MaxDepth,
                Enabled:   true,
                Priority:  source.Priority,
            },
        })
    }

    // Resume existing job
    job.Status = JobStatusPending
    job.NextRunAt = time.Now() // Run soon
    job.UpdatedAt = time.Now()

    if err := s.repo.Upsert(ctx, job); err != nil {
        return fmt.Errorf("resume job: %w", err)
    }

    s.log.Info("Resumed job for enabled source",
        infralogger.String("source_id", event.SourceID.String()),
    )

    return s.repo.RecordProcessedEvent(ctx, event.EventID)
}
```

---

## 3. Dynamic Scheduling Strategy

### 3.1 Schedule Computation Logic

**job/scheduler.go**
```go
package job

import (
    "time"
)

// Priority levels
const (
    PriorityLow      = "low"
    PriorityNormal   = "normal"
    PriorityHigh     = "high"
    PriorityCritical = "critical"
)

// Base intervals by priority (in minutes)
var priorityBaseIntervals = map[string]int{
    PriorityCritical: 15,   // 4x/hour
    PriorityHigh:     30,   // 2x/hour
    PriorityNormal:   60,   // 1x/hour
    PriorityLow:      180,  // 3x/day
}

// Max concurrent crawls by priority
var priorityMaxConcurrent = map[string]int{
    PriorityCritical: 3,
    PriorityHigh:     2,
    PriorityNormal:   1,
    PriorityLow:      1,
}

type ScheduleComputer struct {
    // Could hold config, feature flags, etc.
}

type ScheduleInput struct {
    RateLimit     int    // requests per second allowed
    MaxDepth      int    // crawl depth
    Priority      string // low, normal, high, critical
    FailureCount  int    // consecutive failures (for backoff)
    LastFailureAt *time.Time
}

type ScheduleOutput struct {
    IntervalMinutes int
    IntervalType    string // "minutes", "hours"
    Priority        int    // numeric priority for scheduling order (higher = sooner)
    MaxConcurrent   int
    InitialDelay    time.Duration
}

func NewScheduleComputer() *ScheduleComputer {
    return &ScheduleComputer{}
}

func (sc *ScheduleComputer) ComputeSchedule(input ScheduleInput) ScheduleOutput {
    // Start with base interval from priority
    priority := input.Priority
    if priority == "" {
        priority = PriorityNormal
    }

    baseInterval := priorityBaseIntervals[priority]
    if baseInterval == 0 {
        baseInterval = priorityBaseIntervals[PriorityNormal]
    }

    // Adjust for rate limit
    // Lower rate limit = more polite = longer intervals
    intervalMinutes := sc.adjustForRateLimit(baseInterval, input.RateLimit)

    // Adjust for max depth
    // Deeper crawls take longer, space them out more
    intervalMinutes = sc.adjustForDepth(intervalMinutes, input.MaxDepth)

    // Apply exponential backoff if there are failures
    intervalMinutes = sc.applyBackoff(intervalMinutes, input.FailureCount)

    // Determine interval type for readability
    intervalType := "minutes"
    if intervalMinutes >= 60 && intervalMinutes%60 == 0 {
        intervalType = "hours"
    }

    // Calculate numeric priority for scheduler ordering
    numericPriority := sc.toNumericPriority(priority)

    return ScheduleOutput{
        IntervalMinutes: intervalMinutes,
        IntervalType:    intervalType,
        Priority:        numericPriority,
        MaxConcurrent:   priorityMaxConcurrent[priority],
        InitialDelay:    sc.computeInitialDelay(priority),
    }
}

func (sc *ScheduleComputer) adjustForRateLimit(baseInterval, rateLimit int) int {
    if rateLimit <= 0 {
        rateLimit = 10 // default
    }

    // Lower rate limits need longer intervals to be polite
    // rate_limit 1-5:   +50% interval
    // rate_limit 6-10:  base interval
    // rate_limit 11-20: -25% interval
    // rate_limit 20+:   -50% interval

    switch {
    case rateLimit <= 5:
        return baseInterval * 3 / 2
    case rateLimit <= 10:
        return baseInterval
    case rateLimit <= 20:
        return baseInterval * 3 / 4
    default:
        return baseInterval / 2
    }
}

func (sc *ScheduleComputer) adjustForDepth(interval, maxDepth int) int {
    if maxDepth <= 0 {
        maxDepth = 1
    }

    // Deeper crawls = more pages = longer runtime = space them out
    // depth 1-2: base
    // depth 3-5: +25%
    // depth 6+:  +50%

    switch {
    case maxDepth <= 2:
        return interval
    case maxDepth <= 5:
        return interval * 5 / 4
    default:
        return interval * 3 / 2
    }
}

func (sc *ScheduleComputer) applyBackoff(interval, failureCount int) int {
    if failureCount <= 0 {
        return interval
    }

    // Exponential backoff: interval * 2^failures, capped at 24 hours
    const maxBackoffMinutes = 24 * 60

    backoffInterval := interval
    for i := 0; i < failureCount && backoffInterval < maxBackoffMinutes; i++ {
        backoffInterval *= 2
    }

    if backoffInterval > maxBackoffMinutes {
        backoffInterval = maxBackoffMinutes
    }

    return backoffInterval
}

func (sc *ScheduleComputer) toNumericPriority(priority string) int {
    // Higher number = higher priority = scheduled sooner
    switch priority {
    case PriorityCritical:
        return 100
    case PriorityHigh:
        return 75
    case PriorityNormal:
        return 50
    case PriorityLow:
        return 25
    default:
        return 50
    }
}

func (sc *ScheduleComputer) computeInitialDelay(priority string) time.Duration {
    // Stagger initial runs to avoid thundering herd
    switch priority {
    case PriorityCritical:
        return 0 // Run immediately
    case PriorityHigh:
        return 1 * time.Minute
    case PriorityNormal:
        return 5 * time.Minute
    default:
        return 10 * time.Minute
    }
}

// Future extension: Crawl windows
type CrawlWindow struct {
    StartHour int // 0-23 UTC
    EndHour   int // 0-23 UTC
    DaysOfWeek []time.Weekday
}

func (sc *ScheduleComputer) IsWithinCrawlWindow(window *CrawlWindow, t time.Time) bool {
    if window == nil {
        return true // No window = always allowed
    }

    hour := t.UTC().Hour()
    day := t.UTC().Weekday()

    // Check day of week
    if len(window.DaysOfWeek) > 0 {
        dayAllowed := false
        for _, d := range window.DaysOfWeek {
            if d == day {
                dayAllowed = true
                break
            }
        }
        if !dayAllowed {
            return false
        }
    }

    // Check hour range (handles overnight windows)
    if window.StartHour <= window.EndHour {
        return hour >= window.StartHour && hour < window.EndHour
    }
    // Overnight window (e.g., 22:00 - 06:00)
    return hour >= window.StartHour || hour < window.EndHour
}
```

### 3.2 Database Schema Updates

**Migration: `004_add_auto_managed_jobs.up.sql`**
```sql
-- Add columns for auto-managed job lifecycle
ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS auto_managed BOOLEAN DEFAULT false,
    ADD COLUMN IF NOT EXISTS priority INTEGER DEFAULT 50,
    ADD COLUMN IF NOT EXISTS max_concurrent INTEGER DEFAULT 1,
    ADD COLUMN IF NOT EXISTS failure_count INTEGER DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_failure_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS backoff_until TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS crawl_window_start INTEGER,  -- Hour 0-23 UTC
    ADD COLUMN IF NOT EXISTS crawl_window_end INTEGER;    -- Hour 0-23 UTC

-- Index for efficient due job queries with priority ordering
CREATE INDEX IF NOT EXISTS idx_jobs_due_priority
    ON jobs (next_run_at, priority DESC)
    WHERE status = 'pending' AND (backoff_until IS NULL OR backoff_until < NOW());

-- Table for event idempotency tracking
CREATE TABLE IF NOT EXISTS processed_events (
    event_id UUID PRIMARY KEY,
    processed_at TIMESTAMPTZ DEFAULT NOW()
);

-- Auto-cleanup old processed events (keep 7 days)
CREATE INDEX IF NOT EXISTS idx_processed_events_cleanup
    ON processed_events (processed_at);

-- Comment for documentation
COMMENT ON COLUMN jobs.auto_managed IS 'True if job is managed by event-driven automation';
COMMENT ON COLUMN jobs.priority IS 'Numeric priority 0-100, higher = scheduled sooner';
COMMENT ON COLUMN jobs.backoff_until IS 'Do not run until this time (failure backoff)';
```

**Migration: `004_add_auto_managed_jobs.down.sql`**
```sql
DROP INDEX IF EXISTS idx_jobs_due_priority;
DROP INDEX IF EXISTS idx_processed_events_cleanup;
DROP TABLE IF EXISTS processed_events;

ALTER TABLE jobs
    DROP COLUMN IF EXISTS auto_managed,
    DROP COLUMN IF EXISTS priority,
    DROP COLUMN IF EXISTS max_concurrent,
    DROP COLUMN IF EXISTS failure_count,
    DROP COLUMN IF EXISTS last_failure_at,
    DROP COLUMN IF EXISTS backoff_until,
    DROP COLUMN IF EXISTS crawl_window_start,
    DROP COLUMN IF EXISTS crawl_window_end;
```

### 3.3 IntervalScheduler Modifications

The existing `IntervalScheduler` requires minimal changes:

```go
// Modified query in interval_scheduler.go
func (s *IntervalScheduler) findDueJobs(ctx context.Context) ([]*Job, error) {
    query := `
        SELECT id, source_id, source_name, url, interval_minutes, interval_type,
               next_run_at, status, priority, max_concurrent, failure_count,
               backoff_until, crawl_window_start, crawl_window_end
        FROM jobs
        WHERE status = 'pending'
          AND next_run_at <= $1
          AND (backoff_until IS NULL OR backoff_until <= $1)
          AND lock_token IS NULL
        ORDER BY priority DESC, next_run_at ASC
        LIMIT $2
        FOR UPDATE SKIP LOCKED
    `
    // ... rest of implementation
}

// Add crawl window check before execution
func (s *IntervalScheduler) shouldExecute(job *Job) bool {
    if job.CrawlWindowStart != nil && job.CrawlWindowEnd != nil {
        now := time.Now().UTC()
        hour := now.Hour()

        start := *job.CrawlWindowStart
        end := *job.CrawlWindowEnd

        if start <= end {
            if hour < start || hour >= end {
                return false
            }
        } else {
            // Overnight window
            if hour < start && hour >= end {
                return false
            }
        }
    }
    return true
}

// Add failure handling after crawl completes
func (s *IntervalScheduler) handleCrawlResult(ctx context.Context, job *Job, err error) {
    if err != nil {
        job.FailureCount++
        job.LastFailureAt = timePtr(time.Now())

        // Compute backoff
        backoffMinutes := job.IntervalMinutes * (1 << job.FailureCount) // 2^failures
        if backoffMinutes > 24*60 {
            backoffMinutes = 24 * 60 // Cap at 24 hours
        }
        job.BackoffUntil = timePtr(time.Now().Add(time.Duration(backoffMinutes) * time.Minute))

        s.log.Warn("Crawl failed, applying backoff",
            infralogger.String("source_id", job.SourceID.String()),
            infralogger.Int("failure_count", job.FailureCount),
            infralogger.Int("backoff_minutes", backoffMinutes),
        )
    } else {
        // Reset failure state on success
        job.FailureCount = 0
        job.LastFailureAt = nil
        job.BackoffUntil = nil
    }

    // Update job in database
    // ...
}
```

---

## 4. Dashboard Integration

### 4.1 UI Layout and Screens

**Main Dashboard: Source Operations Center**

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  NORTH CLOUD - SOURCE OPERATIONS                              [User ▼]      │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  HEALTH OVERVIEW                                           [24h ▼]   │   │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐   │   │
│  │  │   312   │  │   298   │  │    8    │  │    6    │  │  94.2%  │   │   │
│  │  │ Sources │  │ Healthy │  │ Warning │  │ Failed  │  │ Success │   │   │
│  │  │  Total  │  │  Jobs   │  │  Jobs   │  │  Jobs   │  │  Rate   │   │   │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘  └─────────┘   │   │
│  │                                                                     │   │
│  │  Throughput: ████████████████████░░░░░░░░  4,231 articles/hr       │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  SOURCES                                      [+ Add] [⬆ Import]    │   │
│  │  ┌──────────────────────────────────────────────────────────────┐   │   │
│  │  │ 🔍 Search sources...          [Status ▼] [Priority ▼] [■ ■]  │   │   │
│  │  └──────────────────────────────────────────────────────────────┘   │   │
│  │  ┌──────────────────────────────────────────────────────────────┐   │   │
│  │  │ □ │ Source              │ Status  │ Last Run  │ Next │ 24h  │   │   │
│  │  ├───┼─────────────────────┼─────────┼───────────┼──────┼──────┤   │   │
│  │  │ □ │ Example News        │ ● OK    │ 5m ago    │ 25m  │  47  │   │   │
│  │  │ □ │ Tech Daily          │ ● OK    │ 12m ago   │ 18m  │ 123  │   │   │
│  │  │ □ │ Finance Wire        │ ◐ Warn  │ 2h ago    │ 30m  │  12  │ ⚠ │   │
│  │  │ □ │ Local Tribune       │ ● Fail  │ 6h ago    │ --   │   0  │ ❌ │   │
│  │  │ □ │ Sports Hub          │ ○ Pause │ 1d ago    │ --   │  --  │   │   │
│  │  │ □ │ World Report        │ ● OK    │ 8m ago    │ 52m  │  89  │   │   │
│  │  │   │ ...                 │         │           │      │      │   │   │
│  │  └──────────────────────────────────────────────────────────────┘   │   │
│  │  [Select All] [With Selected: Pause ▼ | Resume | Force Run | ...]   │   │
│  │                                                      Page 1 of 16   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Source Detail View (Slide-out Panel)**

```
┌────────────────────────────────────────────────────┐
│  Example News                              [✕]     │
├────────────────────────────────────────────────────┤
│  Status: ● Healthy    Priority: High               │
│  URL: https://example.com/news                     │
│                                                    │
│  ┌──────────────────────────────────────────────┐ │
│  │  JOB CONFIGURATION                            │ │
│  │  ─────────────────────────────────────────── │ │
│  │  Mode:       🤖 Auto-managed                  │ │
│  │  Interval:   30 minutes (computed)            │ │
│  │  Rate Limit: 10 req/s                         │ │
│  │  Max Depth:  3 levels                         │ │
│  │                                               │ │
│  │  [Override Interval] [Switch to Manual]       │ │
│  └──────────────────────────────────────────────┘ │
│                                                    │
│  ┌──────────────────────────────────────────────┐ │
│  │  SCHEDULE                                     │ │
│  │  ─────────────────────────────────────────── │ │
│  │  Last Run:     2026-01-29 10:15 UTC (5m ago) │ │
│  │  Next Run:     2026-01-29 10:45 UTC (in 25m) │ │
│  │  Crawl Window: None (always active)          │ │
│  │                                               │ │
│  │  [▶ Force Run Now] [⏸ Pause]                 │ │
│  └──────────────────────────────────────────────┘ │
│                                                    │
│  ┌──────────────────────────────────────────────┐ │
│  │  HEALTH (Last 24 hours)                       │ │
│  │  ─────────────────────────────────────────── │ │
│  │  Runs:     47 successful, 1 failed            │ │
│  │  Articles: 892 extracted                      │ │
│  │  Avg Time: 12.3 seconds                       │ │
│  │  Failures: 1 (rate limited - auto-recovered) │ │
│  │                                               │ │
│  │  Success Rate ███████████████████░ 97.9%     │ │
│  └──────────────────────────────────────────────┘ │
│                                                    │
│  ┌──────────────────────────────────────────────┐ │
│  │  RECENT EXECUTIONS                    [All →] │ │
│  │  ─────────────────────────────────────────── │ │
│  │  10:15  ● Success  47 articles  12.1s        │ │
│  │  09:45  ● Success  38 articles  11.8s        │ │
│  │  09:15  ● Success  41 articles  13.2s        │ │
│  │  08:45  ● Failed   Rate limit exceeded       │ │
│  │  08:14  ● Success  52 articles  12.0s        │ │
│  └──────────────────────────────────────────────┘ │
│                                                    │
│  ┌──────────────────────────────────────────────┐ │
│  │  EVENT LOG                            [All →] │ │
│  │  ─────────────────────────────────────────── │ │
│  │  Jan 28  SOURCE_UPDATED  rate_limit: 5→10   │ │
│  │  Jan 25  SOURCE_CREATED  job auto-created    │ │
│  └──────────────────────────────────────────────┘ │
│                                                    │
└────────────────────────────────────────────────────┘
```

### 4.2 Required API Endpoints

**source-manager (existing + new)**

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v1/sources` | GET | List sources with pagination, filters |
| `/api/v1/sources/:id` | GET | Get single source |
| `/api/v1/sources` | POST | Create source (publishes SOURCE_CREATED) |
| `/api/v1/sources/:id` | PUT | Update source (publishes SOURCE_UPDATED) |
| `/api/v1/sources/:id` | DELETE | Delete source (publishes SOURCE_DELETED) |
| `/api/v1/sources/:id/enable` | POST | **NEW** Enable source (publishes SOURCE_ENABLED) |
| `/api/v1/sources/:id/disable` | POST | **NEW** Disable source (publishes SOURCE_DISABLED) |
| `/api/v1/sources/bulk` | POST | **NEW** Bulk operations (enable/disable/delete) |

**crawler (existing + new)**

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v1/jobs` | GET | List jobs with source enrichment |
| `/api/v1/jobs/:id` | GET | Get job details |
| `/api/v1/jobs/stats` | GET | **NEW** Aggregate statistics |
| `/api/v1/jobs/by-source/:source_id` | GET | **NEW** Get job for a source |
| `/api/v1/jobs/:id/pause` | POST | Pause a job |
| `/api/v1/jobs/:id/resume` | POST | Resume a job |
| `/api/v1/jobs/:id/force-run` | POST | **NEW** Trigger immediate execution |
| `/api/v1/jobs/:id/override` | PUT | **NEW** Override auto-computed settings |
| `/api/v1/jobs/bulk` | POST | **NEW** Bulk operations |
| `/api/v1/sources/:source_id/executions` | GET | Execution history for a source |
| `/api/v1/events/log` | GET | **NEW** Event processing log (for debugging) |

**New Response Formats**

```json
// GET /api/v1/jobs/stats
{
  "total_sources": 312,
  "total_jobs": 312,
  "jobs_by_status": {
    "pending": 298,
    "running": 6,
    "paused": 5,
    "failed": 3
  },
  "health": {
    "healthy": 298,
    "warning": 8,
    "failed": 6
  },
  "last_24h": {
    "total_runs": 4231,
    "successful_runs": 4189,
    "failed_runs": 42,
    "articles_extracted": 89432,
    "success_rate": 0.990
  },
  "throughput": {
    "articles_per_hour": 3726,
    "runs_per_hour": 176
  }
}

// GET /api/v1/jobs/by-source/:source_id (enriched)
{
  "job": {
    "id": "...",
    "source_id": "...",
    "source_name": "Example News",
    "url": "https://example.com",
    "status": "pending",
    "interval_minutes": 30,
    "next_run_at": "2026-01-29T10:45:00Z",
    "auto_managed": true,
    "priority": 75,
    "failure_count": 0
  },
  "health": {
    "status": "healthy",
    "last_run_at": "2026-01-29T10:15:00Z",
    "last_run_status": "success",
    "last_24h_runs": 47,
    "last_24h_failures": 1,
    "last_24h_articles": 892,
    "success_rate": 0.979,
    "avg_duration_seconds": 12.3
  },
  "recent_executions": [
    {
      "id": "...",
      "started_at": "2026-01-29T10:15:00Z",
      "completed_at": "2026-01-29T10:15:12Z",
      "status": "success",
      "articles_count": 47,
      "duration_seconds": 12.1
    }
  ]
}
```

### 4.3 Dashboard Data Model Changes

**source-manager: Add `priority` field to sources**

```sql
-- Migration for source-manager
ALTER TABLE sources
    ADD COLUMN IF NOT EXISTS priority VARCHAR(20) DEFAULT 'normal';

-- Valid values: 'low', 'normal', 'high', 'critical'
```

---

## 5. Resilience and Observability

### 5.1 Resilience Patterns

**1. Redis Outage Handling**

```go
// In event consumer - circuit breaker pattern
type ConsumerWithCircuitBreaker struct {
    consumer     *Consumer
    circuitOpen  atomic.Bool
    lastFailure  atomic.Value // time.Time
    cooldown     time.Duration
}

func (c *ConsumerWithCircuitBreaker) consumeWithFallback(ctx context.Context) {
    if c.circuitOpen.Load() {
        lastFail := c.lastFailure.Load().(time.Time)
        if time.Since(lastFail) > c.cooldown {
            c.circuitOpen.Store(false)
            c.log.Info("Circuit breaker reset, resuming event consumption")
        } else {
            return // Still in cooldown
        }
    }

    err := c.consumer.readAndProcess(ctx)
    if err != nil {
        if isRedisConnectionError(err) {
            c.circuitOpen.Store(true)
            c.lastFailure.Store(time.Now())
            c.log.Error("Redis connection failed, circuit breaker opened",
                infralogger.Error(err),
                infralogger.Duration("cooldown", c.cooldown),
            )
        }
    }
}
```

**2. Reconciliation Job (Fallback for Lost Events)**

```go
// Run periodically (e.g., every 5 minutes) to catch any missed events
func (s *Service) ReconcileSources(ctx context.Context) error {
    // Fetch all enabled sources from source-manager
    sources, err := s.sourceClient.ListSources(ctx, SourceFilter{Enabled: true})
    if err != nil {
        return fmt.Errorf("list sources: %w", err)
    }

    // Get all existing jobs
    jobs, err := s.repo.ListAutoManagedJobs(ctx)
    if err != nil {
        return fmt.Errorf("list jobs: %w", err)
    }

    jobsBySource := make(map[uuid.UUID]*Job)
    for _, j := range jobs {
        jobsBySource[j.SourceID] = j
    }

    // Create missing jobs
    for _, source := range sources {
        if _, exists := jobsBySource[source.ID]; !exists {
            s.log.Warn("Reconciliation: creating missing job",
                infralogger.String("source_id", source.ID.String()),
            )
            // Create job (same logic as HandleSourceCreated)
            // ...
        }
    }

    // Mark jobs for deleted sources
    sourceIDs := make(map[uuid.UUID]bool)
    for _, s := range sources {
        sourceIDs[s.ID] = true
    }

    for sourceID, job := range jobsBySource {
        if !sourceIDs[sourceID] {
            s.log.Warn("Reconciliation: orphaned job found",
                infralogger.String("source_id", sourceID.String()),
            )
            // Either delete or mark for review
            job.Status = JobStatusOrphaned
            // ...
        }
    }

    return nil
}
```

**3. Idempotent Operations**

Already shown in the `processed_events` table - every event handler checks this before processing.

### 5.2 Metrics (Prometheus Format)

```go
// metrics/metrics.go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Event metrics
    EventsReceived = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "crawler_events_received_total",
            Help: "Total source events received from Redis stream",
        },
        []string{"event_type"},
    )

    EventsProcessed = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "crawler_events_processed_total",
            Help: "Total source events successfully processed",
        },
        []string{"event_type", "result"}, // result: success, error, duplicate
    )

    EventProcessingDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "crawler_event_processing_duration_seconds",
            Help:    "Time to process a source event",
            Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5},
        },
        []string{"event_type"},
    )

    ConsumerLag = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "crawler_consumer_lag_messages",
            Help: "Number of unprocessed messages in the stream",
        },
    )

    // Job metrics
    JobsCreated = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "crawler_jobs_created_total",
            Help: "Total auto-managed jobs created",
        },
    )

    JobsUpdated = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "crawler_jobs_updated_total",
            Help: "Total auto-managed jobs updated",
        },
    )

    JobsByStatus = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "crawler_jobs_by_status",
            Help: "Current number of jobs by status",
        },
        []string{"status"},
    )

    JobsByPriority = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "crawler_jobs_by_priority",
            Help: "Current number of jobs by priority",
        },
        []string{"priority"},
    )

    // Scheduler metrics
    SchedulerRuns = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "crawler_scheduler_runs_total",
            Help: "Total scheduler poll cycles",
        },
    )

    SchedulerJobsDispatched = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "crawler_scheduler_jobs_dispatched_total",
            Help: "Total jobs dispatched by scheduler",
        },
    )

    CrawlDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "crawler_crawl_duration_seconds",
            Help:    "Duration of crawl executions",
            Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
        },
        []string{"source_name", "status"},
    )

    CrawlArticlesExtracted = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "crawler_articles_extracted_total",
            Help: "Total articles extracted by crawls",
        },
        []string{"source_name"},
    )
)
```

### 5.3 Logging Standards

```go
// Structured logging with consistent fields
s.log.Info("Event processed",
    infralogger.String("event_id", event.EventID.String()),
    infralogger.String("event_type", string(event.EventType)),
    infralogger.String("source_id", event.SourceID.String()),
    infralogger.Duration("processing_time", time.Since(startTime)),
)

s.log.Error("Job creation failed",
    infralogger.String("source_id", sourceID.String()),
    infralogger.Error(err),
    infralogger.String("action", "create_job"),
)
```

### 5.4 Grafana Dashboard Panels

**Recommended Dashboard Layout:**

1. **Row 1: Event Pipeline Health**
   - Events received rate (by type)
   - Events processed rate (success vs error)
   - Consumer lag
   - Processing duration P50/P95/P99

2. **Row 2: Job Lifecycle**
   - Jobs by status (stacked area)
   - Jobs created/updated rate
   - Failed jobs count
   - Backoff jobs count

3. **Row 3: Scheduler Performance**
   - Scheduler cycles per minute
   - Jobs dispatched rate
   - Queue depth (pending jobs)
   - Active crawls

4. **Row 4: Crawl Performance**
   - Crawl success rate
   - Articles extracted rate
   - Crawl duration heatmap
   - Top failing sources table

**Alert Rules:**

```yaml
# prometheus/alerts.yml
groups:
  - name: crawler_alerts
    rules:
      - alert: HighConsumerLag
        expr: crawler_consumer_lag_messages > 100
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Event consumer falling behind"

      - alert: HighEventProcessingErrors
        expr: rate(crawler_events_processed_total{result="error"}[5m]) > 0.1
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High event processing error rate"

      - alert: JobFailureSpike
        expr: increase(crawler_jobs_by_status{status="failed"}[1h]) > 10
        labels:
          severity: warning
        annotations:
          summary: "Unusual number of job failures"

      - alert: LowCrawlSuccessRate
        expr: rate(crawler_crawl_duration_seconds_count{status="success"}[1h]) / rate(crawler_crawl_duration_seconds_count[1h]) < 0.9
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "Crawl success rate below 90%"
```

---

## 6. Phased Implementation Plan

### Phase 1: Event Infrastructure (2-3 weeks)

**Goal:** Introduce event model without breaking existing functionality.

**Tasks:**
1. Add Redis Streams dependencies to source-manager and crawler
2. Create shared event types package (`infrastructure/events`)
3. Implement `Publisher` in source-manager
4. Implement `Consumer` skeleton in crawler
5. Add event publishing to source-manager handlers (behind feature flag)
6. Create `processed_events` table migration in crawler
7. Add basic metrics for event publishing/consumption
8. Manual testing of event flow

**Success Criteria:**
- Events flow from source-manager to crawler
- Existing manual job creation still works
- No user-facing changes

**Code Changes:**
```
source-manager/
├── internal/events/          # NEW
│   ├── publisher.go
│   └── types.go
├── internal/handler/
│   └── source_handler.go    # Modified to publish events

crawler/
├── internal/events/          # NEW
│   ├── consumer.go
│   └── handler.go
├── migrations/
│   └── 004_processed_events.sql  # NEW

infrastructure/
├── events/                   # NEW (shared types)
│   └── types.go
```

---

### Phase 2: Auto-Managed Jobs (2-3 weeks)

**Goal:** Automatically create and manage jobs for a subset of sources.

**Tasks:**
1. Add `auto_managed`, `priority`, and backoff columns to jobs table
2. Implement `JobService` with full event handlers
3. Implement `ScheduleComputer` with dynamic scheduling logic
4. Add `priority` column to sources table in source-manager
5. Modify `IntervalScheduler` to respect priority and backoff
6. Add feature flag: `AUTO_MANAGE_NEW_SOURCES=true`
7. Create reconciliation job (cron or background goroutine)
8. Add dashboard API: `GET /api/v1/jobs/by-source/:source_id`
9. Integration testing with 10-20 test sources

**Success Criteria:**
- New sources automatically get jobs created
- Source updates propagate to job configuration
- Disabled sources pause their jobs
- Existing manual jobs continue to work

**Feature Flag:**
```go
// config/config.go
type Config struct {
    // ...
    AutoManageNewSources bool `env:"AUTO_MANAGE_NEW_SOURCES" default:"false"`
}

// In source handler
if h.config.AutoManageNewSources {
    h.publisher.PublishSourceCreated(ctx, source)
}
```

---

### Phase 3: Full Migration (2-3 weeks)

**Goal:** Migrate all existing sources to auto-managed jobs.

**Tasks:**
1. Create migration script to convert existing jobs to auto-managed
2. Add `priority` field to all existing sources (default: normal)
3. Run reconciliation to ensure 1:1 source-to-job mapping
4. Deprecate manual job creation endpoint (return 410 Gone or redirect)
5. Update documentation
6. Remove feature flag, make auto-management the default
7. Monitor for regressions with extended soak period

**Migration Script:**
```sql
-- Mark all existing jobs as auto_managed if they have a valid source_id
UPDATE jobs j
SET auto_managed = true,
    priority = 50,  -- normal
    updated_at = NOW()
WHERE EXISTS (
    SELECT 1 FROM sources s WHERE s.id = j.source_id
)
AND auto_managed = false;

-- Set default priority on sources
UPDATE sources
SET priority = 'normal'
WHERE priority IS NULL;
```

**Success Criteria:**
- All 300+ sources have auto-managed jobs
- Zero manual job management required
- Successful 7-day soak period with no critical issues

---

### Phase 4: Dashboard Enhancement (2-3 weeks)

**Goal:** Full operational dashboard with health visibility and bulk operations.

**Tasks:**
1. Implement `GET /api/v1/jobs/stats` endpoint
2. Implement bulk operations endpoint
3. Build "Source Operations" main dashboard view
4. Build source detail slide-out panel
5. Add execution history view
6. Add event log view (for debugging)
7. Add Grafana dashboard JSON export for ops team
8. User acceptance testing with operations team

**Dashboard Components:**
```
dashboard/src/
├── views/
│   └── SourceOperations.vue     # NEW - main view
├── components/
│   ├── HealthOverview.vue       # NEW
│   ├── SourceTable.vue          # NEW
│   ├── SourceDetailPanel.vue    # NEW
│   ├── ExecutionHistory.vue     # NEW
│   └── BulkOperationsBar.vue    # NEW
├── composables/
│   └── useJobStats.ts           # NEW
├── types/
│   └── job.ts                   # NEW (JobStats, JobHealth, etc.)
```

**Success Criteria:**
- Operations team can view all 300+ sources at a glance
- Health status visible within 3 seconds of page load
- Bulk operations work for 50+ sources at once
- Grafana dashboards deployed and alerting configured

---

## 7. Summary

This architecture provides:

| Requirement | Solution |
|-------------|----------|
| **Automated job creation** | Redis Streams events trigger job lifecycle in crawler |
| **Dynamic scheduling** | ScheduleComputer uses rate_limit, depth, priority to compute intervals |
| **Self-healing** | Exponential backoff, failure tracking, auto-recovery on enable |
| **Horizontal scalability** | Consumer groups allow N crawler instances |
| **Observability** | Prometheus metrics, structured logging, Grafana dashboards |
| **Dashboard UX** | Health-first design, bulk ops, drill-down detail panels |

---

## 8. Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Redis outage causes event loss | Medium | High | Reconciliation job runs every 5 min; circuit breaker prevents cascading failures |
| Event storm during bulk import | Medium | Medium | Rate limiting on publisher; consumer processes in batches |
| Schedule computation bugs | Low | Medium | Comprehensive unit tests; gradual rollout via feature flag |
| Consumer lag under load | Medium | Medium | Horizontal scaling via consumer groups; lag alerting |
| Migration breaks existing jobs | Low | High | Phased rollout; backward-compatible schema changes; 7-day soak period |

---

## Appendix: Key Design Decisions

**Why Redis Streams over Pub/Sub:**
Redis Streams provide persistence, consumer groups, and message acknowledgment - essential for at-least-once delivery. With XREADGROUP + XACK, messages aren't lost if a consumer crashes. XAUTOCLAIM handles abandoned messages when a consumer instance dies.

**Idempotency pattern:**
The `processed_events` table ensures that even if the same event is delivered twice (at-least-once semantics), the operation is only performed once. This is crucial for event-driven systems where network partitions can cause redelivery.

**Priority-based scheduling:**
By ordering jobs by priority DESC in the scheduler query, high-priority sources always get scheduled first, even under load. The numeric priority (0-100) allows fine-grained control without changing the scheduler's core loop.
