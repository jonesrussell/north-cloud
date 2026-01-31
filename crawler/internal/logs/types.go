// Package logs provides job execution log streaming and archival functionality.
package logs

import "time"

// CurrentSchemaVersion is the current version of the log entry schema.
// Increment when making breaking changes to LogEntry structure.
const CurrentSchemaVersion = 1

// Configuration constants.
const (
	defaultBufferSize        = 1000
	defaultRetentionDays     = 30
	defaultMinLevel          = "info"
	defaultMinioBucket       = "crawler-logs"
	defaultMilestoneInterval = 50
	defaultRedisKeyPrefix    = "logs"
	defaultRedisTTLSeconds   = 86400 // 24 hours
)

// LogEntry represents a single log line captured during job execution.
type LogEntry struct {
	SchemaVersion int            `json:"schema_version"`
	Timestamp     time.Time      `json:"timestamp"`
	Level         string         `json:"level"` // debug, info, warn, error
	Category      string         `json:"category"`
	Message       string         `json:"message"`
	JobID         string         `json:"job_id"`
	ExecID        string         `json:"execution_id"`
	Fields        map[string]any `json:"fields,omitempty"`
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

	// MilestoneInterval is how often (in pages crawled) to emit progress milestones.
	MilestoneInterval int `default:"50" env:"JOB_LOGS_MILESTONE_INTERVAL" yaml:"milestone_interval"`

	// RedisEnabled enables Redis Streams for log persistence (replaces in-memory buffer).
	RedisEnabled bool `default:"false" env:"JOB_LOGS_REDIS_ENABLED" yaml:"redis_enabled"`

	// RedisKeyPrefix is the prefix for Redis stream keys (e.g., "logs" -> "logs:{job_id}").
	RedisKeyPrefix string `default:"logs" env:"JOB_LOGS_REDIS_KEY_PREFIX" yaml:"redis_key_prefix"`

	// RedisTTLSeconds is how long to keep log streams in Redis (default 24 hours).
	RedisTTLSeconds int `default:"86400" env:"JOB_LOGS_REDIS_TTL_SECONDS" yaml:"redis_ttl_seconds"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Enabled:           true,
		BufferSize:        defaultBufferSize,
		SSEEnabled:        true,
		ArchiveEnabled:    true,
		RetentionDays:     defaultRetentionDays,
		MinLevel:          defaultMinLevel,
		MinioBucket:       defaultMinioBucket,
		MilestoneInterval: defaultMilestoneInterval,
		RedisEnabled:      false,
		RedisKeyPrefix:    defaultRedisKeyPrefix,
		RedisTTLSeconds:   defaultRedisTTLSeconds,
	}
}
