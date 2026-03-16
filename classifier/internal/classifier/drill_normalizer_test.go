package classifier

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

func TestNormalizeCommodity(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Au", "gold"},
		{"au", "gold"},
		{"Ag", "silver"},
		{"Cu", "copper"},
		{"Ni", "nickel"},
		{"Zn", "zinc"},
		{"Li", "lithium"},
		{"U3O8", "uranium"},
		{"Pb", "lead"},
		{"gold", "gold"},
		{"Gold", "gold"},
		{"COPPER", "copper"},
		{"unknown", "unknown"},
		{"", ""},
	}
	for _, tt := range tests {
		got := normalizeCommodity(tt.input)
		if got != tt.want {
			t.Errorf("normalizeCommodity(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeUnit(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"g/t", "g/t"},
		{"gpt", "g/t"},
		{"g per tonne", "g/t"},
		{"grams per tonne", "g/t"},
		{"%", "%"},
		{"percent", "%"},
		{"ppm", "ppm"},
		{"parts per million", "ppm"},
		{"oz/t", "oz/t"},
	}
	for _, tt := range tests {
		got := normalizeUnit(tt.input)
		if got != tt.want {
			t.Errorf("normalizeUnit(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeDrillResults_Dedup(t *testing.T) {
	input := []domain.DrillResult{
		{HoleID: "DDH-24-001", Commodity: "Au", InterceptM: 12.5, Grade: 3.2, Unit: "g/t"},
		{HoleID: "DDH-24-001", Commodity: "Au", InterceptM: 12.5, Grade: 3.2, Unit: "g/t"}, // duplicate
		{HoleID: "DDH-24-002", Commodity: "Cu", InterceptM: 8.0, Grade: 1.5, Unit: "%"},
	}
	got := normalizeDrillResults(input)

	if len(got) != 2 {
		t.Errorf("got %d results after dedup, want 2", len(got))
	}
}

func TestNormalizeDrillResults_DropsInvalid(t *testing.T) {
	input := []domain.DrillResult{
		{HoleID: "DDH-24-001", Commodity: "Au", InterceptM: 12.5, Grade: 3.2, Unit: "g/t"}, // valid
		{HoleID: "", Commodity: "Au", InterceptM: 0, Grade: 0, Unit: "g/t"},                  // invalid: no hole_id AND no intercept AND no grade
	}
	got := normalizeDrillResults(input)

	if len(got) != 1 {
		t.Errorf("got %d results, want 1 (invalid should be dropped)", len(got))
	}
}

func TestNormalizeDrillResults_NormalizesCommodityAndUnit(t *testing.T) {
	input := []domain.DrillResult{
		{HoleID: "DDH-24-001", Commodity: "Au", InterceptM: 12.5, Grade: 3.2, Unit: "gpt"},
	}
	got := normalizeDrillResults(input)

	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}
	if got[0].Commodity != "gold" {
		t.Errorf("Commodity = %q, want gold", got[0].Commodity)
	}
	if got[0].Unit != "g/t" {
		t.Errorf("Unit = %q, want g/t", got[0].Unit)
	}
	if got[0].HoleID != "DDH-24-001" {
		t.Errorf("HoleID = %q, want DDH-24-001", got[0].HoleID)
	}
}
