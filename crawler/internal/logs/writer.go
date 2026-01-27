package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Log level constants for ordering.
const (
	levelDebug = 0
	levelInfo  = 1
	levelWarn  = 2
	levelError = 3
)

// logWriter implements Writer to capture logs for a job execution.
type logWriter struct {
	jobID       string
	executionID string
	buffer      Buffer
	publisher   Publisher
	ctx         context.Context
	minLevel    string

	mu     sync.Mutex
	closed atomic.Bool
}

// NewWriter creates a new log writer for a job execution.
func NewWriter(
	ctx context.Context,
	jobID, executionID string,
	buffer Buffer,
	publisher Publisher,
	minLevel string,
) Writer {
	return &logWriter{
		jobID:       jobID,
		executionID: executionID,
		buffer:      buffer,
		publisher:   publisher,
		ctx:         ctx,
		minLevel:    minLevel,
	}
}

// Write implements io.Writer. It parses JSON log lines and writes to buffer.
func (w *logWriter) Write(p []byte) (n int, err error) {
	if w.closed.Load() {
		return 0, io.ErrClosedPipe
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Parse JSON log line (Zap format)
	lines := bytes.Split(p, []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		entry := w.parseLogLine(line)
		if entry == nil {
			continue
		}

		// Filter by log level
		if !w.shouldCapture(entry.Level) {
			continue
		}

		// Write to buffer
		w.buffer.Write(*entry)

		// Publish to SSE (if enabled and publisher available)
		if w.publisher != nil {
			w.publisher.PublishLogLine(w.ctx, *entry)
		}
	}

	return len(p), nil
}

// WriteEntry writes a structured log entry directly.
func (w *logWriter) WriteEntry(entry LogEntry) {
	if w.closed.Load() {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Ensure job context is set
	entry.JobID = w.jobID
	entry.ExecID = w.executionID

	// Filter by log level
	if !w.shouldCapture(entry.Level) {
		return
	}

	// Write to buffer
	w.buffer.Write(entry)

	// Publish to SSE
	if w.publisher != nil {
		w.publisher.PublishLogLine(w.ctx, entry)
	}
}

// GetBuffer returns the underlying buffer.
func (w *logWriter) GetBuffer() Buffer {
	return w.buffer
}

// Close marks the writer as closed.
func (w *logWriter) Close() error {
	w.closed.Store(true)
	return nil
}

// parseLogLine parses a JSON log line into a LogEntry.
func (w *logWriter) parseLogLine(line []byte) *LogEntry {
	var raw map[string]any
	if err := json.Unmarshal(line, &raw); err != nil {
		// Not valid JSON, create a simple entry
		return &LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   string(line),
			JobID:     w.jobID,
			ExecID:    w.executionID,
		}
	}

	entry := &LogEntry{
		Timestamp: time.Now(),
		JobID:     w.jobID,
		ExecID:    w.executionID,
		Fields:    make(map[string]any),
	}

	// Extract timestamp from either "ts" or "time" field
	entry.Timestamp = extractTimestamp(raw)

	// Extract log level
	if level, ok := raw["level"].(string); ok {
		entry.Level = strings.ToLower(level)
	} else {
		entry.Level = "info"
	}

	// Extract message from either "msg" or "message" field
	entry.Message = extractMessage(raw)

	// Copy remaining fields
	excludeKeys := map[string]bool{
		"ts": true, "time": true, "level": true, "msg": true, "message": true,
	}
	for k, v := range raw {
		if !excludeKeys[k] {
			entry.Fields[k] = v
		}
	}

	return entry
}

// extractTimestamp extracts timestamp from "ts" or "time" field.
func extractTimestamp(raw map[string]any) time.Time {
	if ts, ok := raw["ts"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
			return parsed
		}
	}
	if ts, ok := raw["time"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
			return parsed
		}
	}
	return time.Now()
}

// extractMessage extracts message from "msg" or "message" field.
func extractMessage(raw map[string]any) string {
	if msg, ok := raw["msg"].(string); ok {
		return msg
	}
	if msg, ok := raw["message"].(string); ok {
		return msg
	}
	return ""
}

// shouldCapture checks if the log level meets the minimum threshold.
func (w *logWriter) shouldCapture(level string) bool {
	levelOrder := map[string]int{
		"debug": levelDebug,
		"info":  levelInfo,
		"warn":  levelWarn,
		"error": levelError,
	}

	minLevelNum, ok := levelOrder[w.minLevel]
	if !ok {
		minLevelNum = levelInfo // default to info
	}

	entryLevelNum, ok := levelOrder[level]
	if !ok {
		entryLevelNum = levelInfo // default to info
	}

	return entryLevelNum >= minLevelNum
}
