package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

const (
	// JobDataField is the field name for serialized job data in stream messages.
	JobDataField = "job"

	// MetadataField is the field name for additional metadata.
	MetadataField = "metadata"

	// EnqueuedAtField is the field name for enqueue timestamp.
	EnqueuedAtField = "enqueued_at"

	// Default max stream length to prevent unbounded growth.
	defaultMaxStreamLen = 10000
)

// Producer handles enqueueing jobs to Redis Streams.
type Producer struct {
	client       *StreamsClient
	maxStreamLen int64
}

// ProducerConfig holds configuration for the Producer.
type ProducerConfig struct {
	MaxStreamLen int64 // Maximum stream length (0 = no limit)
}

// NewProducer creates a new job producer.
func NewProducer(client *StreamsClient, cfg ProducerConfig) *Producer {
	maxLen := cfg.MaxStreamLen
	if maxLen <= 0 {
		maxLen = defaultMaxStreamLen
	}

	return &Producer{
		client:       client,
		maxStreamLen: maxLen,
	}
}

// JobMessage represents a job message in the stream.
type JobMessage struct {
	ID         string         `json:"id"`
	Job        *domain.Job    `json:"job"`
	EnqueuedAt time.Time      `json:"enqueued_at"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// Enqueue adds a job to the appropriate priority stream.
func (p *Producer) Enqueue(
	ctx context.Context, job *domain.Job, priority Priority, metadata map[string]any,
) (string, error) {
	if job == nil {
		return "", errors.New("job cannot be nil")
	}

	if !priority.IsValid() {
		priority = PriorityNormal
	}

	// Serialize the job
	jobData, marshalErr := json.Marshal(job)
	if marshalErr != nil {
		return "", fmt.Errorf("failed to serialize job: %w", marshalErr)
	}

	// Prepare message fields
	values := map[string]any{
		JobDataField:    string(jobData),
		EnqueuedAtField: time.Now().UTC().Format(time.RFC3339),
	}

	// Add metadata if provided
	if metadata != nil {
		metaData, metaMarshalErr := json.Marshal(metadata)
		if metaMarshalErr != nil {
			return "", fmt.Errorf("failed to serialize metadata: %w", metaMarshalErr)
		}
		values[MetadataField] = string(metaData)
	}

	// Add to the appropriate stream
	stream := p.client.StreamName(priority)
	messageID, addErr := p.client.XAdd(ctx, stream, values)
	if addErr != nil {
		return "", fmt.Errorf("failed to enqueue job to stream %s: %w", stream, addErr)
	}

	return messageID, nil
}

// EnqueueBatch adds multiple jobs to streams based on their priorities.
func (p *Producer) EnqueueBatch(ctx context.Context, jobs []*domain.Job, getPriority func(*domain.Job) Priority) ([]string, error) {
	if len(jobs) == 0 {
		return nil, nil
	}

	messageIDs := make([]string, 0, len(jobs))

	for _, job := range jobs {
		priority := PriorityNormal
		if getPriority != nil {
			priority = getPriority(job)
		}

		messageID, err := p.Enqueue(ctx, job, priority, nil)
		if err != nil {
			return messageIDs, fmt.Errorf("failed to enqueue job %s: %w", job.ID, err)
		}

		messageIDs = append(messageIDs, messageID)
	}

	return messageIDs, nil
}

// EnqueueWithTimeout adds a job with a context timeout.
func (p *Producer) EnqueueWithTimeout(ctx context.Context, job *domain.Job, priority Priority, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return p.Enqueue(ctx, job, priority, nil)
}

// TrimStream trims a stream to the maximum length.
func (p *Producer) TrimStream(ctx context.Context, priority Priority) error {
	stream := p.client.StreamName(priority)
	return p.client.XTrimMaxLen(ctx, stream, p.maxStreamLen)
}

// TrimAllStreams trims all priority streams to the maximum length.
func (p *Producer) TrimAllStreams(ctx context.Context) error {
	for _, priority := range AllPriorities() {
		if err := p.TrimStream(ctx, priority); err != nil {
			return fmt.Errorf("failed to trim stream %s: %w", priority.String(), err)
		}
	}
	return nil
}

// GetQueueDepth returns the current queue depth for a priority level.
func (p *Producer) GetQueueDepth(ctx context.Context, priority Priority) (int64, error) {
	stream := p.client.StreamName(priority)
	return p.client.XLen(ctx, stream)
}

// GetAllQueueDepths returns the queue depth for all priority levels.
func (p *Producer) GetAllQueueDepths(ctx context.Context) (map[Priority]int64, error) {
	depths := make(map[Priority]int64, len(AllPriorities()))

	for _, priority := range AllPriorities() {
		depth, err := p.GetQueueDepth(ctx, priority)
		if err != nil {
			return depths, fmt.Errorf("failed to get depth for %s: %w", priority.String(), err)
		}
		depths[priority] = depth
	}

	return depths, nil
}
