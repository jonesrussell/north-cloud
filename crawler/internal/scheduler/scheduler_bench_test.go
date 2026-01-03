package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

// BenchmarkJobScheduling benchmarks the interval scheduler job processing
func BenchmarkJobScheduling(b *testing.B) {
	// Create mock jobs for benchmarking
	jobs := make([]*domain.Job, 100)
	now := time.Now()

	for i := 0; i < 100; i++ {
		jobs[i] = &domain.Job{
			ID:              int64(i + 1),
			SourceID:        "test-source",
			SourceName:      "example.com",
			URL:             "https://example.com",
			Status:          domain.JobStatusPending,
			IntervalMinutes: 30,
			IntervalType:    "minutes",
			NextRunAt:       &now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate job filtering logic
		readyJobs := make([]*domain.Job, 0, 10)
		for _, job := range jobs {
			if job.NextRunAt != nil && job.NextRunAt.Before(time.Now()) && job.Status == domain.JobStatusPending {
				readyJobs = append(readyJobs, job)
			}
		}
	}
}

// BenchmarkLockAcquisition benchmarks distributed lock acquisition logic
func BenchmarkLockAcquisition(b *testing.B) {
	job := &domain.Job{
		ID:         1,
		SourceID:   "test-source",
		SourceName: "example.com",
		URL:        "https://example.com",
		Status:     domain.JobStatusPending,
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
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

	for i := 0; i < b.N; i++ {
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

	for i := 0; i < b.N; i++ {
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
	stateMachine := NewStateMachine()

	testCases := []struct {
		from domain.JobStatus
		to   domain.JobStatus
	}{
		{domain.JobStatusPending, domain.JobStatusScheduled},
		{domain.JobStatusScheduled, domain.JobStatusRunning},
		{domain.JobStatusRunning, domain.JobStatusCompleted},
		{domain.JobStatusRunning, domain.JobStatusFailed},
		{domain.JobStatusRunning, domain.JobStatusPaused},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			_ = stateMachine.CanTransition(tc.from, tc.to)
		}
	}
}

// BenchmarkMetricsCollection benchmarks thread-safe metrics updates
func BenchmarkMetricsCollection(b *testing.B) {
	metrics := NewMetrics()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate concurrent metrics updates
		metrics.IncrementJobsProcessed()
		metrics.IncrementJobsSucceeded()
		metrics.RecordJobDuration(100 * time.Millisecond)
	}
}

// BenchmarkJobExecutionHistory benchmarks creating execution records
func BenchmarkJobExecutionHistory(b *testing.B) {
	now := time.Now()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		execution := &domain.JobExecution{
			JobID:               1,
			ExecutionNumber:     int64(i + 1),
			Status:              "completed",
			StartTime:           now,
			EndTime:             &now,
			DurationSeconds:     120,
			ItemsCrawled:        50,
			ItemsIndexed:        50,
			ErrorMessage:        nil,
			MemoryUsedBytes:     1024 * 1024 * 10, // 10MB
			CPUTimeMilliseconds: 500,
		}

		_ = execution
	}
}

// Helper function to generate lock tokens (simulated)
func generateLockToken() string {
	return "lock-token-12345678"
}
