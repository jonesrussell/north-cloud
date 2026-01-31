package job_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/job"
)

func TestScheduleComputer_ComputeSchedule_NormalPriority(t *testing.T) {
	t.Helper()

	sc := job.NewScheduleComputer()
	output := sc.ComputeSchedule(job.ScheduleInput{
		RateLimit: 10,
		MaxDepth:  2,
		Priority:  "normal",
	})

	// Normal priority base: 60 minutes
	// Rate limit 10: no adjustment
	// Max depth 2: no adjustment
	// Expected: 60 minutes
	if output.IntervalMinutes != 60 {
		t.Errorf("expected 60 minutes, got %d", output.IntervalMinutes)
	}

	if output.NumericPriority != 50 {
		t.Errorf("expected priority 50, got %d", output.NumericPriority)
	}
}

func TestScheduleComputer_ComputeSchedule_HighPriority(t *testing.T) {
	t.Helper()

	sc := job.NewScheduleComputer()
	output := sc.ComputeSchedule(job.ScheduleInput{
		RateLimit: 10,
		MaxDepth:  2,
		Priority:  "high",
	})

	// High priority base: 30 minutes
	if output.IntervalMinutes != 30 {
		t.Errorf("expected 30 minutes, got %d", output.IntervalMinutes)
	}

	if output.NumericPriority != 75 {
		t.Errorf("expected priority 75, got %d", output.NumericPriority)
	}
}

func TestScheduleComputer_ComputeSchedule_LowRateLimit(t *testing.T) {
	t.Helper()

	sc := job.NewScheduleComputer()
	output := sc.ComputeSchedule(job.ScheduleInput{
		RateLimit: 3, // Low rate limit
		MaxDepth:  2,
		Priority:  "normal",
	})

	// Normal base: 60, low rate limit: +50% = 90 minutes
	if output.IntervalMinutes != 90 {
		t.Errorf("expected 90 minutes for low rate limit, got %d", output.IntervalMinutes)
	}
}

func TestScheduleComputer_ComputeSchedule_DeepCrawl(t *testing.T) {
	t.Helper()

	sc := job.NewScheduleComputer()
	output := sc.ComputeSchedule(job.ScheduleInput{
		RateLimit: 10,
		MaxDepth:  6, // Deep crawl
		Priority:  "normal",
	})

	// Normal base: 60, deep crawl: +50% = 90 minutes
	if output.IntervalMinutes != 90 {
		t.Errorf("expected 90 minutes for deep crawl, got %d", output.IntervalMinutes)
	}
}

func TestScheduleComputer_ComputeSchedule_WithFailures(t *testing.T) {
	t.Helper()

	sc := job.NewScheduleComputer()
	output := sc.ComputeSchedule(job.ScheduleInput{
		RateLimit:    10,
		MaxDepth:     2,
		Priority:     "normal",
		FailureCount: 2,
	})

	// Normal base: 60, 2 failures: 60 * 2^2 = 240 minutes
	if output.IntervalMinutes != 240 {
		t.Errorf("expected 240 minutes with 2 failures, got %d", output.IntervalMinutes)
	}
}

func TestScheduleComputer_ComputeSchedule_BackoffCapped(t *testing.T) {
	t.Helper()

	sc := job.NewScheduleComputer()
	output := sc.ComputeSchedule(job.ScheduleInput{
		RateLimit:    10,
		MaxDepth:     2,
		Priority:     "normal",
		FailureCount: 10, // Many failures
	})

	// Backoff capped at 24 hours = 1440 minutes
	maxBackoff := 24 * 60
	if output.IntervalMinutes > maxBackoff {
		t.Errorf("expected backoff capped at %d, got %d", maxBackoff, output.IntervalMinutes)
	}
}

func TestScheduleComputer_ComputeInitialDelay(t *testing.T) {
	t.Helper()

	sc := job.NewScheduleComputer()

	tests := []struct {
		priority      string
		expectedDelay time.Duration
	}{
		{"critical", 0},
		{"high", 1 * time.Minute},
		{"normal", 5 * time.Minute},
		{"low", 10 * time.Minute},
	}

	for _, tt := range tests {
		output := sc.ComputeSchedule(job.ScheduleInput{
			RateLimit: 10,
			MaxDepth:  2,
			Priority:  tt.priority,
		})

		if output.InitialDelay != tt.expectedDelay {
			t.Errorf("priority %s: expected delay %v, got %v",
				tt.priority, tt.expectedDelay, output.InitialDelay)
		}
	}
}
