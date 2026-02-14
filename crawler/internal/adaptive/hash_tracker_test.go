package adaptive_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/adaptive"
)

func TestComputeHash(t *testing.T) {
	t.Parallel()

	hash := adaptive.ComputeHash(
		[]byte("<html><body>Hello World</body></html>"),
	)
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}

	// Same input should produce same hash.
	hash2 := adaptive.ComputeHash(
		[]byte("<html><body>Hello World</body></html>"),
	)
	if hash != hash2 {
		t.Fatalf(
			"expected same hash for same input: %s != %s", hash, hash2,
		)
	}

	// Different input should produce different hash.
	hash3 := adaptive.ComputeHash(
		[]byte("<html><body>Different</body></html>"),
	)
	if hash == hash3 {
		t.Fatal("expected different hash for different input")
	}
}

func TestComputeHash_EmptyInput(t *testing.T) {
	t.Parallel()

	hash := adaptive.ComputeHash([]byte{})
	if hash == "" {
		t.Fatal("expected non-empty hash even for empty input")
	}

	// SHA-256 of empty input is well-known.
	const expectedEmptySHA256 = "e3b0c44298fc1c149afbf4c8996fb924" +
		"27ae41e4649b934ca495991b7852b855"
	if hash != expectedEmptySHA256 {
		t.Fatalf(
			"expected empty SHA-256 %s, got %s",
			expectedEmptySHA256, hash,
		)
	}
}

func TestCalculateAdaptiveInterval(t *testing.T) {
	t.Parallel()

	const (
		baselineMinutes = 30
		oneHourMinutes  = 60
		twoHours        = 2
		fourHours       = 4
		highUnchanged   = 7
	)

	baseline := baselineMinutes * time.Minute
	maxInterval := adaptive.MaxAdaptiveInterval

	tests := []struct {
		name           string
		unchangedCount int
		expected       time.Duration
	}{
		{"changed (0 unchanged)", 0, baseline},
		{"1 unchanged", 1, oneHourMinutes * time.Minute},
		{"2 unchanged", twoHours, twoHours * time.Hour},
		{"3 unchanged", twoHours + 1, fourHours * time.Hour},
		{"7+ unchanged caps at max", highUnchanged, maxInterval},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := adaptive.CalculateAdaptiveInterval(
				baseline, maxInterval, tt.unchangedCount,
			)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCalculateAdaptiveInterval_NegativeCount(t *testing.T) {
	t.Parallel()

	const baselineMinutes = 30

	baseline := baselineMinutes * time.Minute
	maxInterval := adaptive.MaxAdaptiveInterval

	result := adaptive.CalculateAdaptiveInterval(baseline, maxInterval, -1)
	if result != baseline {
		t.Errorf(
			"expected baseline %v for negative count, got %v",
			baseline, result,
		)
	}
}

func TestCalculateAdaptiveInterval_ZeroBaseline(t *testing.T) {
	t.Parallel()

	const unchangedCount = 3

	result := adaptive.CalculateAdaptiveInterval(
		0, adaptive.MaxAdaptiveInterval, unchangedCount,
	)
	if result != 0 {
		t.Errorf("expected 0 for zero baseline, got %v", result)
	}
}
