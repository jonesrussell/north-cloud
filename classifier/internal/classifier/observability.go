package classifier

import (
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const titleExcerptWordLimit = 10

// truncateWords returns the first n words of s, appending "..." if truncated.
func truncateWords(s string, n int) string {
	words := strings.Fields(s)
	if len(words) <= n {
		return s
	}
	return strings.Join(words[:n], " ") + "..."
}

// classifyErrorType categorizes an ML sidecar error message for dashboard filtering.
func classifyErrorType(errMsg string) string {
	lower := strings.ToLower(errMsg)
	switch {
	case strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "timeout"):
		return "timeout"
	case strings.Contains(lower, "returned 5"):
		return "5xx"
	case strings.Contains(lower, "returned 4"):
		return "4xx"
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "dial tcp") ||
		strings.Contains(lower, "no such host"):
		return "connection"
	case strings.Contains(lower, "decode") || strings.Contains(lower, "unmarshal") ||
		strings.Contains(lower, "eof"):
		return "decode"
	default:
		return "unknown"
	}
}

// logSidecarSuccess emits a structured Info log for a successful ML sidecar classification.
func (c *Classifier) logSidecarSuccess(
	sidecar string, raw *domain.RawContent, contentType string,
	relevance string, confidence, mlConfRaw float64,
	ruleTriggered, decisionPath string,
	latencyMs, processingTimeMs int64, modelVersion string,
) {
	c.logger.Info("ML sidecar classification complete",
		infralogger.String("sidecar", sidecar),
		infralogger.String("content_id", raw.ID),
		infralogger.String("content_type", contentType),
		infralogger.String("source", raw.SourceName),
		infralogger.String("title_excerpt", truncateWords(raw.Title, titleExcerptWordLimit)),
		infralogger.String("relevance", relevance),
		infralogger.Float64("confidence", confidence),
		infralogger.Float64("ml_confidence_raw", mlConfRaw),
		infralogger.String("rule_triggered", ruleTriggered),
		infralogger.String("decision_path", decisionPath),
		infralogger.Int64("latency_ms", latencyMs),
		infralogger.Int64("processing_time_ms", processingTimeMs),
		infralogger.String("model_version", modelVersion),
		infralogger.String("outcome", "success"),
	)
}

// logSidecarError emits a structured Warn log for a failed ML sidecar classification.
func (c *Classifier) logSidecarError(
	sidecar string, raw *domain.RawContent, contentType string,
	err error, latencyMs int64,
) {
	c.logger.Warn("ML sidecar classification failed",
		infralogger.String("sidecar", sidecar),
		infralogger.String("content_id", raw.ID),
		infralogger.String("content_type", contentType),
		infralogger.String("source", raw.SourceName),
		infralogger.String("title_excerpt", truncateWords(raw.Title, titleExcerptWordLimit)),
		infralogger.String("outcome", "error"),
		infralogger.String("error_type", classifyErrorType(err.Error())),
		infralogger.String("error_detail", err.Error()),
		infralogger.Int64("latency_ms", latencyMs),
	)
}

// logSidecarNilResult emits a structured Error log when an ML sidecar returns a nil result with no error.
// A nil result without an error is a contract violation in the sidecar interface and indicates a client bug.
func (c *Classifier) logSidecarNilResult(sidecar, contentID string, latencyMs int64) {
	c.logger.Error("ML sidecar returned nil result without error",
		infralogger.String("sidecar", sidecar),
		infralogger.String("content_id", contentID),
		infralogger.Int64("latency_ms", latencyMs),
		infralogger.String("outcome", "nil_result"),
	)
}
