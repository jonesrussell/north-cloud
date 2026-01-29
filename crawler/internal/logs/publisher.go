package logs

import (
	"context"
	"sync/atomic"

	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/sse"
)

// ssePublisher implements Publisher for SSE log streaming.
type ssePublisher struct {
	broker  sse.Broker
	logger  infralogger.Logger
	enabled atomic.Bool
}

// NewPublisher creates a new SSE log publisher.
func NewPublisher(broker sse.Broker, logger infralogger.Logger, enabled bool) Publisher {
	p := &ssePublisher{
		broker: broker,
		logger: logger,
	}
	p.enabled.Store(enabled)
	return p
}

// PublishLogLine publishes a single log line event to SSE subscribers.
func (p *ssePublisher) PublishLogLine(ctx context.Context, entry LogEntry) {
	if !p.enabled.Load() || p.broker == nil {
		return
	}

	event := sse.NewLogLineEvent(
		entry.JobID,
		entry.ExecID,
		entry.Level,
		entry.Category,
		entry.Message,
		entry.Fields,
	)

	if err := p.broker.Publish(ctx, event); err != nil {
		// Log at debug level - SSE failures shouldn't affect job execution
		p.logger.Debug("Failed to publish log line to SSE",
			infralogger.Error(err),
			infralogger.String("job_id", entry.JobID),
		)
	}
}

// PublishLogArchived publishes a log archived event to SSE subscribers.
func (p *ssePublisher) PublishLogArchived(ctx context.Context, metadata *LogMetadata) {
	if !p.enabled.Load() || p.broker == nil {
		return
	}

	event := sse.NewLogArchivedEvent(
		metadata.JobID,
		metadata.ExecutionID,
		metadata.ExecutionNumber,
		metadata.ObjectKey,
		metadata.SizeBytes,
		metadata.LineCount,
	)

	if err := p.broker.Publish(ctx, event); err != nil {
		p.logger.Debug("Failed to publish log archived event to SSE",
			infralogger.Error(err),
			infralogger.String("job_id", metadata.JobID),
		)
	}
}

// noopPublisher is a no-op implementation when SSE is disabled.
type noopPublisher struct{}

// NewNoopPublisher creates a publisher that does nothing.
func NewNoopPublisher() Publisher {
	return &noopPublisher{}
}

func (p *noopPublisher) PublishLogLine(_ context.Context, _ LogEntry)         {}
func (p *noopPublisher) PublishLogArchived(_ context.Context, _ *LogMetadata) {}
