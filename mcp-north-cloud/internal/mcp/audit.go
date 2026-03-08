package mcp

import (
	"encoding/json"
	"time"

	"github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// AuditEntry captures metrics for a single tool invocation.
type AuditEntry struct {
	ToolName    string
	RequestID   any
	DurationMs  int64
	ResultBytes int
	Success     bool
	ErrorCode   int
	Timestamp   time.Time
	ParamKeys   []string
}

// logToolAudit emits a structured INFO log line to stderr for audit purposes.
func logToolAudit(log logger.Logger, entry AuditEntry) {
	if log == nil {
		return
	}

	fields := []logger.Field{
		logger.String("tool", entry.ToolName),
		logger.Any("request_id", entry.RequestID),
		logger.Int64("duration_ms", entry.DurationMs),
		logger.Int("result_bytes", entry.ResultBytes),
		logger.Bool("success", entry.Success),
		logger.String("timestamp", entry.Timestamp.Format(time.RFC3339)),
	}

	if len(entry.ParamKeys) > 0 {
		fields = append(fields, logger.Any("param_keys", entry.ParamKeys))
	}

	if !entry.Success {
		fields = append(fields, logger.Int("error_code", entry.ErrorCode))
	}

	log.Info("tool_call", fields...)
}

// extractParamKeys unmarshals just the top-level keys from a JSON object.
// Returns nil for invalid JSON or non-object values.
func extractParamKeys(arguments json.RawMessage) []string {
	if len(arguments) == 0 {
		return nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(arguments, &obj); err != nil {
		return nil
	}

	if len(obj) == 0 {
		return nil
	}

	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}

	return keys
}
