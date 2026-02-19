package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/sse"
)

// Constants for v2 stream handler.
const (
	v2ReplayLineCount   = 200              // Number of lines to replay on connect
	v2BlockTimeout      = 5 * time.Second  // XREAD BLOCK timeout
	v2HeartbeatInterval = 15 * time.Second // Heartbeat interval for keep-alive
	v2InitialStreamID   = "0"              // Start from beginning of stream
	v2ReadBatchSize     = 100              // Max entries per XREAD call
	v2RetryDelay        = 100              // Retry delay in ms after error
	v2HeartbeatComment  = ":heartbeat\n\n" // SSE comment for keep-alive
)

// LogsStreamV2Handler handles v2 SSE streaming directly from Redis Streams.
// This handler bypasses the SSE broker and reads directly from Redis,
// providing more reliable log streaming during job execution.
type LogsStreamV2Handler struct {
	redisWriter *logs.RedisStreamWriter
	logger      infralogger.Logger
}

// NewLogsStreamV2Handler creates a new v2 logs stream handler.
func NewLogsStreamV2Handler(
	redisWriter *logs.RedisStreamWriter,
	logger infralogger.Logger,
) *LogsStreamV2Handler {
	return &LogsStreamV2Handler{
		redisWriter: redisWriter,
		logger:      logger,
	}
}

// Stream handles GET /api/v1/jobs/:id/logs/stream/v2
// Streams log events via SSE directly from Redis Streams.
func (h *LogsStreamV2Handler) Stream(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job ID required"})
		return
	}

	if h.redisWriter == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Redis streaming not available"})
		return
	}

	// Set SSE headers
	sse.SetSSEHeaders(c.Writer)
	c.Writer.Flush()

	// Send connected event
	if connErr := h.sendConnectedEvent(c, jobID); connErr != nil {
		return
	}

	// Check for Last-Event-ID header for resume
	lastID := c.GetHeader("Last-Event-ID")
	if lastID == "" {
		lastID = v2InitialStreamID
	}

	// Replay buffered logs if starting from beginning
	if lastID == v2InitialStreamID {
		lastID = h.replayLogs(c, jobID)
	}

	h.logger.Debug("SSE v2 log stream started",
		infralogger.String("job_id", jobID),
		infralogger.String("client_ip", c.ClientIP()),
		infralogger.String("last_id", lastID),
	)

	// Stream new entries using XREAD BLOCK
	h.streamLogs(c, jobID, lastID)

	h.logger.Debug("SSE v2 log stream ended",
		infralogger.String("job_id", jobID),
	)
}

// sendConnectedEvent sends the initial connected event to the client.
func (h *LogsStreamV2Handler) sendConnectedEvent(c *gin.Context, jobID string) error {
	connEvent := sse.Event{
		Type: "connected",
		Data: map[string]any{
			"message": "Log stream v2 connected",
			"job_id":  jobID,
		},
	}
	if writeErr := sse.WriteEventDirect(c.Writer, connEvent); writeErr != nil {
		h.logger.Debug("Failed to write connected event", infralogger.Error(writeErr))
		return writeErr
	}
	return nil
}

// replayLogs sends the last N log entries to the client.
// Returns the ID of the last replayed entry (or "0" if none).
func (h *LogsStreamV2Handler) replayLogs(c *gin.Context, jobID string) string {
	entries, readErr := h.redisWriter.ReadLast(c.Request.Context(), jobID, v2ReplayLineCount)
	if readErr != nil {
		h.logger.Debug("Failed to read logs for replay",
			infralogger.Error(readErr),
			infralogger.String("job_id", jobID),
		)
		return v2InitialStreamID
	}

	if len(entries) == 0 {
		return v2InitialStreamID
	}

	// Convert entries to LogLineData for the replay event
	lines := h.entriesToLogLineData(entries)

	// Send replay event
	event := sse.NewLogReplayEvent(lines)
	if writeErr := sse.WriteEventDirect(c.Writer, event); writeErr != nil {
		h.logger.Debug("Failed to write replay event", infralogger.Error(writeErr))
		return v2InitialStreamID
	}

	h.logger.Debug("Replayed logs",
		infralogger.String("job_id", jobID),
		infralogger.Int("count", len(entries)),
	)

	// Return "$" to only get new entries after current stream position
	return "$"
}

// entriesToLogLineData converts log entries to SSE LogLineData format.
func (h *LogsStreamV2Handler) entriesToLogLineData(entries []logs.LogEntry) []sse.LogLineData {
	lines := make([]sse.LogLineData, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, sse.LogLineData{
			JobID:       entry.JobID,
			ExecutionID: entry.ExecID,
			Timestamp:   entry.Timestamp.Format(time.RFC3339Nano),
			Level:       entry.Level,
			Category:    entry.Category,
			Message:     entry.Message,
			Fields:      entry.Fields,
		})
	}
	return lines
}

// streamLogs continuously streams new log entries using XREAD BLOCK.
func (h *LogsStreamV2Handler) streamLogs(c *gin.Context, jobID, lastID string) {
	streamKey := h.redisWriter.StreamKey(jobID)
	client := h.redisWriter.Client()
	ctx := c.Request.Context()

	heartbeatTicker := time.NewTicker(v2HeartbeatInterval)
	defer heartbeatTicker.Stop()

	currentLastID := lastID

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeatTicker.C:
			if !h.sendHeartbeat(c.Writer) {
				return
			}
		default:
			newLastID, shouldReturn := h.processStreamEntries(ctx, c, client, streamKey, currentLastID)
			if shouldReturn {
				return
			}
			currentLastID = newLastID
		}
	}
}

// sendHeartbeat sends a heartbeat comment to keep the connection alive.
func (h *LogsStreamV2Handler) sendHeartbeat(w gin.ResponseWriter) bool {
	if _, err := w.WriteString(v2HeartbeatComment); err != nil {
		return false
	}
	w.Flush()
	return true
}

// processStreamEntries reads and sends new entries from the Redis stream.
// Returns the new lastID and whether the streaming loop should return.
func (h *LogsStreamV2Handler) processStreamEntries(
	ctx context.Context,
	c *gin.Context,
	client *redis.Client,
	streamKey, lastID string,
) (string, bool) {
	entries, newLastID, readErr := h.readNewEntries(ctx, client, streamKey, lastID)
	if readErr != nil {
		if errors.Is(readErr, context.Canceled) || errors.Is(readErr, context.DeadlineExceeded) {
			return lastID, true
		}
		h.logger.Debug("XREAD error", infralogger.Error(readErr))
		time.Sleep(v2RetryDelay * time.Millisecond)
		return lastID, false
	}

	if len(entries) == 0 {
		return lastID, false
	}

	// Send each entry to the client
	for _, entry := range entries {
		if writeErr := h.sendLogEntry(c.Writer, entry); writeErr != nil {
			h.logger.Debug("SSE write failed", infralogger.Error(writeErr))
			return newLastID, true
		}
	}

	return newLastID, false
}

// sendLogEntry sends a single log entry as an SSE event.
func (h *LogsStreamV2Handler) sendLogEntry(w gin.ResponseWriter, entry logs.LogEntry) error {
	event := sse.Event{
		Type: sse.EventTypeLogLine,
		Data: sse.LogLineData{
			JobID:       entry.JobID,
			ExecutionID: entry.ExecID,
			Timestamp:   entry.Timestamp.Format(time.RFC3339Nano),
			Level:       entry.Level,
			Category:    entry.Category,
			Message:     entry.Message,
			Fields:      entry.Fields,
		},
	}
	return sse.WriteEventDirect(w, event)
}

// readNewEntries reads new entries from Redis using XREAD BLOCK.
// Returns the entries, the new last ID, and any error.
func (h *LogsStreamV2Handler) readNewEntries(
	ctx context.Context,
	client *redis.Client,
	streamKey, lastID string,
) ([]logs.LogEntry, string, error) {
	streams, err := client.XRead(ctx, &redis.XReadArgs{
		Streams: []string{streamKey, lastID},
		Count:   v2ReadBatchSize,
		Block:   v2BlockTimeout,
	}).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, lastID, nil
		}
		return nil, lastID, err
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, lastID, nil
	}

	messages := streams[0].Messages
	entries := make([]logs.LogEntry, 0, len(messages))
	newLastID := lastID

	for _, msg := range messages {
		entry := h.parseMessage(msg)
		entries = append(entries, entry)
		newLastID = msg.ID
	}

	return entries, newLastID, nil
}

// parseMessage converts a Redis stream message to a LogEntry.
func (h *LogsStreamV2Handler) parseMessage(msg redis.XMessage) logs.LogEntry {
	entry := logs.LogEntry{}

	if ts, ok := msg.Values["timestamp"].(string); ok {
		t, parseErr := time.Parse(time.RFC3339Nano, ts)
		if parseErr == nil {
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

	return entry
}
