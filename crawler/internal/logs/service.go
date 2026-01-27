package logs

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// activeWriter tracks an active log writer for a job execution.
type activeWriter struct {
	writer          Writer
	jobID           string
	executionID     string
	executionNumber int
	startedAt       time.Time
}

// logService implements Service for job log management.
type logService struct {
	config        *Config
	archiver      Archiver
	publisher     Publisher
	executionRepo database.ExecutionRepositoryInterface
	logger        infralogger.Logger

	activeWriters map[string]*activeWriter // keyed by executionID
	mu            sync.RWMutex
}

// NewService creates a new log service.
func NewService(
	cfg *Config,
	archiver Archiver,
	publisher Publisher,
	executionRepo database.ExecutionRepositoryInterface,
	logger infralogger.Logger,
) Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	return &logService{
		config:        cfg,
		archiver:      archiver,
		publisher:     publisher,
		executionRepo: executionRepo,
		logger:        logger,
		activeWriters: make(map[string]*activeWriter),
	}
}

// StartCapture initializes log capture for a job execution.
func (s *logService) StartCapture(
	ctx context.Context,
	jobID, executionID string,
	executionNumber int,
) (Writer, error) {
	if !s.config.Enabled {
		return &noopWriter{}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already capturing for this execution
	if _, exists := s.activeWriters[executionID]; exists {
		return nil, fmt.Errorf("log capture already active for execution: %s", executionID)
	}

	// Create buffer and writer
	buffer := NewBuffer(s.config.BufferSize)
	writer := NewWriter(ctx, jobID, executionID, buffer, s.publisher, s.config.MinLevel)

	// Track active writer
	s.activeWriters[executionID] = &activeWriter{
		writer:          writer,
		jobID:           jobID,
		executionID:     executionID,
		executionNumber: executionNumber,
		startedAt:       time.Now(),
	}

	s.logger.Debug("Started log capture",
		infralogger.String("job_id", jobID),
		infralogger.String("execution_id", executionID),
		infralogger.Int("execution_number", executionNumber),
	)

	return writer, nil
}

// StopCapture finalizes log capture (archives to MinIO, updates DB).
func (s *logService) StopCapture(ctx context.Context, _, executionID string) (*LogMetadata, error) {
	if !s.config.Enabled {
		return &LogMetadata{}, nil
	}

	s.mu.Lock()
	aw, exists := s.activeWriters[executionID]
	if exists {
		delete(s.activeWriters, executionID)
	}
	s.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("no active log capture for execution: %s", executionID)
	}

	// Close the writer
	if err := aw.writer.Close(); err != nil {
		s.logger.Warn("Failed to close log writer",
			infralogger.Error(err),
			infralogger.String("execution_id", executionID),
		)
	}

	// Get buffer content
	buffer := aw.writer.GetBuffer()
	content := buffer.Bytes()
	lineCount := buffer.LineCount()

	// Archive if enabled and there are logs
	var metadata *LogMetadata
	if s.config.ArchiveEnabled && len(content) > 0 {
		task := &ArchiveTask{
			JobID:           aw.jobID,
			ExecutionID:     aw.executionID,
			ExecutionNumber: aw.executionNumber,
			Content:         content,
			LineCount:       lineCount,
			StartedAt:       aw.startedAt,
		}

		objectKey, err := s.archiver.Archive(ctx, task)
		if err != nil {
			s.logger.Error("Failed to archive logs",
				infralogger.Error(err),
				infralogger.String("job_id", aw.jobID),
				infralogger.String("execution_id", aw.executionID),
			)
			// Continue without archiving - don't fail the job
		} else {
			metadata = &LogMetadata{
				JobID:           aw.jobID,
				ExecutionID:     aw.executionID,
				ExecutionNumber: aw.executionNumber,
				ObjectKey:       objectKey,
				SizeBytes:       int64(len(content)),
				LineCount:       lineCount,
				StartedAt:       aw.startedAt,
			}

			// Update execution record with log metadata
			if updateErr := s.updateExecutionLogMetadata(ctx, aw.executionID, metadata); updateErr != nil {
				s.logger.Error("Failed to update execution with log metadata",
					infralogger.Error(updateErr),
					infralogger.String("execution_id", aw.executionID),
				)
			}

			// Publish SSE event
			s.publisher.PublishLogArchived(ctx, metadata)

			s.logger.Info("Archived job logs",
				infralogger.String("job_id", aw.jobID),
				infralogger.String("execution_id", aw.executionID),
				infralogger.String("object_key", objectKey),
				infralogger.Int("line_count", lineCount),
			)
		}
	}

	return metadata, nil
}

// GetLogReader retrieves archived logs from MinIO.
func (s *logService) GetLogReader(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	return s.archiver.GetObject(ctx, objectKey)
}

// IsCapturing returns true if logs are being captured for the execution.
func (s *logService) IsCapturing(executionID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.activeWriters[executionID]
	return exists
}

// Close gracefully shuts down the service.
func (s *logService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close all active writers
	for id, aw := range s.activeWriters {
		if err := aw.writer.Close(); err != nil {
			s.logger.Warn("Failed to close log writer during shutdown",
				infralogger.Error(err),
				infralogger.String("execution_id", id),
			)
		}
	}
	s.activeWriters = make(map[string]*activeWriter)

	// Close archiver
	if err := s.archiver.Close(); err != nil {
		s.logger.Warn("Failed to close log archiver", infralogger.Error(err))
	}

	return nil
}

// updateExecutionLogMetadata updates the execution record with log metadata.
func (s *logService) updateExecutionLogMetadata(ctx context.Context, executionID string, metadata *LogMetadata) error {
	execution, err := s.executionRepo.GetByID(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to get execution: %w", err)
	}

	execution.LogObjectKey = &metadata.ObjectKey
	execution.LogSizeBytes = &metadata.SizeBytes
	execution.LogLineCount = &metadata.LineCount

	if updateErr := s.executionRepo.Update(ctx, execution); updateErr != nil {
		return fmt.Errorf("failed to update execution: %w", updateErr)
	}

	return nil
}

// noopWriter is a no-op writer when logging is disabled.
type noopWriter struct{}

func (w *noopWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w *noopWriter) WriteEntry(_ LogEntry)       {}
func (w *noopWriter) GetBuffer() Buffer           { return NewBuffer(0) }
func (w *noopWriter) Close() error                { return nil }

// Ensure noopWriter implements Writer
var _ Writer = (*noopWriter)(nil)

// Ensure logService implements Service
var _ Service = (*logService)(nil)

// Ensure activeWriter fields are used
var _ = domain.JobExecution{}
