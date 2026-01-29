package logs

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	maxPerSecond int
	tokens       float64
	lastRefill   time.Time
	mu           sync.Mutex
}

// NewRateLimiter creates a new rate limiter.
// Returns nil if maxPerSecond <= 0 (disabled).
func NewRateLimiter(maxPerSecond int) *RateLimiter {
	if maxPerSecond <= 0 {
		return nil
	}
	return &RateLimiter{
		maxPerSecond: maxPerSecond,
		tokens:       float64(maxPerSecond),
		lastRefill:   time.Now(),
	}
}

// Allow returns true if a log can be emitted, false if throttled.
func (r *RateLimiter) Allow() bool {
	if r == nil {
		return true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.tokens += elapsed * float64(r.maxPerSecond)
	if r.tokens > float64(r.maxPerSecond) {
		r.tokens = float64(r.maxPerSecond)
	}
	r.lastRefill = now

	// Check if we have a token
	if r.tokens >= 1 {
		r.tokens--
		return true
	}

	return false
}

// Stats returns current throttler state for diagnostics.
func (r *RateLimiter) Stats() (tokens float64, maxPerSec int) {
	if r == nil {
		return 0, 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tokens, r.maxPerSecond
}
