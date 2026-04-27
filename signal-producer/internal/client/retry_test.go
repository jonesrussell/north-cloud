package client

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

func retryTestBackoffs() []time.Duration {
	return []time.Duration{
		5 * time.Millisecond,
		10 * time.Millisecond,
		15 * time.Millisecond,
	}
}

func TestRetry_FirstAttemptSucceeds(t *testing.T) {
	t.Parallel()
	var calls int32
	op := func(_ context.Context) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}
	if err := retry(context.Background(), retryTestBackoffs(), op, infralogger.NewNop()); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if c := atomic.LoadInt32(&calls); c != 1 {
		t.Errorf("calls: got %d, want 1", c)
	}
}

func TestRetry_RetryableThenSuccess(t *testing.T) {
	t.Parallel()
	var calls int32
	op := func(_ context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return fmt.Errorf("transient: %w", errServer)
		}
		return nil
	}
	if err := retry(context.Background(), retryTestBackoffs(), op, infralogger.NewNop()); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if c := atomic.LoadInt32(&calls); c != 3 {
		t.Errorf("calls: got %d, want 3", c)
	}
}

func TestRetry_ClientErrorShortCircuits(t *testing.T) {
	t.Parallel()
	var calls int32
	op := func(_ context.Context) error {
		atomic.AddInt32(&calls, 1)
		return fmt.Errorf("bad: %w", errClient)
	}
	err := retry(context.Background(), retryTestBackoffs(), op, infralogger.NewNop())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errClient) {
		t.Errorf("expected errClient, got %v", err)
	}
	if c := atomic.LoadInt32(&calls); c != 1 {
		t.Errorf("calls: got %d, want 1 (no retry on 4xx)", c)
	}
}

func TestRetry_ExhaustsBackoffs(t *testing.T) {
	t.Parallel()
	var calls int32
	op := func(_ context.Context) error {
		atomic.AddInt32(&calls, 1)
		return fmt.Errorf("still bad: %w", errServer)
	}
	err := retry(context.Background(), retryTestBackoffs(), op, infralogger.NewNop())
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if !errors.Is(err, errServer) {
		t.Errorf("expected errServer wrap, got %v", err)
	}
	// 1 initial + 3 retries.
	if c := atomic.LoadInt32(&calls); c != 4 {
		t.Errorf("calls: got %d, want 4", c)
	}
}

func TestRetry_ContextCancelDuringSleep(t *testing.T) {
	t.Parallel()
	var calls int32
	op := func(_ context.Context) error {
		atomic.AddInt32(&calls, 1)
		return fmt.Errorf("server: %w", errServer)
	}
	ctx, cancel := context.WithCancel(context.Background())
	// Use a long backoff and cancel before the timer fires.
	backoffs := []time.Duration{500 * time.Millisecond}
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	err := retry(ctx, backoffs, op, infralogger.NewNop())
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error from cancel")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("cancel was not honored promptly; elapsed=%v", elapsed)
	}
	// Initial call counts as 1; cancel happened before retry attempt.
	if c := atomic.LoadInt32(&calls); c != 1 {
		t.Errorf("calls: got %d, want 1", c)
	}
}

func TestRetry_TransportErrorIsRetryable(t *testing.T) {
	t.Parallel()
	var calls int32
	transportErr := errors.New("connection refused")
	op := func(_ context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 2 {
			return transportErr
		}
		return nil
	}
	if err := retry(context.Background(), retryTestBackoffs(), op, infralogger.NewNop()); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if c := atomic.LoadInt32(&calls); c != 2 {
		t.Errorf("calls: got %d, want 2", c)
	}
}
