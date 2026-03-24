package bootstrap_test

import (
	"context"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/bootstrap"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

func TestLogAdapter_Info(t *testing.T) {
	t.Parallel()

	log := infralogger.NewNop()
	adapter := bootstrap.NewLogAdapterForTest(log)

	// Should not panic
	bootstrap.LogAdapterInfo(adapter, "test message", "key", "value")
}

func TestLogAdapter_Warn(t *testing.T) {
	t.Parallel()

	log := infralogger.NewNop()
	adapter := bootstrap.NewLogAdapterForTest(log)

	// Should not panic
	bootstrap.LogAdapterWarn(adapter, "test warning", "key", "value")
}

func TestLogAdapter_Error(t *testing.T) {
	t.Parallel()

	log := infralogger.NewNop()
	adapter := bootstrap.NewLogAdapterForTest(log)

	// Should not panic
	bootstrap.LogAdapterError(adapter, "test error", "key", "value")
}

func TestLogAdapter_WithMultipleFields(t *testing.T) {
	t.Parallel()

	log := infralogger.NewNop()
	adapter := bootstrap.NewLogAdapterForTest(log)

	// Should handle multiple key-value pairs
	bootstrap.LogAdapterInfo(adapter, "multi", "k1", "v1", "k2", 42, "k3", true)
}

func TestLogAdapter_WithNoFields(t *testing.T) {
	t.Parallel()

	log := infralogger.NewNop()
	adapter := bootstrap.NewLogAdapterForTest(log)

	// Should handle no fields
	bootstrap.LogAdapterInfo(adapter, "no fields")
}

func TestBuildProxiedHTTPClient_NilPool(t *testing.T) {
	t.Parallel()

	client := bootstrap.BuildProxiedHTTPClientForTest(30*time.Second, infralogger.NewNop())
	if client == nil {
		t.Fatal("expected non-nil HTTP client")
	}
	if client.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", client.Timeout)
	}
	// With nil pool, transport should be nil (default)
	if client.Transport != nil {
		t.Error("expected nil Transport for nil pool")
	}
}

// mockFrontierStatsRepo implements api.FrontierRepoForHandler for stats testing.
type mockFrontierStatsRepo struct {
	stats *database.FrontierStats
	err   error
	calls int
}

func (m *mockFrontierStatsRepo) Stats(_ context.Context) (*database.FrontierStats, error) {
	m.calls++
	return m.stats, m.err
}

func (m *mockFrontierStatsRepo) List(_ context.Context, _ database.FrontierFilters) ([]*domain.FrontierURL, int, error) {
	return nil, 0, nil
}

func (m *mockFrontierStatsRepo) Submit(_ context.Context, _ database.SubmitParams) error {
	return nil
}

func (m *mockFrontierStatsRepo) ResetForRetry(_ context.Context, _ string) error {
	return nil
}

func (m *mockFrontierStatsRepo) Delete(_ context.Context, _ string) error {
	return nil
}

func TestRunFrontierStatsLogger_ContextCancellation(t *testing.T) {
	t.Parallel()

	repo := &mockFrontierStatsRepo{
		stats: &database.FrontierStats{
			TotalPending:  10,
			TotalFetching: 5,
			TotalFetched:  100,
			TotalFailed:   2,
			TotalDead:     1,
		},
	}
	log := infralogger.NewNop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	done := make(chan struct{})
	go func() {
		bootstrap.RunFrontierStatsLoggerForTest(ctx, repo, log)
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("runFrontierStatsLogger did not return after context cancellation")
	}

	// Should have been called at least once (the immediate call before the loop)
	if repo.calls == 0 {
		t.Error("expected at least one stats call")
	}
}

func TestRunFrontierStatsLogger_ErrorDoesNotPanic(t *testing.T) {
	t.Parallel()

	repo := &mockFrontierStatsRepo{
		err: context.DeadlineExceeded,
	}
	log := infralogger.NewNop()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		bootstrap.RunFrontierStatsLoggerForTest(ctx, repo, log)
		close(done)
	}()

	// Let it run one immediate call + one tick with error
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// ok - no panic
	case <-time.After(2 * time.Second):
		t.Fatal("did not return after cancel")
	}
}
