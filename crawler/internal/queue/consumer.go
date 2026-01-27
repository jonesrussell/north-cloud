package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/redis/go-redis/v9"
)

const (
	// Default consumer group name.
	defaultConsumerGroup = "scheduler"

	// Default block timeout for reading from streams.
	defaultBlockTimeout = 5 * time.Second

	// Default count of messages to read per batch.
	defaultBatchSize = 10

	// Default minimum idle time before claiming pending messages.
	defaultClaimMinIdle = 5 * time.Minute

	// Maximum pending messages to check at once.
	maxPendingCheck = 100

	// streamsPerPriority is the number of stream entries per priority (stream name + ">").
	streamsPerPriority = 2
)

// Consumer handles reading jobs from Redis Streams.
type Consumer struct {
	client        *StreamsClient
	consumerGroup string
	consumerID    string
	blockTimeout  time.Duration
	batchSize     int64
	claimMinIdle  time.Duration
}

// ConsumerConfig holds configuration for the Consumer.
type ConsumerConfig struct {
	ConsumerGroup string        // Consumer group name
	ConsumerID    string        // Unique consumer identifier
	BlockTimeout  time.Duration // Block timeout for reads (0 = default)
	BatchSize     int64         // Number of messages per read (0 = default)
	ClaimMinIdle  time.Duration // Min idle time before claiming (0 = default)
}

// ConsumedJob represents a job read from the queue.
type ConsumedJob struct {
	MessageID  string
	Job        *domain.Job
	Priority   Priority
	EnqueuedAt time.Time
	Metadata   map[string]any
}

// NewConsumer creates a new job consumer.
func NewConsumer(client *StreamsClient, cfg ConsumerConfig) (*Consumer, error) {
	if cfg.ConsumerID == "" {
		return nil, errors.New("consumer ID is required")
	}

	group := cfg.ConsumerGroup
	if group == "" {
		group = defaultConsumerGroup
	}

	blockTimeout := cfg.BlockTimeout
	if blockTimeout <= 0 {
		blockTimeout = defaultBlockTimeout
	}

	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}

	claimMinIdle := cfg.ClaimMinIdle
	if claimMinIdle <= 0 {
		claimMinIdle = defaultClaimMinIdle
	}

	return &Consumer{
		client:        client,
		consumerGroup: group,
		consumerID:    cfg.ConsumerID,
		blockTimeout:  blockTimeout,
		batchSize:     batchSize,
		claimMinIdle:  claimMinIdle,
	}, nil
}

// Initialize creates consumer groups for all priority streams.
func (c *Consumer) Initialize(ctx context.Context) error {
	for _, priority := range AllPriorities() {
		stream := c.client.StreamName(priority)
		if err := c.client.CreateConsumerGroup(ctx, stream, c.consumerGroup); err != nil {
			return fmt.Errorf("failed to create consumer group for %s: %w", stream, err)
		}
	}
	return nil
}

// Read reads jobs from streams, prioritizing high-priority jobs first.
// Returns consumed jobs and any error encountered.
func (c *Consumer) Read(ctx context.Context) ([]*ConsumedJob, error) {
	// First, check for pending messages that need to be reclaimed
	reclaimedJobs := c.reclaimPending(ctx)

	if len(reclaimedJobs) > 0 {
		return reclaimedJobs, nil
	}

	// Read from streams in priority order
	return c.readNewMessages(ctx)
}

// ReadFromPriority reads jobs from a specific priority stream only.
func (c *Consumer) ReadFromPriority(ctx context.Context, priority Priority) ([]*ConsumedJob, error) {
	stream := c.client.StreamName(priority)
	streams := []string{stream, ">"}

	messages, err := c.client.XReadGroup(ctx, c.consumerGroup, c.consumerID, streams, c.batchSize, c.blockTimeout)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil // No messages available
		}
		return nil, fmt.Errorf("failed to read from stream %s: %w", stream, err)
	}

	return c.parseMessages(messages, priority)
}

// Acknowledge acknowledges successful processing of a job.
func (c *Consumer) Acknowledge(ctx context.Context, job *ConsumedJob) error {
	if job == nil {
		return errors.New("job cannot be nil")
	}

	stream := c.client.StreamName(job.Priority)
	return c.client.XAck(ctx, stream, c.consumerGroup, job.MessageID)
}

// AcknowledgeBatch acknowledges multiple jobs at once.
func (c *Consumer) AcknowledgeBatch(ctx context.Context, jobs []*ConsumedJob) error {
	if len(jobs) == 0 {
		return nil
	}

	// Group by priority/stream
	byStream := make(map[Priority][]string)
	for _, job := range jobs {
		byStream[job.Priority] = append(byStream[job.Priority], job.MessageID)
	}

	// Acknowledge each stream's messages
	for priority, ids := range byStream {
		stream := c.client.StreamName(priority)
		if err := c.client.XAck(ctx, stream, c.consumerGroup, ids...); err != nil {
			return fmt.Errorf("failed to acknowledge messages in stream %s: %w", stream, err)
		}
	}

	return nil
}

// GetPendingCount returns the count of pending messages for a priority level.
func (c *Consumer) GetPendingCount(ctx context.Context, priority Priority) (int64, error) {
	stream := c.client.StreamName(priority)
	pending, err := c.client.XPending(ctx, stream, c.consumerGroup)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get pending count: %w", err)
	}
	return pending.Count, nil
}

// GetAllPendingCounts returns pending counts for all priority levels.
func (c *Consumer) GetAllPendingCounts(ctx context.Context) (map[Priority]int64, error) {
	counts := make(map[Priority]int64, len(AllPriorities()))

	for _, priority := range AllPriorities() {
		count, err := c.GetPendingCount(ctx, priority)
		if err != nil {
			return counts, err
		}
		counts[priority] = count
	}

	return counts, nil
}

// readNewMessages reads new messages from streams in priority order.
func (c *Consumer) readNewMessages(ctx context.Context) ([]*ConsumedJob, error) {
	// Build streams list: high, normal, low with ">" for each
	priorities := AllPriorities()
	streamCount := len(priorities) * streamsPerPriority
	streams := make([]string, 0, streamCount)
	for _, priority := range priorities {
		streams = append(streams, c.client.StreamName(priority))
	}
	for range priorities {
		streams = append(streams, ">") // Read new messages
	}

	messages, err := c.client.XReadGroup(ctx, c.consumerGroup, c.consumerID, streams, c.batchSize, c.blockTimeout)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil // No messages available
		}
		return nil, fmt.Errorf("failed to read from streams: %w", err)
	}

	return c.parseAllMessages(messages)
}

// reclaimPending attempts to reclaim pending messages that have exceeded the idle threshold.
func (c *Consumer) reclaimPending(ctx context.Context) []*ConsumedJob {
	var reclaimedJobs []*ConsumedJob

	for _, priority := range AllPriorities() {
		stream := c.client.StreamName(priority)

		// Get pending entries
		pending, err := c.client.XPendingExt(ctx, stream, c.consumerGroup, "-", "+", maxPendingCheck)
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			continue // Log and continue with other priorities
		}

		// Find messages that exceed idle threshold
		var idsToReclaim []string
		for _, entry := range pending {
			if entry.Idle >= c.claimMinIdle {
				idsToReclaim = append(idsToReclaim, entry.ID)
			}
		}

		if len(idsToReclaim) == 0 {
			continue
		}

		// Claim the messages
		claimedMessages, claimErr := c.client.XClaim(
			ctx, stream, c.consumerGroup, c.consumerID, c.claimMinIdle, idsToReclaim...,
		)
		if claimErr != nil {
			continue // Log and continue
		}

		// Parse reclaimed messages
		for _, msg := range claimedMessages {
			parsedJob, parseErr := c.parseMessage(msg, priority)
			if parseErr != nil {
				continue // Skip malformed messages
			}
			reclaimedJobs = append(reclaimedJobs, parsedJob)
		}
	}

	return reclaimedJobs
}

// parseAllMessages parses messages from all priority streams.
func (c *Consumer) parseAllMessages(streams []redis.XStream) ([]*ConsumedJob, error) {
	var jobs []*ConsumedJob

	priorities := AllPriorities()
	for i, stream := range streams {
		priority := priorities[i]
		streamJobs, err := c.parseMessages([]redis.XStream{stream}, priority)
		if err != nil {
			return jobs, err
		}
		jobs = append(jobs, streamJobs...)
	}

	return jobs, nil
}

// parseMessages parses messages from a single stream.
func (c *Consumer) parseMessages(streams []redis.XStream, priority Priority) ([]*ConsumedJob, error) {
	var jobs []*ConsumedJob

	for _, stream := range streams {
		for _, msg := range stream.Messages {
			job, err := c.parseMessage(msg, priority)
			if err != nil {
				// Skip malformed messages but log the error
				continue
			}
			jobs = append(jobs, job)
		}
	}

	return jobs, nil
}

// parseMessage parses a single stream message into a ConsumedJob.
func (c *Consumer) parseMessage(msg redis.XMessage, priority Priority) (*ConsumedJob, error) {
	jobData, ok := msg.Values[JobDataField].(string)
	if !ok {
		return nil, errors.New("missing or invalid job data")
	}

	var job domain.Job
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	consumedJob := &ConsumedJob{
		MessageID: msg.ID,
		Job:       &job,
		Priority:  priority,
	}

	// Parse enqueued_at timestamp
	if enqueuedStr, hasEnqueued := msg.Values[EnqueuedAtField].(string); hasEnqueued {
		if t, parseErr := time.Parse(time.RFC3339, enqueuedStr); parseErr == nil {
			consumedJob.EnqueuedAt = t
		}
	}

	// Parse metadata
	if metaStr, hasMeta := msg.Values[MetadataField].(string); hasMeta {
		var metadata map[string]any
		if unmarshalErr := json.Unmarshal([]byte(metaStr), &metadata); unmarshalErr == nil {
			consumedJob.Metadata = metadata
		}
	}

	return consumedJob, nil
}

// ConsumerGroup returns the consumer group name.
func (c *Consumer) ConsumerGroup() string {
	return c.consumerGroup
}

// ConsumerID returns the consumer ID.
func (c *Consumer) ConsumerID() string {
	return c.consumerID
}
