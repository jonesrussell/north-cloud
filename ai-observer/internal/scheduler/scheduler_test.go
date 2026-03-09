package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/scheduler"
)

func TestBudget_Deduct(t *testing.T) {
	t.Helper()
	b := scheduler.NewBudget(1000)

	ok := b.Deduct(500)
	if !ok {
		t.Error("expected first deduction to succeed")
	}

	ok = b.Deduct(600)
	if ok {
		t.Error("expected second deduction to fail (would exceed budget)")
	}
}

func TestBudget_Reset(t *testing.T) {
	t.Helper()
	b := scheduler.NewBudget(1000)
	b.Deduct(1000)

	b.Reset()

	ok := b.Deduct(500)
	if !ok {
		t.Error("expected deduction to succeed after reset")
	}
}

func TestScheduler_RunOnce_NoCategories(t *testing.T) {
	t.Helper()
	s := scheduler.New(
		nil, // fast categories
		nil, // slow categories
		nil,
		nil,
		scheduler.Config{
			IntervalSeconds:      1,
			MaxTokensPerInterval: 0,
			WindowDuration:       time.Hour,
			DryRun:               false,
		},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Should return without panic when there are no categories
	s.RunOnce(ctx)
}

func TestScheduler_DualTicker_Config(t *testing.T) {
	t.Helper()

	const (
		fastInterval = 30
		slowInterval = 21600
		tokenBudget  = 1000
		driftHours   = 6
	)

	cfg := scheduler.Config{
		IntervalSeconds:      fastInterval,
		MaxTokensPerInterval: tokenBudget,
		WindowDuration:       time.Hour,
		DryRun:               false,
		DriftIntervalSeconds: slowInterval,
		DriftWindowDuration:  driftHours * time.Hour,
	}
	s := scheduler.New(nil, nil, nil, nil, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	s.RunOnce(ctx)
}

func TestScheduler_RunDrift_NoCategories(t *testing.T) {
	t.Helper()
	s := scheduler.New(
		nil, // fast categories
		nil, // slow categories
		nil,
		nil,
		scheduler.Config{
			IntervalSeconds:      1,
			MaxTokensPerInterval: 0,
			WindowDuration:       time.Hour,
			DryRun:               false,
		},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Should return without panic when there are no slow categories
	s.RunDrift(ctx)
}
