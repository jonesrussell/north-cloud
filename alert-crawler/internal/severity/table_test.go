package severity_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/severity"
)

func TestNewTable_LowercasesKeys(t *testing.T) {
	t.Helper()

	raw := map[string]domain.Severity{
		"Fentanyl":    domain.SeverityHigh,
		"CARFENTANIL": domain.SeverityCritical,
	}
	tbl := severity.NewTable(raw)

	if _, ok := tbl.Lookup("fentanyl"); !ok {
		t.Error("expected lowercase key 'fentanyl' to be found")
	}

	if _, ok := tbl.Lookup("carfentanil"); !ok {
		t.Error("expected lowercase key 'carfentanil' to be found")
	}
}

func TestLookup_NotFound(t *testing.T) {
	t.Helper()

	tbl := severity.NewTable(map[string]domain.Severity{})

	_, ok := tbl.Lookup("unknown-substance")
	if ok {
		t.Error("expected not-found for unknown substance")
	}
}

func TestLookup_TrimsAndLowercases(t *testing.T) {
	t.Helper()

	tbl := severity.NewTable(map[string]domain.Severity{
		"fentanyl": domain.SeverityHigh,
	})

	s, ok := tbl.Lookup("  Fentanyl  ")
	if !ok {
		t.Fatal("expected to find 'fentanyl' with trimmed+lowercased input")
	}

	if s != domain.SeverityHigh {
		t.Errorf("expected SeverityHigh, got %q", s)
	}
}
