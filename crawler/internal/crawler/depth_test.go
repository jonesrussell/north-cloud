package crawler

import (
	"testing"
)

func TestCollyMaxDepth_UnlimitedSentinel(t *testing.T) {
	t.Helper()

	// -1 is the "unlimited" sentinel — must pass 0 to Colly
	got := collyMaxDepth(-1)
	if got != 0 {
		t.Errorf("collyMaxDepth(-1) = %d, want 0 (Colly unlimited)", got)
	}
}

func TestCollyMaxDepth_ZeroUsesDefault(t *testing.T) {
	t.Helper()

	// 0 means "unset" — must use defaultMaxDepth
	got := collyMaxDepth(0)
	if got != defaultMaxDepth {
		t.Errorf("collyMaxDepth(0) = %d, want %d (default)", got, defaultMaxDepth)
	}
}

func TestCollyMaxDepth_PositivePassedThrough(t *testing.T) {
	t.Helper()

	got := collyMaxDepth(5)
	if got != 5 {
		t.Errorf("collyMaxDepth(5) = %d, want 5", got)
	}
}
