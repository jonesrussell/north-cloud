package insights_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/insights"
)

func TestBuildDocument_Fields(t *testing.T) {
	t.Helper()
	ins := category.Insight{
		Category:         "classifier",
		Severity:         "medium",
		Summary:          "Test insight",
		Details:          map[string]any{"domain": "example.com"},
		SuggestedActions: []string{"Do something"},
		TokensUsed:       100,
		Model:            "claude-haiku-4-5-20251001",
	}
	version := "0.1.0"
	now := time.Now().UTC()

	doc := insights.BuildDocument(ins, version, now)

	if doc["category"] != "classifier" {
		t.Errorf("expected category 'classifier', got %v", doc["category"])
	}
	if doc["severity"] != "medium" {
		t.Errorf("expected severity 'medium', got %v", doc["severity"])
	}
	if doc["observer_version"] != version {
		t.Errorf("expected observer_version %q, got %v", version, doc["observer_version"])
	}
	if doc["tokens_used"] != 100 {
		t.Errorf("expected tokens_used 100, got %v", doc["tokens_used"])
	}
}
