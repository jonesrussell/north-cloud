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

	// Should not panic
	provider.RecordClassification(ctx, "test-source", true, 100*time.Millisecond)
	provider.RecordClassification(ctx, "test-source", false, 50*time.Millisecond)
}

func TestRecordRuleMatch(t *testing.T) {
	provider := getTestProvider(t)
	ctx := context.Background()

	// Should not panic
	provider.RecordRuleMatch(ctx, 5*time.Millisecond, 25, 3)
}

func TestSetQueueDepth(t *testing.T) {
	provider := getTestProvider(t)

	// Should not panic
	provider.SetQueueDepth(100)
	provider.SetActiveWorkers(5)
}
