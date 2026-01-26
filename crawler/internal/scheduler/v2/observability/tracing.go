package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	// TracerName is the name of the tracer for the V2 scheduler.
	TracerName = "github.com/jonesrussell/north-cloud/crawler/scheduler/v2"
)

// Tracer provides tracing capabilities for the V2 scheduler.
type Tracer struct {
	tracer trace.Tracer
}

// NewTracer creates a new tracer.
func NewTracer() *Tracer {
	return &Tracer{
		tracer: otel.Tracer(TracerName),
	}
}

// StartSpan starts a new span with the given name.
// Caller is responsible for calling span.End().
//
//nolint:spancheck // span is returned to caller who manages its lifecycle
func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

// JobSchedulingSpan starts a span for job scheduling operations.
// Caller is responsible for calling span.End().
//
//nolint:spancheck // span is returned to caller who manages its lifecycle
func (t *Tracer) JobSchedulingSpan(ctx context.Context, jobID, scheduleType string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, "scheduler.schedule_job",
		trace.WithAttributes(
			attribute.String("job.id", jobID),
			attribute.String("job.schedule_type", scheduleType),
		),
	)
}

// JobExecutionSpan starts a span for job execution.
// Caller is responsible for calling span.End().
//
//nolint:spancheck // span is returned to caller who manages its lifecycle
func (t *Tracer) JobExecutionSpan(ctx context.Context, jobID, sourceID, url string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, "scheduler.execute_job",
		trace.WithAttributes(
			attribute.String("job.id", jobID),
			attribute.String("job.source_id", sourceID),
			attribute.String("job.url", url),
		),
	)
}

// QueueOperationSpan starts a span for queue operations.
// Caller is responsible for calling span.End().
//
//nolint:spancheck // span is returned to caller who manages its lifecycle
func (t *Tracer) QueueOperationSpan(ctx context.Context, operation, priority string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, fmt.Sprintf("queue.%s", operation),
		trace.WithAttributes(
			attribute.String("queue.operation", operation),
			attribute.String("queue.priority", priority),
		),
	)
}

// WorkerSpan starts a span for worker operations.
// Caller is responsible for calling span.End().
//
//nolint:spancheck // span is returned to caller who manages its lifecycle
func (t *Tracer) WorkerSpan(ctx context.Context, workerID string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, "worker.process",
		trace.WithAttributes(
			attribute.String("worker.id", workerID),
		),
	)
}

// TriggerSpan starts a span for trigger operations.
// Caller is responsible for calling span.End().
//
//nolint:spancheck // span is returned to caller who manages its lifecycle
func (t *Tracer) TriggerSpan(ctx context.Context, triggerType, pattern string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, fmt.Sprintf("trigger.%s", triggerType),
		trace.WithAttributes(
			attribute.String("trigger.type", triggerType),
			attribute.String("trigger.pattern", pattern),
		),
	)
}

// LeaderElectionSpan starts a span for leader election operations.
// Caller is responsible for calling span.End().
//
//nolint:spancheck // span is returned to caller who manages its lifecycle
func (t *Tracer) LeaderElectionSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, fmt.Sprintf("leader.%s", operation),
		trace.WithAttributes(
			attribute.String("leader.operation", operation),
		),
	)
}

// AddJobAttributes adds job-related attributes to a span.
func AddJobAttributes(span trace.Span, jobID, sourceID, status string, priority int) {
	span.SetAttributes(
		attribute.String("job.id", jobID),
		attribute.String("job.source_id", sourceID),
		attribute.String("job.status", status),
		attribute.Int("job.priority", priority),
	)
}

// AddQueueAttributes adds queue-related attributes to a span.
func AddQueueAttributes(span trace.Span, streamName string, messageCount int) {
	span.SetAttributes(
		attribute.String("queue.stream", streamName),
		attribute.Int("queue.message_count", messageCount),
	)
}

// AddWorkerAttributes adds worker-related attributes to a span.
func AddWorkerAttributes(span trace.Span, workerID string, busy bool) {
	span.SetAttributes(
		attribute.String("worker.id", workerID),
		attribute.Bool("worker.busy", busy),
	)
}

// AddTriggerAttributes adds trigger-related attributes to a span.
func AddTriggerAttributes(span trace.Span, triggerType, pattern string, matchedJobs int) {
	span.SetAttributes(
		attribute.String("trigger.type", triggerType),
		attribute.String("trigger.pattern", pattern),
		attribute.Int("trigger.matched_jobs", matchedJobs),
	)
}

// RecordError records an error on a span.
func RecordError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// SetSuccess marks a span as successful.
func SetSuccess(span trace.Span) {
	span.SetStatus(codes.Ok, "success")
}

// SpanFromContext extracts the current span from context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// ContextWithSpan creates a new context with the given span.
func ContextWithSpan(ctx context.Context, span trace.Span) context.Context {
	return trace.ContextWithSpan(ctx, span)
}
