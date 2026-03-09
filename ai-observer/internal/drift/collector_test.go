package drift_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/drift"
)

func TestCollector_New(t *testing.T) {
	t.Helper()
	c := drift.NewCollector(nil)
	if c == nil {
		t.Error("expected non-nil collector")
	}
}
