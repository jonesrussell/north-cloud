// Package logs provides job execution log streaming and archival functionality.
package logs

import "time"

// Configuration constants.
const (
	defaultBufferSize    = 1000
	defaultRetentionDays = 30
	defaultMinLevel      = "info"
	defaultMinioBucket   = "crawler-logs"
)

// LogEntry represents a single log line captured during job execution.
type LogEntry struct {
	Timestamp time.Time      `json:"timestamp"`
	Level     string         `json:"level"` // debug, info, warn, error
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
	JobID     string         `json:"job_id"`
	ExecID    string         `json:"execution_id"`
}

// LogMetadata represents metadata about archived logs stored in the database.
type LogMetadata struct {
	JobID           string    `json:"job_id"`
	ExecutionID     string    `json:"execution_id"`
	ExecutionNumber int       `json:"execution_number"`
	ObjectKey       string    `json:"log_object_key"`
	SizeBytes       int64     `json:"log_size_bytes"`
	LineCount       int       `json:"log_line_count"`
	StartedAt       time.Time `json:"started_at"`
}

// ArchiveTask represents a task to archive logs to MinIO.
type ArchiveTask struct {
	JobID           string
	ExecutionID     string
	ExecutionNumber int
	Content         []byte // Gzipped log content
	LineCount       int
	StartedAt       time.Time
}

// Config configures the log service.
type Config struct {
	// Enabled enables the job log capture system.
	Enabled bool `default:"true" env:"JOB_LOGS_ENABLED" yaml:"enabled"`

	// BufferSize is the max number of log entries to buffer in memory per job.
	BufferSize int `default:"1000" env:"JOB_LOGS_BUFFER_SIZE" yaml:"buffer_size"`

	// SSEEnabled enables live log streaming via SSE.
	SSEEnabled bool `default:"true" env:"JOB_LOGS_SSE_ENABLED" yaml:"sse_enabled"`

	// ArchiveEnabled enables MinIO archival of completed job logs.
	ArchiveEnabled bool `default:"true" env:"JOB_LOGS_ARCHIVE_ENABLED" yaml:"archive_enabled"`

	// RetentionDays is how long to keep archived logs in MinIO.
	RetentionDays int `default:"30" env:"JOB_LOGS_RETENTION_DAYS" yaml:"retention_days"`

	// MinLevel is the minimum log level to capture (debug, info, warn, error).
	MinLevel string `default:"info" env:"JOB_LOGS_MIN_LEVEL" yaml:"min_level"`

	// MinioBucket is the bucket name for log archives.
	MinioBucket string `default:"crawler-logs" env:"JOB_LOGS_MINIO_BUCKET" yaml:"minio_bucket"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Enabled:        true,
		BufferSize:     defaultBufferSize,
		SSEEnabled:     true,
		ArchiveEnabled: true,
		RetentionDays:  defaultRetentionDays,
		MinLevel:       defaultMinLevel,
		MinioBucket:    defaultMinioBucket,
	}
}
