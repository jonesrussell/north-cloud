package logs

import (
	"context"
	"io"
	"time"
)

// Service manages log streaming and archival for job executions.
type Service interface {
	// StartCapture initializes log capture for a job execution.
	// Returns a Writer that should be used for logging during the job.
	StartCapture(ctx context.Context, jobID, executionID string, executionNumber int) (Writer, error)

	// StopCapture finalizes log capture (archives to MinIO, updates DB).
	StopCapture(ctx context.Context, jobID, executionID string) (*LogMetadata, error)

	// GetLogReader retrieves archived logs from MinIO as a Reader.
	GetLogReader(ctx context.Context, objectKey string) (io.ReadCloser, error)

	// IsCapturing returns true if logs are being captured for the given execution.
	IsCapturing(executionID string) bool

	// Close gracefully shuts down the service.
	Close() error
}

// Writer captures log output for a job execution.
type Writer interface {
	io.Writer

	// WriteEntry writes a structured log entry.
	WriteEntry(entry LogEntry)

	// GetBuffer returns the current log buffer for SSE streaming.
	GetBuffer() Buffer

	// Close flushes and closes the writer.
	Close() error
}

// Buffer manages in-memory log buffering for live streaming.
type Buffer interface {
	// Write appends a log entry to the buffer.
	Write(entry LogEntry)

	// ReadSince returns all entries since the given timestamp.
	ReadSince(since time.Time) []LogEntry

	// ReadAll returns all buffered entries.
	ReadAll() []LogEntry

	// Size returns the number of entries in the buffer.
	Size() int

	// Clear empties the buffer.
	Clear()

	// Bytes returns the buffer content as a byte slice (for archiving).
	Bytes() []byte

	// LineCount returns the total number of lines written.
	LineCount() int
}

// Archiver handles MinIO upload of completed job logs.
type Archiver interface {
	// Archive uploads logs to MinIO synchronously.
	Archive(ctx context.Context, task *ArchiveTask) (string, error)

	// GetObject retrieves an archived log file from MinIO.
	GetObject(ctx context.Context, objectKey string) (io.ReadCloser, error)

	// Close gracefully shuts down the archiver.
	Close() error
}

// Publisher publishes log events to SSE subscribers.
type Publisher interface {
	// PublishLogLine publishes a single log line event.
	PublishLogLine(ctx context.Context, entry LogEntry)

	// PublishLogArchived publishes a log archived event.
	PublishLogArchived(ctx context.Context, metadata *LogMetadata)
}
