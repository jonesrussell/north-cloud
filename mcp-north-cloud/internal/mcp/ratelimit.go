package mcp

import (
	"sync"
	"time"
)

// Rate limit constants.
const (
	globalRatePerMinute  = 60
	perToolRatePerMinute = 20
	windowDuration       = time.Minute
)

// RateLimiter provides in-process rate limiting using a sliding window counter.
// It enforces both a global limit and per-tool limits to prevent runaway LLM loops.
type RateLimiter struct {
	mu          sync.Mutex
	globalCalls []time.Time
	toolCalls   map[string][]time.Time
}

// NewRateLimiter creates a RateLimiter with default limits.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		toolCalls: make(map[string][]time.Time),
	}
}

// Allow checks if a tool call is permitted under both global and per-tool rate limits.
// Returns false if either limit is exceeded.
func (rl *RateLimiter) Allow(toolName string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-windowDuration)

	// Prune and check global limit
	rl.globalCalls = pruneOld(rl.globalCalls, cutoff)
	if len(rl.globalCalls) >= globalRatePerMinute {
		return false
	}

	// Prune and check per-tool limit
	rl.toolCalls[toolName] = pruneOld(rl.toolCalls[toolName], cutoff)
	if len(rl.toolCalls[toolName]) >= perToolRatePerMinute {
		return false
	}

	// Record the call
	rl.globalCalls = append(rl.globalCalls, now)
	rl.toolCalls[toolName] = append(rl.toolCalls[toolName], now)

	return true
}

// pruneOld removes timestamps older than cutoff.
func pruneOld(calls []time.Time, cutoff time.Time) []time.Time {
	i := 0
	for i < len(calls) && calls[i].Before(cutoff) {
		i++
	}
	return calls[i:]
}
