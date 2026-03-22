package drift_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	driftcat "github.com/jonesrussell/north-cloud/ai-observer/internal/category/drift"
)

func TestDriftCategory_ImplementsInterface(t *testing.T) {
	t.Helper()
	var _ category.Category = (*driftcat.Category)(nil)
}

func TestDriftCategory_Name(t *testing.T) {
	t.Helper()
	c := driftcat.New(nil, driftcat.Config{}, nil)
	if got := c.Name(); got != "drift" {
		t.Errorf("expected name 'drift', got %q", got)
	}
}

func TestDriftCategory_MaxEventsPerRun(t *testing.T) {
	t.Helper()
	c := driftcat.New(nil, driftcat.Config{}, nil)
	if got := c.MaxEventsPerRun(); got != 0 {
		t.Errorf("expected MaxEventsPerRun 0, got %d", got)
	}
}

func TestDriftCategory_ModelTier(t *testing.T) {
	t.Helper()
	c := driftcat.New(nil, driftcat.Config{}, nil)
	if got := c.ModelTier(); got != "haiku" {
		t.Errorf("expected model tier 'haiku', got %q", got)
	}
}
