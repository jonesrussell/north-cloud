package scheduler

import (
	"sync"
	"testing"
	"time"
)

func TestSchedulerMetrics_IncrementCounters(t *testing.T) {
	m := &SchedulerMetrics{}

	// Test all increment methods
	m.IncrementScheduled()
	m.IncrementRunning()
	m.IncrementCompleted()
	m.IncrementFailed()
	m.IncrementCancelled()
	m.IncrementTotalExecutions()

	// Verify values
	if m.JobsScheduled != 1 {
		t.Errorf("JobsScheduled = %d, want 1", m.JobsScheduled)
	}
	if m.JobsRunning != 1 {
		t.Errorf("JobsRunning = %d, want 1", m.JobsRunning)
	}
	if m.JobsCompleted != 1 {
		t.Errorf("JobsCompleted = %d, want 1", m.JobsCompleted)
	}
	if m.JobsFailed != 1 {
		t.Errorf("JobsFailed = %d, want 1", m.JobsFailed)
	}
	if m.JobsCancelled != 1 {
		t.Errorf("JobsCancelled = %d, want 1", m.JobsCancelled)
	}
	if m.TotalExecutions != 1 {
		t.Errorf("TotalExecutions = %d, want 1", m.TotalExecutions)
	}
}

func TestSchedulerMetrics_DecrementRunning(t *testing.T) {
	m := &SchedulerMetrics{}

	m.IncrementRunning()
	m.IncrementRunning()
	m.IncrementRunning()

	if m.JobsRunning != 3 {
		t.Errorf("JobsRunning = %d, want 3", m.JobsRunning)
	}

	m.DecrementRunning()

	if m.JobsRunning != 2 {
		t.Errorf("JobsRunning = %d, want 2 after decrement", m.JobsRunning)
	}
}

func TestSchedulerMetrics_UpdateAggregateMetrics(t *testing.T) {
	m := &SchedulerMetrics{}

	avgDuration := 1500.5
	successRate := 0.85

	m.UpdateAggregateMetrics(avgDuration, successRate)

	if m.AverageDurationMs != avgDuration {
		t.Errorf("AverageDurationMs = %f, want %f", m.AverageDurationMs, avgDuration)
	}
	if m.SuccessRate != successRate {
		t.Errorf("SuccessRate = %f, want %f", m.SuccessRate, successRate)
	}
	if m.LastMetricsUpdate.IsZero() {
		t.Error("LastMetricsUpdate should be set")
	}
}

func TestSchedulerMetrics_UpdateLastCheck(t *testing.T) {
	m := &SchedulerMetrics{}

	if !m.LastCheckAt.IsZero() {
		t.Error("LastCheckAt should be zero initially")
	}

	m.UpdateLastCheck()

	if m.LastCheckAt.IsZero() {
		t.Error("LastCheckAt should be set after UpdateLastCheck")
	}
}

func TestSchedulerMetrics_AddStaleLocksCleared(t *testing.T) {
	m := &SchedulerMetrics{}

	m.AddStaleLocksCleared(5)
	if m.StaleLocksCleared != 5 {
		t.Errorf("StaleLocksCleared = %d, want 5", m.StaleLocksCleared)
	}

	m.AddStaleLocksCleared(3)
	if m.StaleLocksCleared != 8 {
		t.Errorf("StaleLocksCleared = %d, want 8", m.StaleLocksCleared)
	}
}

func TestSchedulerMetrics_Snapshot(t *testing.T) {
	m := &SchedulerMetrics{}

	// Set some values
	m.IncrementScheduled()
	m.IncrementRunning()
	m.IncrementCompleted()
	m.IncrementFailed()
	m.IncrementCancelled()
	m.IncrementTotalExecutions()
	m.UpdateAggregateMetrics(1000.0, 0.9)
	m.UpdateLastCheck()
	m.AddStaleLocksCleared(2)

	// Take snapshot
	snapshot := m.Snapshot()

	// Verify snapshot matches original
	if snapshot.JobsScheduled != m.JobsScheduled {
		t.Errorf("Snapshot JobsScheduled = %d, want %d", snapshot.JobsScheduled, m.JobsScheduled)
	}
	if snapshot.JobsRunning != m.JobsRunning {
		t.Errorf("Snapshot JobsRunning = %d, want %d", snapshot.JobsRunning, m.JobsRunning)
	}
	if snapshot.JobsCompleted != m.JobsCompleted {
		t.Errorf("Snapshot JobsCompleted = %d, want %d", snapshot.JobsCompleted, m.JobsCompleted)
	}
	if snapshot.JobsFailed != m.JobsFailed {
		t.Errorf("Snapshot JobsFailed = %d, want %d", snapshot.JobsFailed, m.JobsFailed)
	}
	if snapshot.JobsCancelled != m.JobsCancelled {
		t.Errorf("Snapshot JobsCancelled = %d, want %d", snapshot.JobsCancelled, m.JobsCancelled)
	}
	if snapshot.TotalExecutions != m.TotalExecutions {
		t.Errorf("Snapshot TotalExecutions = %d, want %d", snapshot.TotalExecutions, m.TotalExecutions)
	}
	if snapshot.AverageDurationMs != m.AverageDurationMs {
		t.Errorf("Snapshot AverageDurationMs = %f, want %f", snapshot.AverageDurationMs, m.AverageDurationMs)
	}
	if snapshot.SuccessRate != m.SuccessRate {
		t.Errorf("Snapshot SuccessRate = %f, want %f", snapshot.SuccessRate, m.SuccessRate)
	}
	if snapshot.StaleLocksCleared != m.StaleLocksCleared {
		t.Errorf("Snapshot StaleLocksCleared = %d, want %d", snapshot.StaleLocksCleared, m.StaleLocksCleared)
	}
}

// TestSchedulerMetrics_Concurrency tests thread safety
func TestSchedulerMetrics_Concurrency(t *testing.T) {
	m := &SchedulerMetrics{}
	var wg sync.WaitGroup

	// Number of concurrent goroutines
	numGoroutines := 100
	incrementsPerGoroutine := 100

	// Test concurrent increments
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				m.IncrementScheduled()
				m.IncrementRunning()
				m.DecrementRunning()
				m.IncrementCompleted()
				m.IncrementFailed()
				m.IncrementCancelled()
				m.IncrementTotalExecutions()
				m.UpdateLastCheck()
				m.AddStaleLocksCleared(1)
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify final counts
	expected := int64(numGoroutines * incrementsPerGoroutine)

	if m.JobsScheduled != expected {
		t.Errorf("JobsScheduled = %d, want %d", m.JobsScheduled, expected)
	}
	if m.JobsRunning != 0 { // Should be 0 because we increment and decrement
		t.Errorf("JobsRunning = %d, want 0", m.JobsRunning)
	}
	if m.JobsCompleted != expected {
		t.Errorf("JobsCompleted = %d, want %d", m.JobsCompleted, expected)
	}
	if m.JobsFailed != expected {
		t.Errorf("JobsFailed = %d, want %d", m.JobsFailed, expected)
	}
	if m.JobsCancelled != expected {
		t.Errorf("JobsCancelled = %d, want %d", m.JobsCancelled, expected)
	}
	if m.TotalExecutions != expected {
		t.Errorf("TotalExecutions = %d, want %d", m.TotalExecutions, expected)
	}
	if m.StaleLocksCleared != expected {
		t.Errorf("StaleLocksCleared = %d, want %d", m.StaleLocksCleared, expected)
	}
}

// TestSchedulerMetrics_ConcurrentSnapshots tests concurrent read/write operations
func TestSchedulerMetrics_ConcurrentSnapshots(t *testing.T) {
	m := &SchedulerMetrics{}
	var wg sync.WaitGroup

	// Writers
	numWriters := 10
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				m.IncrementScheduled()
				m.UpdateAggregateMetrics(float64(j), 0.5)
				time.Sleep(time.Microsecond)
			}
		}()
	}

	// Readers
	numReaders := 10
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = m.Snapshot()
				time.Sleep(time.Microsecond)
			}
		}()
	}

	wg.Wait()

	// Should complete without race conditions or deadlocks
}
