package severity_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/severity"
)

// testTable returns a shared lookup table used across multiple test cases.
func testTable() severity.Table {
	return severity.NewTable(map[string]domain.Severity{
		"fentanyl":    domain.SeverityHigh,
		"carfentanil": domain.SeverityCritical,
		"cocaine":     domain.SeverityLow,
	})
}

func TestInfer_HighestWins(t *testing.T) {
	t.Helper()

	hazard := domain.Hazard{
		HarmReduction: &domain.HarmReductionHazard{
			HazardType:  domain.HazardOpioidSupply,
			Substances:  []string{"fentanyl", "carfentanil"},
			Composition: nil,
		},
	}

	got := severity.Infer(hazard, testTable())
	if got != domain.SeverityCritical {
		t.Errorf("expected critical, got %q", got)
	}
}

func TestInfer_UnknownSubstance(t *testing.T) {
	t.Helper()

	hazard := domain.Hazard{
		HarmReduction: &domain.HarmReductionHazard{
			HazardType: domain.HazardOpioidSupply,
			Substances: []string{"mystery-compound"},
		},
	}

	got := severity.Infer(hazard, testTable())
	if got != severity.FloorSeverity {
		t.Errorf("expected FloorSeverity (%q), got %q", severity.FloorSeverity, got)
	}
}

func TestInfer_EmptyHazard(t *testing.T) {
	t.Helper()

	hazard := domain.Hazard{HarmReduction: nil}

	got := severity.Infer(hazard, testTable())
	if got != severity.FloorSeverity {
		t.Errorf("expected FloorSeverity (%q), got %q", severity.FloorSeverity, got)
	}
}

func TestInfer_CompositionAlsoChecked(t *testing.T) {
	t.Helper()

	hazard := domain.Hazard{
		HarmReduction: &domain.HarmReductionHazard{
			HazardType: domain.HazardOpioidSupply,
			Substances: nil,
			Composition: []domain.Substance{
				{Name: "carfentanil", Percentage: 5.0, IsActiveIngredient: true},
			},
		},
	}

	got := severity.Infer(hazard, testTable())
	if got != domain.SeverityCritical {
		t.Errorf("expected critical from composition, got %q", got)
	}
}

func TestInfer_CaseInsensitive(t *testing.T) {
	t.Helper()

	hazard := domain.Hazard{
		HarmReduction: &domain.HarmReductionHazard{
			HazardType: domain.HazardOpioidSupply,
			Substances: []string{"Fentanyl"},
		},
	}

	got := severity.Infer(hazard, testTable())
	if got != domain.SeverityHigh {
		t.Errorf("expected high (case-insensitive match), got %q", got)
	}
}

func TestInfer_TrimsWhitespace(t *testing.T) {
	t.Helper()

	hazard := domain.Hazard{
		HarmReduction: &domain.HarmReductionHazard{
			HazardType: domain.HazardOpioidSupply,
			Substances: []string{"  carfentanil  "},
		},
	}

	got := severity.Infer(hazard, testTable())
	if got != domain.SeverityCritical {
		t.Errorf("expected critical (whitespace-trimmed match), got %q", got)
	}
}
