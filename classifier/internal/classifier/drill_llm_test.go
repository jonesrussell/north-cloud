package classifier

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

// mockDrillMLClient implements a mock for testing.
type mockDrillMLClient struct {
	results []domain.DrillResult
	err     error
}

func (m *mockDrillMLClient) Extract(body string) ([]domain.DrillResult, error) {
	return m.results, m.err
}

func TestOrchestrateDrillExtraction_RegexComplete(t *testing.T) {
	body := `DDH-24-001 returned 12.5m @ 3.2 g/t Au`
	mock := &mockDrillMLClient{} // should not be called

	results, method := orchestrateDrillExtraction(body, true, true, mock)

	if method != "regex" {
		t.Errorf("method = %q, want regex", method)
	}
	if len(results) == 0 {
		t.Error("expected results from regex")
	}
}

func TestOrchestrateDrillExtraction_LLMFallback_Partial(t *testing.T) {
	// Body has hole IDs but no parseable grade pattern
	body := `Drill holes DDH-24-001 and DDH-24-002 returned significant results.`
	mock := &mockDrillMLClient{
		results: []domain.DrillResult{
			{HoleID: "DDH-24-001", Commodity: "gold", InterceptM: 12.5, Grade: 3.2, Unit: "g/t"},
		},
	}

	results, method := orchestrateDrillExtraction(body, true, true, mock)

	if method != "hybrid" && method != "llm" {
		t.Errorf("method = %q, want hybrid or llm", method)
	}
	if len(results) == 0 {
		t.Error("expected results from LLM fallback")
	}
}

func TestOrchestrateDrillExtraction_LLMDisabled(t *testing.T) {
	body := `Drill holes DDH-24-001 and DDH-24-002 returned significant results.`
	mock := &mockDrillMLClient{} // should not be called

	results, method := orchestrateDrillExtraction(body, true, false, mock)

	// With LLM disabled and only partial regex, should return partial results or empty
	if method == "llm" || method == "hybrid" {
		t.Errorf("method = %q, should not use LLM when disabled", method)
	}
	_ = results
}

func TestOrchestrateDrillExtraction_NoSignals(t *testing.T) {
	body := `The company announced a new mining project.`

	results, method := orchestrateDrillExtraction(body, false, true, nil)

	if method != "" {
		t.Errorf("method = %q, want empty", method)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}
