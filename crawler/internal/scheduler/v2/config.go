// Package v2 provides the V2 scheduler implementation with Redis Streams and cron support.
package v2

import (
	"errors"
	"time"
)

const (
	// DefaultWorkerPoolSize is the default number of workers.
	DefaultWorkerPoolSize = 10

	// DefaultQueueBatchSize is the default number of jobs to read per batch.
	DefaultQueueBatchSize = 50

	// DefaultLeaderTTL is the default leader election TTL.
	DefaultLeaderTTL = 30 * time.Second

	// DefaultPollInterval is the default interval for checking ready jobs.
	DefaultPollInterval = 10 * time.Second

	// DefaultJobTimeout is the default timeout for job execution.
	DefaultJobTimeout = 1 * time.Hour

	// DefaultDrainTimeout is the default timeout for graceful shutdown.
	DefaultDrainTimeout = 30 * time.Second

	// MinWorkerPoolSize is the minimum worker pool size.
	MinWorkerPoolSize = 1

	// MaxWorkerPoolSize is the maximum worker pool size.
	MaxWorkerPoolSize = 100
)

// Config holds configuration for the V2 scheduler.
type Config struct {
	// Enabled controls whether V2 scheduler is active.
	Enabled bool

	// Worker pool settings
	WorkerPoolSize int
	JobTimeout     time.Duration
	DrainTimeout   time.Duration

	// Queue settings
	QueueBatchSize int
	PollInterval   time.Duration

	// Redis settings
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	StreamPrefix  string

	// Leader election settings
	LeaderTTL time.Duration
	LeaderKey string

	// Feature flags
	EnableCronScheduling bool
	EnableEventTriggers  bool
	EnableCircuitBreaker bool
	EnableMetrics        bool
	EnableTracing        bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:              false, // Disabled by default for migration
		WorkerPoolSize:       DefaultWorkerPoolSize,
		JobTimeout:           DefaultJobTimeout,
		DrainTimeout:         DefaultDrainTimeout,
		QueueBatchSize:       DefaultQueueBatchSize,
		PollInterval:         DefaultPollInterval,
		StreamPrefix:         "crawler",
		LeaderTTL:            DefaultLeaderTTL,
		LeaderKey:            "crawler:scheduler:leader",
		EnableCronScheduling: true,
		EnableEventTriggers:  true,
		EnableCircuitBreaker: true,
		EnableMetrics:        true,
		EnableTracing:        false, // Disabled by default
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil // No validation needed if disabled
	}

	if c.WorkerPoolSize < MinWorkerPoolSize {
		return errors.New("worker pool size must be at least 1")
	}
	if c.WorkerPoolSize > MaxWorkerPoolSize {
		return errors.New("worker pool size cannot exceed 100")
	}
	if c.JobTimeout <= 0 {
		return errors.New("job timeout must be positive")
	}
	if c.DrainTimeout <= 0 {
		return errors.New("drain timeout must be positive")
	}
	if c.QueueBatchSize <= 0 {
		return errors.New("queue batch size must be positive")
	}
	if c.PollInterval <= 0 {
		return errors.New("poll interval must be positive")
	}
	if c.RedisAddr == "" {
		return errors.New("redis address is required")
	}
	if c.LeaderTTL <= 0 {
		return errors.New("leader TTL must be positive")
	}
	if c.LeaderKey == "" {
		return errors.New("leader key is required")
	}

	return nil
}

// WithEnabled sets whether the scheduler is enabled.
func (c *Config) WithEnabled(enabled bool) *Config {
	c.Enabled = enabled
	return c
}

// WithWorkerPoolSize sets the worker pool size.
func (c *Config) WithWorkerPoolSize(size int) *Config {
	c.WorkerPoolSize = size
	return c
}

// WithRedis sets the Redis connection settings.
func (c *Config) WithRedis(addr, password string, db int) *Config {
	c.RedisAddr = addr
	c.RedisPassword = password
	c.RedisDB = db
	return c
}

// WithStreamPrefix sets the Redis streams prefix.
func (c *Config) WithStreamPrefix(prefix string) *Config {
	c.StreamPrefix = prefix
	return c
}

// WithLeaderElection sets the leader election settings.
func (c *Config) WithLeaderElection(key string, ttl time.Duration) *Config {
	c.LeaderKey = key
	c.LeaderTTL = ttl
	return c
}

// WithFeatures enables or disables specific features.
func (c *Config) WithFeatures(cron, events, circuitBreaker, metrics, tracing bool) *Config {
	c.EnableCronScheduling = cron
	c.EnableEventTriggers = events
	c.EnableCircuitBreaker = circuitBreaker
	c.EnableMetrics = metrics
	c.EnableTracing = tracing
	return c
}
