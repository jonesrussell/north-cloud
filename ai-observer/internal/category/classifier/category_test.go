package classifier_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	classifiercategory "github.com/jonesrussell/north-cloud/ai-observer/internal/category/classifier"
)

func TestClassifierCategory_ImplementsInterface(t *testing.T) {
	t.Helper()
	var _ category.Category = &classifiercategory.Category{}
}

func TestClassifierCategory_Name(t *testing.T) {
	t.Helper()
	c := classifiercategory.New(nil, 200, "claude-haiku-4-5-20251001")
	if c.Name() != "classifier" {
		t.Errorf("expected name 'classifier', got %q", c.Name())
	}
}

func TestClassifierCategory_MaxEventsPerRun(t *testing.T) {
	t.Helper()
	const maxEvents = 150
	c := classifiercategory.New(nil, maxEvents, "claude-haiku-4-5-20251001")
	if c.MaxEventsPerRun() != maxEvents {
		t.Errorf("expected %d, got %d", maxEvents, c.MaxEventsPerRun())
	}
}
