package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

func fixtureHazard(t *testing.T) domain.Hazard {
	t.Helper()

	confirmDate := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)

	return domain.Hazard{
		HarmReduction: &domain.HarmReductionHazard{
			HazardType: domain.HazardOpioidSupply,
			Substances: []string{"fentanyl", "heroin"},
			Composition: []domain.Substance{
				{Name: "fentanyl", Percentage: 12.5, IsActiveIngredient: true},
				{Name: "heroin", Percentage: 87.5, IsActiveIngredient: false, Note: "base substance"},
			},
			VisualDescription: "White powder, fine grain",
			LabSource:         "Safer Sites Winnipeg drug-checking",
			ConfirmationDate:  &confirmDate,
		},
	}
}

func TestHazard_RoundTrip(t *testing.T) {
	t.Parallel()

	original := fixtureHazard(t)

	data, marshalErr := json.Marshal(original)
	if marshalErr != nil {
		t.Fatalf("marshal failed: %v", marshalErr)
	}

	var decoded domain.Hazard
	if unmarshalErr := json.Unmarshal(data, &decoded); unmarshalErr != nil {
		t.Fatalf("unmarshal failed: %v", unmarshalErr)
	}

	if decoded.HarmReduction == nil {
		t.Fatal("decoded HarmReduction is nil after round-trip")
	}

	if decoded.HarmReduction.HazardType != original.HarmReduction.HazardType {
		t.Errorf("HazardType mismatch: got %q, want %q",
			decoded.HarmReduction.HazardType, original.HarmReduction.HazardType)
	}

	if len(decoded.HarmReduction.Substances) != len(original.HarmReduction.Substances) {
		t.Errorf("Substances length mismatch: got %d, want %d",
			len(decoded.HarmReduction.Substances), len(original.HarmReduction.Substances))
	}

	if len(decoded.HarmReduction.Composition) != len(original.HarmReduction.Composition) {
		t.Errorf("Composition length mismatch: got %d, want %d",
			len(decoded.HarmReduction.Composition), len(original.HarmReduction.Composition))
	}

	if decoded.HarmReduction.VisualDescription != original.HarmReduction.VisualDescription {
		t.Errorf("VisualDescription mismatch: got %q, want %q",
			decoded.HarmReduction.VisualDescription, original.HarmReduction.VisualDescription)
	}

	if decoded.HarmReduction.LabSource != original.HarmReduction.LabSource {
		t.Errorf("LabSource mismatch: got %q, want %q",
			decoded.HarmReduction.LabSource, original.HarmReduction.LabSource)
	}
}

func TestHazard_JSONShape_NoNestingKey(t *testing.T) {
	t.Parallel()

	h := fixtureHazard(t)

	data, marshalErr := json.Marshal(h)
	if marshalErr != nil {
		t.Fatalf("marshal failed: %v", marshalErr)
	}

	// The JSON must not contain a wrapping "harm_reduction" key.
	// Fields must appear at the top level of the hazard object.
	var raw map[string]any
	if jsonErr := json.Unmarshal(data, &raw); jsonErr != nil {
		t.Fatalf("parse JSON: %v", jsonErr)
	}

	if _, ok := raw["harm_reduction"]; ok {
		t.Error("marshaled hazard must not have a 'harm_reduction' nesting key")
	}

	if _, ok := raw["hazard_type"]; !ok {
		t.Error("marshaled hazard must have 'hazard_type' at root level")
	}

	if _, ok := raw["substances"]; !ok {
		t.Error("marshaled hazard must have 'substances' at root level")
	}
}

func TestHazard_Marshal_NilHarmReduction(t *testing.T) {
	t.Parallel()

	h := domain.Hazard{HarmReduction: nil}

	_, marshalErr := json.Marshal(h)
	if marshalErr == nil {
		t.Error("expected error marshaling hazard with nil HarmReduction, got nil")
	}
}

func TestHazard_Validate_Nil(t *testing.T) {
	t.Parallel()

	h := domain.Hazard{HarmReduction: nil}

	if validateErr := h.Validate(); validateErr == nil {
		t.Error("expected error for nil HarmReduction, got nil")
	}
}

func TestHazard_Validate_Valid(t *testing.T) {
	t.Parallel()

	h := fixtureHazard(t)

	if validateErr := h.Validate(); validateErr != nil {
		t.Errorf("expected valid hazard to pass Validate(), got: %v", validateErr)
	}
}

func TestHazardTypeEnumCoverage(t *testing.T) {
	t.Parallel()

	types := []domain.HazardType{
		domain.HazardOpioidSupply,
		domain.HazardStimulantSupply,
		domain.HazardBenzoSupply,
		domain.HazardOther,
	}

	for _, ht := range types {
		if ht == "" {
			t.Errorf("hazard type constant is empty string")
		}
	}
}

func TestSubstance_Fields(t *testing.T) {
	t.Parallel()

	s := domain.Substance{
		Name:               "fentanyl",
		Percentage:         12.5,
		IsActiveIngredient: true,
		Note:               "detected by immunoassay",
	}

	data, marshalErr := json.Marshal(s)
	if marshalErr != nil {
		t.Fatalf("marshal Substance: %v", marshalErr)
	}

	var decoded domain.Substance
	if unmarshalErr := json.Unmarshal(data, &decoded); unmarshalErr != nil {
		t.Fatalf("unmarshal Substance: %v", unmarshalErr)
	}

	if decoded.Name != s.Name {
		t.Errorf("Name mismatch: got %q, want %q", decoded.Name, s.Name)
	}

	if decoded.Percentage != s.Percentage {
		t.Errorf("Percentage mismatch: got %v, want %v", decoded.Percentage, s.Percentage)
	}

	if decoded.IsActiveIngredient != s.IsActiveIngredient {
		t.Errorf("IsActiveIngredient mismatch: got %v, want %v", decoded.IsActiveIngredient, s.IsActiveIngredient)
	}

	if decoded.Note != s.Note {
		t.Errorf("Note mismatch: got %q, want %q", decoded.Note, s.Note)
	}
}
