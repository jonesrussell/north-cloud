package api

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/sse"
)

// Constants for log handler operations.
const (
	maxExecutionsForSearch    = 100      // Maximum executions to search for log download
	latestExecutionIdentifier = "latest" // Identifier for the latest execution
	sseReplayLineCount        = 200      // Number of buffered lines to replay on SSE connect
)

// Errors for log handler operations.
var (
	errNoLogsArchived = errors.New("no logs archived")
	errNoLogsFound    = errors.New("no logs found")
)

// LogsHandler handles log-related HTTP endpoints.
type LogsHandler struct {
	logService    logs.Service
	executionRepo database.ExecutionRepositoryInterface
	sseBroker     sse.Broker
	logger        infralogger.Logger
}

// NewLogsHandler creates a new LogsHandler.
func NewLogsHandler(
	logService logs.Service,
	executionRepo database.ExecutionRepositoryInterface,
	sseBroker sse.Broker,
	logger infralogger.Logger,
) *LogsHandler {
	return &LogsHandler{
		logService:    logService,
		executionRepo: executionRepo,
		sseBroker:     sseBroker,
		logger:        logger,
	}
}

// StreamLogs handles GET /api/v1/jobs/:id/logs/stream
// Streams log events via SSE for a specific job.
func (h *LogsHandler) StreamLogs(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job ID required"})
		return
	}

	// Set SSE headers
	sse.SetSSEHeaders(c.Writer)
	c.Writer.Flush()

	// Subscribe to SSE broker with filter for this job's logs
	filter := func(event sse.Event) bool {
		switch event.Type {
		case sse.EventTypeLogLine:
			if data, ok := event.Data.(sse.LogLineData); ok {
				return data.JobID == jobID
			}
		case sse.EventTypeLogArchived:
			if data, ok := event.Data.(sse.LogArchivedData); ok {
				return data.JobID == jobID
			}
		}
		return false
	}

	eventChan, cleanup := h.sseBroker.Subscribe(c.Request.Context(), sse.WithFilter(filter))
	defer cleanup()

	// Send connected event
	connEvent := sse.Event{
		Type: "connected",
		Data: map[string]any{
			"message": "Log stream connected",
			"job_id":  jobID,
		},
	}
	if err := sse.WriteEventDirect(c.Writer, connEvent); err != nil {
		h.logger.Debug("Failed to write connected event", infralogger.Error(err))
		return
	}

	// Replay buffered logs if job is actively capturing
	replayCount := h.replayBufferedLogs(c, jobID)

	h.logger.Debug("SSE log stream started",
		infralogger.String("job_id", jobID),
		infralogger.String("client_ip", c.ClientIP()),
		infralogger.Int("replayed_lines", replayCount),
	)

	// Stream events until client disconnects
	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				return
			}
			if err := sse.WriteEventDirect(c.Writer, event); err != nil {
				h.logger.Debug("SSE write failed", infralogger.Error(err))
				return
			}
		case <-c.Request.Context().Done():
			h.logger.Debug("SSE log stream ended",
				infralogger.String("job_id", jobID),
			)
			return
		}
	}
}

// replayBufferedLogs sends buffered log entries to the SSE client.
// Returns the number of entries replayed.
func (h *LogsHandler) replayBufferedLogs(c *gin.Context, jobID string) int {
	buffer := h.logService.GetLiveBuffer(jobID)
	if buffer == nil {
		return 0
	}

	entries := buffer.ReadLast(sseReplayLineCount)
	if len(entries) == 0 {
		return 0
	}

	for _, entry := range entries {
		event := sse.Event{
			Type: sse.EventTypeLogLine,
			Data: sse.LogLineData{
				JobID:       entry.JobID,
				ExecutionID: entry.ExecID,
				Timestamp:   entry.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"),
				Level:       entry.Level,
				Message:     entry.Message,
				Fields:      entry.Fields,
			},
		}
		if err := sse.WriteEventDirect(c.Writer, event); err != nil {
			h.logger.Debug("Failed to write replay event", infralogger.Error(err))
			return len(entries)
		}
	}

	return len(entries)
}

// GetLogsMetadata handles GET /api/v1/jobs/:id/logs
// Returns metadata about available logs for a job's executions.
func (h *LogsHandler) GetLogsMetadata(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job ID required"})
		return
	}

	// Parse pagination
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	// Get executions for the job
	executions, err := h.executionRepo.ListByJobID(c.Request.Context(), jobID, limit, offset)
	if err != nil {
		h.logger.Error("Failed to list executions",
			infralogger.Error(err),
			infralogger.String("job_id", jobID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list executions"})
		return
	}

	// Build response with log availability info
	logsInfo := make([]gin.H, 0, len(executions))
	for _, exec := range executions {
		info := gin.H{
			"execution_id":     exec.ID,
			"execution_number": exec.ExecutionNumber,
			"status":           exec.Status,
			"started_at":       exec.StartedAt,
			"completed_at":     exec.CompletedAt,
			"log_available":    exec.LogObjectKey != nil,
		}
		if exec.LogObjectKey != nil {
			info["log_object_key"] = *exec.LogObjectKey
			if exec.LogSizeBytes != nil {
				info["log_size_bytes"] = *exec.LogSizeBytes
			}
			if exec.LogLineCount != nil {
				info["log_line_count"] = *exec.LogLineCount
			}
		}
		logsInfo = append(logsInfo, info)
	}

	// Check if job is currently running (live logs available)
	hasLiveLogs := false
	if len(executions) > 0 && executions[0].Status == "running" {
		hasLiveLogs = h.logService.IsCapturing(executions[0].ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"job_id":        jobID,
		"executions":    logsInfo,
		"has_live_logs": hasLiveLogs,
		"limit":         limit,
		"offset":        offset,
	})
}

// DownloadLogs handles GET /api/v1/jobs/:id/logs/download
// Downloads archived logs for a specific execution.
func (h *LogsHandler) DownloadLogs(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job ID required"})
		return
	}

	// Get execution number from query param
	executionNumStr := c.Query("execution")
	if executionNumStr == "" {
		// Default to latest execution
		executionNumStr = latestExecutionIdentifier
	}

	// Find the log object key based on execution parameter
	objectKey, executionNum, findErr := h.findLogObjectKey(c, jobID, executionNumStr)
	if findErr != nil {
		return // Error response already sent
	}

	// Get log reader from service
	reader, readerErr := h.logService.GetLogReader(c.Request.Context(), objectKey)
	if readerErr != nil {
		h.logger.Error("Failed to get log reader",
			infralogger.Error(readerErr),
			infralogger.String("object_key", objectKey),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve logs"})
		return
	}
	defer reader.Close()

	// Set headers for gzip download
	filename := fmt.Sprintf("job-%s-exec-%d.log.gz", jobID, executionNum)
	c.Header("Content-Type", "application/gzip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// Stream response
	if _, copyErr := io.Copy(c.Writer, reader); copyErr != nil {
		h.logger.Error("Failed to stream logs",
			infralogger.Error(copyErr),
			infralogger.String("object_key", objectKey),
		)
		// Don't write JSON error - headers already sent
		return
	}

	h.logger.Debug("Downloaded logs",
		infralogger.String("job_id", jobID),
		infralogger.Int("execution_number", executionNum),
		infralogger.String("object_key", objectKey),
	)
}

// findLogObjectKey finds the object key for log download.
// Returns the object key, execution number, and any error.
// If error is non-nil, an HTTP response has already been sent.
func (h *LogsHandler) findLogObjectKey(
	c *gin.Context, jobID, executionNumStr string,
) (objectKey string, execNum int, err error) {
	if executionNumStr == latestExecutionIdentifier {
		return h.findLatestLogObjectKey(c, jobID)
	}
	return h.findSpecificLogObjectKey(c, jobID, executionNumStr)
}

// findLatestLogObjectKey finds the log object key for the latest execution.
func (h *LogsHandler) findLatestLogObjectKey(
	c *gin.Context, jobID string,
) (objectKey string, execNum int, err error) {
	execution, err := h.executionRepo.GetLatestByJobID(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no executions found for job"})
		return "", 0, err
	}
	if execution.LogObjectKey == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no logs archived for latest execution"})
		return "", 0, errNoLogsArchived
	}
	return *execution.LogObjectKey, execution.ExecutionNumber, nil
}

// findSpecificLogObjectKey finds the log object key for a specific execution number.
func (h *LogsHandler) findSpecificLogObjectKey(
	c *gin.Context, jobID, executionNumStr string,
) (objectKey string, execNum int, err error) {
	executionNum, _ := strconv.Atoi(executionNumStr)
	executions, err := h.executionRepo.ListByJobID(c.Request.Context(), jobID, maxExecutionsForSearch, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to find execution"})
		return "", 0, err
	}

	for _, exec := range executions {
		if exec.ExecutionNumber == executionNum && exec.LogObjectKey != nil {
			return *exec.LogObjectKey, executionNum, nil
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "no logs found for specified execution"})
	return "", 0, errNoLogsFound
}

// ViewLogs handles GET /api/v1/jobs/:id/logs/view
// Returns decompressed log content as JSON for viewing in the UI.
func (h *LogsHandler) ViewLogs(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job ID required"})
		return
	}

	// Get execution number from query param
	executionNumStr := c.Query("execution")
	if executionNumStr == "" {
		executionNumStr = latestExecutionIdentifier
	}

	// Find the log object key
	objectKey, executionNum, findErr := h.findLogObjectKey(c, jobID, executionNumStr)
	if findErr != nil {
		return // Error response already sent
	}

	// Get log reader from service (returns gzipped content)
	reader, readerErr := h.logService.GetLogReader(c.Request.Context(), objectKey)
	if readerErr != nil {
		h.logger.Error("Failed to get log reader",
			infralogger.Error(readerErr),
			infralogger.String("object_key", objectKey),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve logs"})
		return
	}
	defer reader.Close()

	// Decompress gzip content
	gzReader, gzErr := newGzipReader(reader)
	if gzErr != nil {
		h.logger.Error("Failed to create gzip reader",
			infralogger.Error(gzErr),
			infralogger.String("object_key", objectKey),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decompress logs"})
		return
	}
	defer gzReader.Close()

	// Read decompressed content
	content, readErr := io.ReadAll(gzReader)
	if readErr != nil {
		h.logger.Error("Failed to read log content",
			infralogger.Error(readErr),
			infralogger.String("object_key", objectKey),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read logs"})
		return
	}

	// Parse JSON lines into log entries
	lines := parseLogLines(content)

	h.logger.Debug("Viewed logs",
		infralogger.String("job_id", jobID),
		infralogger.Int("execution_number", executionNum),
		infralogger.Int("line_count", len(lines)),
	)

	c.JSON(http.StatusOK, gin.H{
		"job_id":           jobID,
		"execution_number": executionNum,
		"lines":            lines,
		"line_count":       len(lines),
	})
}

// newGzipReader creates a gzip reader from the given reader.
func newGzipReader(r io.Reader) (*gzip.Reader, error) {
	return gzip.NewReader(r)
}

// parseLogLines parses JSON lines content into a slice of log entries.
func parseLogLines(content []byte) []map[string]any {
	var lines []map[string]any
	scanner := bufio.NewScanner(bufio.NewReader(
		&bytesReader{data: content, pos: 0},
	))

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			// If not valid JSON, create a simple message entry
			entry = map[string]any{
				"message": string(line),
				"level":   "info",
			}
		}
		lines = append(lines, entry)
	}

	return lines
}

// bytesReader implements io.Reader for a byte slice.
type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
