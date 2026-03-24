package bootstrap_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/bootstrap"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
)

func TestToInfraFields_Empty(t *testing.T) {
	t.Parallel()

	fields := bootstrap.ToInfraFieldsForTest(nil)
	if len(fields) != 0 {
		t.Errorf("expected 0 fields, got %d", len(fields))
	}
}

func TestToInfraFields_KeyValuePairs(t *testing.T) {
	t.Parallel()

	fields := bootstrap.ToInfraFieldsForTest([]any{
		"key1", "value1",
		"key2", 42,
	})
	if len(fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(fields))
	}
}

func TestToInfraFields_OddCount(t *testing.T) {
	t.Parallel()

	// Odd number of args - last one is dropped
	fields := bootstrap.ToInfraFieldsForTest([]any{
		"key1", "value1",
		"orphan",
	})
	if len(fields) != 1 {
		t.Errorf("expected 1 field (orphan dropped), got %d", len(fields))
	}
}

func TestToInfraFields_NonStringKey(t *testing.T) {
	t.Parallel()

	// Non-string key should be skipped
	fields := bootstrap.ToInfraFieldsForTest([]any{
		42, "value",
		"key", "value",
	})
	if len(fields) != 1 {
		t.Errorf("expected 1 field (non-string key skipped), got %d", len(fields))
	}
}

func TestDiscoveryConfigAdapter_NilConfig(t *testing.T) {
	t.Parallel()

	adapter := bootstrap.NewDiscoveryConfigAdapter(nil)

	if al := adapter.Allowlist(); al != nil {
		t.Errorf("expected nil Allowlist, got %v", al)
	}
	if bl := adapter.Blocklist(); bl != nil {
		t.Errorf("expected nil Blocklist, got %v", bl)
	}
	if maxCandidates := adapter.MaxNewCandidatesPerRun(); maxCandidates != 0 {
		t.Errorf("expected 0 MaxNewCandidatesPerRun, got %d", maxCandidates)
	}
	if budget := adapter.GlobalCrawlBudgetPerDay(); budget != 0 {
		t.Errorf("expected 0 GlobalCrawlBudgetPerDay, got %d", budget)
	}
}

func TestDiscoveryConfigAdapter_WithConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.DiscoveryConfig{
		Allowlist:               []string{"example.com", "news.org"},
		Blocklist:               []string{"bad.com"},
		MaxNewCandidatesPerRun:  50,
		GlobalCrawlBudgetPerDay: 100,
	}
	adapter := bootstrap.NewDiscoveryConfigAdapter(cfg)

	if al := adapter.Allowlist(); len(al) != 2 {
		t.Errorf("expected 2 allowlist entries, got %d", len(al))
	}
	if bl := adapter.Blocklist(); len(bl) != 1 {
		t.Errorf("expected 1 blocklist entry, got %d", len(bl))
	}
	if maxCandidates := adapter.MaxNewCandidatesPerRun(); maxCandidates != 50 {
		t.Errorf("expected MaxNewCandidatesPerRun 50, got %d", maxCandidates)
	}
	if budget := adapter.GlobalCrawlBudgetPerDay(); budget != 100 {
		t.Errorf("expected GlobalCrawlBudgetPerDay 100, got %d", budget)
	}
}
