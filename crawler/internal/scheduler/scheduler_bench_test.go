package scheduler_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler"
)

// BenchmarkJobScheduling benchmarks the interval scheduler job processing
func BenchmarkJobScheduling(b *testing.B) {
	// Create mock jobs for benchmarking
	jobs := make([]*domain.Job, 100)
	now := time.Now()

	for i := range 100 {
		sourceName := "example.com"
		intervalMinutes := 30
		jobs[i] = &domain.Job{
			ID:              "job-" + string(rune(i+1)),
			SourceID:        "test-source",
			SourceName:      &sourceName,
			URL:             "https://example.com",
			Status:          string(scheduler.StatePending),
			IntervalMinutes: &intervalMinutes,
			IntervalType:    "minutes",
			NextRunAt:       &now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Simulate job filtering logic
		readyJobs := make([]*domain.Job, 0, 10)
		for _, job := range jobs {
			if job.NextRunAt != nil && job.NextRunAt.Before(time.Now()) && job.Status == string(scheduler.StatePending) {
				readyJobs = append(readyJobs, job)
			}
		}
		_ = readyJobs
	}
}

// BenchmarkLockAcquisition benchmarks distributed lock acquisition logic
func BenchmarkLockAcquisition(b *testing.B) {
	sourceName := "example.com"
	job := &domain.Job{
		ID:         "job-1",
		SourceID:   "test-source",
		SourceName: &sourceName,
		URL:        "https://example.com",
		Status:     string(scheduler.StatePending),
	}
	_ = job.ID
	_ = job.SourceID
	_ = job.SourceName
	_ = job.URL
	_ = job.Status

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Simulate lock token generation
		lockToken := generateLockToken()

		// Simulate lock check
		isLocked := job.LockToken != nil && job.LockToken == &lockToken
		_ = isLocked
	}
}

// BenchmarkNextRunCalculation benchmarks calculating next run time for intervals
func BenchmarkNextRunCalculation(b *testing.B) {
	baseTime := time.Now()

	testCases := []struct {
		intervalMinutes int
		intervalType    string
	}{
		{30, "minutes"},
		{1, "hours"},
		{6, "hours"},
		{1, "days"},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		for _, tc := range testCases {
			var duration time.Duration

			switch tc.intervalType {
			case "minutes":
				duration = time.Duration(tc.intervalMinutes) * time.Minute
			case "hours":
				duration = time.Duration(tc.intervalMinutes) * time.Hour
			case "days":
				duration = time.Duration(tc.intervalMinutes) * 24 * time.Hour
			default:
				duration = time.Duration(tc.intervalMinutes) * time.Minute
			}

			_ = baseTime.Add(duration)
		}
	}
}

// BenchmarkExponentialBackoff benchmarks retry backoff calculation
func BenchmarkExponentialBackoff(b *testing.B) {
	const baseBackoffSeconds = 60
	const maxBackoffSeconds = 3600

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Simulate exponential backoff for up to 5 retries
		for attempt := 1; attempt <= 5; attempt++ {
			// Calculate: base × 2^(attempt-1)
			backoff := baseBackoffSeconds * (1 << uint(attempt-1))

			// Cap at max backoff
			if backoff > maxBackoffSeconds {
				backoff = maxBackoffSeconds
			}

			_ = time.Duration(backoff) * time.Second
		}
	}
}

// BenchmarkJobStatusTransition benchmarks state machine validation
func BenchmarkJobStatusTransition(b *testing.B) {
	testCases := []struct {
		from scheduler.JobState
		to   scheduler.JobState
	}{
		{scheduler.StatePending, scheduler.StateScheduled},
		{scheduler.StateScheduled, scheduler.StateRunning},
		{scheduler.StateRunning, scheduler.StateCompleted},
		{scheduler.StateRunning, scheduler.StateFailed},
		{scheduler.StateRunning, scheduler.StatePaused},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		for _, tc := range testCases {
			_ = scheduler.ValidateStateTransition(tc.from, tc.to)
		}
	}
}

// BenchmarkMetricsCollection benchmarks thread-safe metrics updates
func BenchmarkMetricsCollection(b *testing.B) {
	metrics := &scheduler.SchedulerMetrics{}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Simulate concurrent metrics updates
		metrics.IncrementScheduled()
		metrics.IncrementCompleted()
		metrics.UpdateAggregateMetrics(100.0, 0.95)
	}
}

// BenchmarkJobExecutionHistory benchmarks creating execution records
func BenchmarkJobExecutionHistory(b *testing.B) {
	now := time.Now()

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		durationMs := int64(120000) // 120 seconds in milliseconds
		cpuTimeMs := int64(500)
		memoryPeakMB := 10
		execution := &domain.JobExecution{
			ID:              "exec-" + string(rune(i+1)),
			JobID:           "job-1",
			ExecutionNumber: i + 1,
			Status:          "completed",
			StartedAt:       now,
			CompletedAt:     &now,
			DurationMs:      &durationMs,
			ItemsCrawled:    50,
			ItemsIndexed:    50,
			ErrorMessage:    nil,
			CPUTimeMs:       &cpuTimeMs,
			MemoryPeakMB:    &memoryPeakMB,
		}

		_ = execution.ID
		_ = execution.JobID
		_ = execution.ExecutionNumber
		_ = execution.Status
		_ = execution.StartedAt
		_ = execution.CompletedAt
		_ = execution.DurationMs
		_ = execution.ItemsCrawled
		_ = execution.ItemsIndexed
		_ = execution.ErrorMessage
		_ = execution.CPUTimeMs
		_ = execution.MemoryPeakMB
	}
}

// Helper function to generate lock tokens (simulated)
func generateLockToken() string {
	return "lock-token-12345678"
}
