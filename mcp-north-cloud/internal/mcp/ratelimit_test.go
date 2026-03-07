//nolint:testpackage // testing unexported RateLimiter internals
package mcp

import (
	"fmt"
	"testing"
)

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter()

	for i := range perToolRatePerMinute {
		if !rl.Allow("test_tool") {
			t.Fatalf("call %d should be allowed (under per-tool limit)", i+1)
		}
	}
}

func TestRateLimiter_DeniesPerToolOverLimit(t *testing.T) {
	rl := NewRateLimiter()

	// Exhaust per-tool limit
	for range perToolRatePerMinute {
		rl.Allow("test_tool")
	}

	if rl.Allow("test_tool") {
		t.Error("should deny after per-tool limit is exhausted")
	}
}

func TestRateLimiter_OtherToolStillAllowed(t *testing.T) {
	rl := NewRateLimiter()

	// Exhaust per-tool limit for one tool
	for range perToolRatePerMinute {
		rl.Allow("tool_a")
	}

	// Different tool should still be allowed
	if !rl.Allow("tool_b") {
		t.Error("different tool should still be allowed")
	}
}

func TestRateLimiter_DeniesGlobalOverLimit(t *testing.T) {
	rl := NewRateLimiter()

	// Use different tool names to avoid per-tool limit
	for i := range globalRatePerMinute {
		toolName := fmt.Sprintf("tool_%d", i)
		rl.Allow(toolName)
	}

	if rl.Allow("another_tool") {
		t.Error("should deny after global limit is exhausted")
	}
}
