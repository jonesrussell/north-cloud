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
		nil,
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
