// Package logs provides configuration for job log streaming and archival.
package logs

// Config represents job logs configuration.
type Config struct {
	// Enabled toggles job log capture on/off
	Enabled bool `env:"JOB_LOGS_ENABLED" yaml:"enabled"`
	// BufferSize is the max number of log entries to buffer in memory per job
	BufferSize int `env:"JOB_LOGS_BUFFER_SIZE" yaml:"buffer_size"`
	// SSEEnabled enables live log streaming via SSE
	SSEEnabled bool `env:"JOB_LOGS_SSE_ENABLED" yaml:"sse_enabled"`
	// ArchiveEnabled enables MinIO archival of completed job logs
	ArchiveEnabled bool `env:"JOB_LOGS_ARCHIVE_ENABLED" yaml:"archive_enabled"`
	// RetentionDays is how long to keep archived logs in MinIO
	RetentionDays int `env:"JOB_LOGS_RETENTION_DAYS" yaml:"retention_days"`
	// MinLevel is the minimum log level to capture (debug, info, warn, error)
	MinLevel string `env:"JOB_LOGS_MIN_LEVEL" yaml:"min_level"`
	// MinioBucket is the bucket name for log archives
	MinioBucket string `env:"JOB_LOGS_MINIO_BUCKET" yaml:"minio_bucket"`
	// MilestoneInterval is how often (in pages crawled) to emit progress milestones
	MilestoneInterval int `env:"JOB_LOGS_MILESTONE_INTERVAL" yaml:"milestone_interval"`

	// RedisEnabled enables Redis Streams for log persistence (replaces in-memory buffer)
	RedisEnabled bool `env:"JOB_LOGS_REDIS_ENABLED" yaml:"redis_enabled"`
	// RedisKeyPrefix is the prefix for Redis stream keys (e.g., "logs" -> "logs:{job_id}")
	RedisKeyPrefix string `env:"JOB_LOGS_REDIS_KEY_PREFIX" yaml:"redis_key_prefix"`
	// RedisTTLSeconds is how long to keep log streams in Redis (default 24 hours)
	RedisTTLSeconds int `env:"JOB_LOGS_REDIS_TTL_SECONDS" yaml:"redis_ttl_seconds"`
}

// Default values for logs configuration.
const (
	defaultBufferSize        = 1000
	defaultRetentionDays     = 30
	defaultMinLevel          = "info"
	defaultMinioBucket       = "crawler-logs"
	defaultMilestoneInterval = 50
	defaultRedisKeyPrefix    = "logs"
	defaultRedisTTLSeconds   = 86400 // 24 hours
)

// NewConfig returns a new logs configuration with default values.
func NewConfig() *Config {
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
