package telemetry_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/telemetry"
)

// providerOnce ensures we only create one Provider per test run to avoid
// duplicate Prometheus metric registration errors from promauto's global registry
var (
	testProvider *telemetry.Provider
	providerOnce sync.Once
)

func getTestProvider(t *testing.T) *telemetry.Provider {
	t.Helper()
	providerOnce.Do(func() {
		testProvider = telemetry.NewProvider()
	})
	return testProvider
}

func TestNewProvider(t *testing.T) {
	provider := getTestProvider(t)
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
	if provider.Tracer == nil {
		t.Error("expected non-nil tracer")
	}
	if provider.Metrics == nil {
		t.Error("expected non-nil metrics")
	}
}

func TestRecordClassification(t *testing.T) {
	provider := getTestProvider(t)
	ctx := context.Background()

	// Record multiple classifications
	provider.RecordClassification(ctx, "test-source-1", true, 100*time.Millisecond)
	provider.RecordClassification(ctx, "test-source-1", true, 50*time.Millisecond)
	provider.RecordClassification(ctx, "test-source-2", false, 200*time.Millisecond)

	// Verify metrics are accessible (non-nil)
	if provider.Metrics == nil {
		t.Error("metrics should be accessible after recording")
	}

	// Test with zero duration (edge case)
	provider.RecordClassification(ctx, "test-source-3", true, 0)

	// Test with very long duration (edge case)
	provider.RecordClassification(ctx, "test-source-4", false, 10*time.Second)
}

func TestRecordRuleMatch(t *testing.T) {
	provider := getTestProvider(t)
	ctx := context.Background()

	testCases := []struct {
		name       string
		duration   time.Duration
		rulesCount int
		matchCount int
	}{
		{"fast with matches", 5 * time.Millisecond, 25, 3},
		{"slow with no matches", 100 * time.Millisecond, 50, 0},
		{"zero duration", 0, 10, 1},
		{"many matches", 10 * time.Millisecond, 100, 50},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Should complete without panic
			provider.RecordRuleMatch(ctx, tc.duration, tc.rulesCount, tc.matchCount)
		})
	}
}

func TestSetQueueDepth(t *testing.T) {
	provider := getTestProvider(t)

	testCases := []struct {
		name    string
		depth   int
		workers int
	}{
		{"normal load", 100, 5},
		{"empty queue", 0, 5},
		{"high load", 10000, 10},
		{"single worker", 50, 1},
		{"no workers", 0, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Should complete without panic and accept any valid int
			provider.SetQueueDepth(tc.depth)
			provider.SetActiveWorkers(tc.workers)
		})
	}
}

func TestRecordClassification_Concurrency(t *testing.T) {
	provider := getTestProvider(t)
	ctx := context.Background()

	// Test concurrent access to metrics
	var wg sync.WaitGroup
	concurrency := 10
	iterations := 100

	for i := range concurrency {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := range iterations {
				provider.RecordClassification(ctx, "concurrent-source", j%2 == 0, time.Duration(j)*time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	// If we get here without panic/race, concurrent access is safe
}
