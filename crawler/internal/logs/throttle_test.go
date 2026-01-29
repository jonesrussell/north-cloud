package logs_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
)

func TestRateLimiter_Allow(t *testing.T) {
	t.Helper()

	t.Run("nil limiter always allows", func(t *testing.T) {
		var r *logs.RateLimiter
		for range 100 {
			if !r.Allow() {
				t.Errorf("nil RateLimiter.Allow() = false, want true")
			}
		}
	})

	t.Run("zero rate disables limiting", func(t *testing.T) {
		r := logs.NewRateLimiter(0)
		if r != nil {
			t.Errorf("NewRateLimiter(0) should return nil")
		}
	})

	t.Run("respects rate limit", func(t *testing.T) {
		r := logs.NewRateLimiter(10) // 10 per second

		// Should allow first 10
		allowed := 0
		for range 15 {
			if r.Allow() {
				allowed++
			}
		}

		if allowed != 10 {
			t.Errorf("allowed %d, want 10", allowed)
		}
	})

	t.Run("refills over time", func(t *testing.T) {
		r := logs.NewRateLimiter(10)

		// Exhaust tokens
		for range 10 {
			r.Allow()
		}

		// Should be denied
		if r.Allow() {
			t.Error("should be denied after exhausting tokens")
		}

		// Wait for refill
		time.Sleep(150 * time.Millisecond)

		// Should allow again (1-2 tokens refilled)
		if !r.Allow() {
			t.Error("should allow after refill")
		}
	})
}

func TestRateLimiter_Stats(t *testing.T) {
	t.Helper()

	t.Run("nil limiter returns zeros", func(t *testing.T) {
		var r *logs.RateLimiter
		tokens, maxRate := r.Stats()
		if tokens != 0 || maxRate != 0 {
			t.Errorf("nil Stats() = (%v, %v), want (0, 0)", tokens, maxRate)
		}
	})

	t.Run("returns current state", func(t *testing.T) {
		r := logs.NewRateLimiter(50)
		_, maxRate := r.Stats()
		if maxRate != 50 {
			t.Errorf("Stats() max = %v, want 50", maxRate)
		}
	})
}
