package bootstrap_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/bootstrap"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// mockStaleURLRecoverer implements bootstrap.StaleURLRecoverer for testing.
type mockStaleURLRecoverer struct {
	recovered int
	err       error
	calls     int
}

func (m *mockStaleURLRecoverer) RecoverStaleURLs(_ context.Context, _ time.Time) (int, error) {
	m.calls++
	return m.recovered, m.err
}

func TestRunStaleURLRecovery_ContextCancellation(t *testing.T) {
	t.Parallel()

	repo := &mockStaleURLRecoverer{recovered: 0, err: nil}
	log := infralogger.NewNop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Should return quickly since context is cancelled
	done := make(chan struct{})
	go func() {
		bootstrap.RunStaleURLRecoveryForTest(ctx, repo, log, time.Minute, 50*time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("runStaleURLRecovery did not return after context cancellation")
	}
}

func TestRunStaleURLRecovery_RecoversCalls(t *testing.T) {
	t.Parallel()

	repo := &mockStaleURLRecoverer{recovered: 3, err: nil}
	log := infralogger.NewNop()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		bootstrap.RunStaleURLRecoveryForTest(ctx, repo, log, time.Minute, 50*time.Millisecond)
		close(done)
	}()

	// Wait for at least one tick
	time.Sleep(150 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("runStaleURLRecovery did not return after cancel")
	}

	if repo.calls == 0 {
		t.Error("expected at least one recovery call")
	}
}

func TestRunStaleURLRecovery_ErrorDoesNotPanic(t *testing.T) {
	t.Parallel()

	repo := &mockStaleURLRecoverer{recovered: 0, err: errors.New("db error")}
	log := infralogger.NewNop()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		bootstrap.RunStaleURLRecoveryForTest(ctx, repo, log, time.Minute, 50*time.Millisecond)
		close(done)
	}()

	// Let it run a couple ticks with errors
	time.Sleep(150 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// ok - no panic
	case <-time.After(2 * time.Second):
		t.Fatal("runStaleURLRecovery did not return after cancel")
	}
}
