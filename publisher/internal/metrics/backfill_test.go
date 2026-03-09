package metrics_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/publisher/internal/metrics"
)

func TestBackfillKeyConstants(t *testing.T) {
	t.Helper()

	tests := []struct {
		name string
		key  string
		want string
	}{
		{"total", metrics.KeyBackfillTotal, "metrics:indigenous_backfill_total"},
		{"success", metrics.KeyBackfillSuccess, "metrics:indigenous_backfill_success"},
		{"failed", metrics.KeyBackfillFailed, "metrics:indigenous_backfill_failed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()
			if tc.key != tc.want {
				t.Errorf("key %q: got %q, want %q", tc.name, tc.key, tc.want)
			}
		})
	}
}
