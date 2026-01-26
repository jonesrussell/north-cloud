// Package worker provides a bounded worker pool for executing crawler jobs.
package worker

import (
	"errors"
	"time"
)

const (
	// DefaultPoolSize is the default number of workers in the pool.
	DefaultPoolSize = 10

	// DefaultDrainTimeout is the default timeout for graceful shutdown.
	DefaultDrainTimeout = 30 * time.Second

	// DefaultJobTimeout is the default timeout for individual job execution.
	DefaultJobTimeout = 1 * time.Hour

	// DefaultHealthCheckInterval is the default interval for worker health checks.
	DefaultHealthCheckInterval = 30 * time.Second

	// MinPoolSize is the minimum allowed pool size.
	MinPoolSize = 1

	// MaxPoolSize is the maximum allowed pool size.
	MaxPoolSize = 100

	// DefaultCircuitBreakerThreshold is the default failure threshold.
	DefaultCircuitBreakerThreshold = 5
)

// Config holds configuration for the worker pool.
type Config struct {
	// PoolSize is the number of concurrent workers.
	PoolSize int

	// DrainTimeout is the maximum time to wait for workers to finish during shutdown.
	DrainTimeout time.Duration

	// JobTimeout is the default timeout for job execution.
	JobTimeout time.Duration

	// HealthCheckInterval is the interval between worker health checks.
	HealthCheckInterval time.Duration

	// EnableCircuitBreaker enables per-domain circuit breakers.
	EnableCircuitBreaker bool

	// CircuitBreakerThreshold is the failure count threshold before opening the circuit.
	CircuitBreakerThreshold int

	// CircuitBreakerTimeout is the timeout before attempting to close the circuit.
	CircuitBreakerTimeout time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		PoolSize:                DefaultPoolSize,
		DrainTimeout:            DefaultDrainTimeout,
		JobTimeout:              DefaultJobTimeout,
		HealthCheckInterval:     DefaultHealthCheckInterval,
		EnableCircuitBreaker:    true,
		CircuitBreakerThreshold: DefaultCircuitBreakerThreshold,
		CircuitBreakerTimeout:   1 * time.Minute,
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.PoolSize < MinPoolSize {
		return errors.New("pool size must be at least 1")
	}
	if c.PoolSize > MaxPoolSize {
		return errors.New("pool size cannot exceed 100")
	}
	if c.DrainTimeout <= 0 {
		return errors.New("drain timeout must be positive")
	}
	if c.JobTimeout <= 0 {
		return errors.New("job timeout must be positive")
	}
	if c.HealthCheckInterval <= 0 {
		return errors.New("health check interval must be positive")
	}
	if c.EnableCircuitBreaker && c.CircuitBreakerThreshold <= 0 {
		return errors.New("circuit breaker threshold must be positive")
	}
	if c.EnableCircuitBreaker && c.CircuitBreakerTimeout <= 0 {
		return errors.New("circuit breaker timeout must be positive")
	}
	return nil
}

// WithPoolSize sets the pool size.
func (c *Config) WithPoolSize(size int) *Config {
	c.PoolSize = size
	return c
}

// WithDrainTimeout sets the drain timeout.
func (c *Config) WithDrainTimeout(timeout time.Duration) *Config {
	c.DrainTimeout = timeout
	return c
}

// WithJobTimeout sets the job timeout.
func (c *Config) WithJobTimeout(timeout time.Duration) *Config {
	c.JobTimeout = timeout
	return c
}

// WithCircuitBreaker enables circuit breaker with the given threshold and timeout.
func (c *Config) WithCircuitBreaker(threshold int, timeout time.Duration) *Config {
	c.EnableCircuitBreaker = true
	c.CircuitBreakerThreshold = threshold
	c.CircuitBreakerTimeout = timeout
	return c
}
