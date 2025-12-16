// Package retry provides retry utilities with exponential backoff for transient failures.
package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

var (
	// ErrMaxAttemptsExceeded is returned when max retry attempts are exceeded
	ErrMaxAttemptsExceeded = errors.New("max retry attempts exceeded")
	// ErrContextCancelled is returned when the context is cancelled during retry
	ErrContextCancelled = errors.New("context cancelled during retry")
)

// Config configures retry behavior
type Config struct {
	// MaxAttempts is the maximum number of retry attempts (including initial attempt)
	MaxAttempts int
	// InitialDelay is the initial delay before the first retry
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries (caps exponential backoff)
	MaxDelay time.Duration
	// Multiplier is the exponential backoff multiplier (default: 2.0)
	Multiplier float64
	// IsRetryable determines if an error should be retried
	IsRetryable func(error) bool
}

// DefaultConfig returns a default retry configuration
func DefaultConfig() Config {
	return Config{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		IsRetryable:  DefaultIsRetryable,
	}
}

// DefaultIsRetryable determines if an error is retryable by default
// Returns true for network errors, timeouts, and 5xx HTTP errors
func DefaultIsRetryable(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Check for common retryable error patterns
	retryablePatterns := []string{
		"timeout",
		"deadline exceeded",
		"connection refused",
		"connection reset",
		"no such host",
		"temporary failure",
		"network is unreachable",
		"i/o timeout",
		"context deadline exceeded",
	}

	for _, pattern := range retryablePatterns {
		if contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	// Simple case-insensitive contains check
	sLower := toLower(s)
	substrLower := toLower(substr)
	
	if len(sLower) < len(substrLower) {
		return false
	}
	
	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}
	return false
}

// toLower converts a string to lowercase (simple implementation)
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// Retry executes a function with retry logic and exponential backoff
func Retry(ctx context.Context, config Config, fn func() error) error {
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 3
	}
	if config.InitialDelay <= 0 {
		config.InitialDelay = 100 * time.Millisecond
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 30 * time.Second
	}
	if config.Multiplier <= 0 {
		config.Multiplier = 2.0
	}
	if config.IsRetryable == nil {
		config.IsRetryable = DefaultIsRetryable
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Check context cancellation
		if ctx.Err() != nil {
			return fmt.Errorf("%w: %v", ErrContextCancelled, ctx.Err())
		}

		// Execute the function
		err := fn()
		if err == nil {
			// Success - no retry needed
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !config.IsRetryable(err) {
			return err
		}

		// Don't sleep after the last attempt
		if attempt < config.MaxAttempts {
			// Calculate exponential backoff delay
			backoffDelay := time.Duration(float64(delay) * math.Pow(config.Multiplier, float64(attempt-1)))
			if backoffDelay > config.MaxDelay {
				backoffDelay = config.MaxDelay
			}

			// Wait with context cancellation support
			select {
			case <-ctx.Done():
				return fmt.Errorf("%w: %v", ErrContextCancelled, ctx.Err())
			case <-time.After(backoffDelay):
				// Continue to next attempt
			}
		}
	}

	return fmt.Errorf("%w after %d attempts: %w", ErrMaxAttemptsExceeded, config.MaxAttempts, lastErr)
}

// RetryWithDefaults executes a function with default retry configuration
func RetryWithDefaults(ctx context.Context, fn func() error) error {
	return Retry(ctx, DefaultConfig(), fn)
}

