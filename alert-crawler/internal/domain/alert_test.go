package domain_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

// fixtureAlert builds a fully-populated, schema-valid Alert for tests.
func fixtureAlert(t *testing.T) domain.Alert {
	t.Helper()

	issuedAt := time.Date(2026, 5, 5, 14, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, 6, 4, 14, 0, 0, 0, time.UTC)
	confirmDate := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	crawledAt := time.Date(2026, 5, 5, 14, 5, 0, 0, time.UTC)

	return domain.Alert{
		ID:             "safersites:20260505fentanyl",
		Category:       domain.CategoryHarmReduction,
		Severity:       domain.SeverityHigh,
		Scope:          []string{"canada:manitoba", "canada:manitoba:winnipeg"},
		IssuedAt:       issuedAt,
		ExpiresAt:      &expiresAt,
		LifecycleState: domain.LifecycleActive,
		Title:          "Fentanyl detected in Winnipeg opioid supply — May 5 2026",
		Summary: "Safer Sites Winnipeg drug-checking confirmed fentanyl in heroin. " +
			"Potency above baseline. Do not use alone.",
		Hazard: domain.Hazard{
			HarmReduction: &domain.HarmReductionHazard{
				HazardType: domain.HazardOpioidSupply,
				Substances: []string{"fentanyl", "heroin"},
				Composition: []domain.Substance{
					{Name: "fentanyl", Percentage: 12.5, IsActiveIngredient: true},
					{Name: "heroin", Percentage: 87.5, IsActiveIngredient: false},
				},
				VisualDescription: "White powder, fine grain",
				LabSource:         "Safer Sites Winnipeg drug-checking",
				ConfirmationDate:  &confirmDate,
			},
		},
		Guidance: []string{
			"Do not use alone",
			"Have naloxone on hand",
			"Start low, go slow",
			"Call 911 if someone overdoses",
		},
		Sources: []domain.SourceAttribution{
			{
				SourceID:        "safersites",
				SourceName:      "Safer Sites Winnipeg",
				URL:             "https://safersiteswinnipeg.ca/alerts/20260505-fentanyl",
				AttributionText: "Drug-checking result published by Safer Sites Winnipeg harm reduction program.",
			},
		},
		RevisionHistory: []domain.Revision{
			{
				RevisionAt:    issuedAt,
				RevisionKind:  "created",
				ChangeSummary: "Initial alert published",
			},
		},
		ParseQuality:  domain.ParseClean,
		CrawledAt:     crawledAt,
		LastUpdatedAt: crawledAt,
	}
}

func TestAlert_RoundTrip(t *testing.T) {
	t.Parallel()

	original := fixtureAlert(t)

	data, marshalErr := json.Marshal(original)
	if marshalErr != nil {
		t.Fatalf("marshal failed: %v", marshalErr)
	}

	var decoded domain.Alert
	if unmarshalErr := json.Unmarshal(data, &decoded); unmarshalErr != nil {
		t.Fatalf("unmarshal failed: %v", unmarshalErr)
	}

	// Key field checks (deep-equal on time.Time can be fragile due to monotonic clock).
	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: got %q, want %q", decoded.ID, original.ID)
	}

	if decoded.Category != original.Category {
		t.Errorf("Category mismatch: got %q, want %q", decoded.Category, original.Category)
	}

	if decoded.Severity != original.Severity {
		t.Errorf("Severity mismatch: got %q, want %q", decoded.Severity, original.Severity)
	}

	if len(decoded.Scope) != len(original.Scope) {
		t.Errorf("Scope length mismatch: got %d, want %d", len(decoded.Scope), len(original.Scope))
	}

	if decoded.LifecycleState != original.LifecycleState {
		t.Errorf("LifecycleState mismatch: got %q, want %q", decoded.LifecycleState, original.LifecycleState)
	}

	if decoded.ParseQuality != original.ParseQuality {
		t.Errorf("ParseQuality mismatch: got %q, want %q", decoded.ParseQuality, original.ParseQuality)
	}

	if decoded.Title != original.Title {
		t.Errorf("Title mismatch: got %q, want %q", decoded.Title, original.Title)
	}

	if decoded.Hazard.HarmReduction == nil {
		t.Fatal("decoded Hazard.HarmReduction is nil")
	}

	if decoded.Hazard.HarmReduction.HazardType != original.Hazard.HarmReduction.HazardType {
		t.Errorf("HazardType mismatch: got %q, want %q",
			decoded.Hazard.HarmReduction.HazardType, original.Hazard.HarmReduction.HazardType)
	}

	if decoded.ExpiresAt == nil {
		t.Fatal("decoded ExpiresAt is nil")
	}

	if !decoded.ExpiresAt.Equal(*original.ExpiresAt) {
		t.Errorf("ExpiresAt mismatch: got %v, want %v", decoded.ExpiresAt, original.ExpiresAt)
	}
}

func TestAlert_Validate_Valid(t *testing.T) {
	t.Parallel()

	a := fixtureAlert(t)

	if validateErr := a.Validate(); validateErr != nil {
		t.Errorf("expected valid alert to pass Validate(), got: %v", validateErr)
	}
}

func TestAlert_Validate_EmptyID(t *testing.T) {
	t.Parallel()

	a := fixtureAlert(t)
	a.ID = ""

	if validateErr := a.Validate(); validateErr == nil {
		t.Error("expected error for empty ID, got nil")
	}
}

func TestAlert_Validate_InvalidCategory(t *testing.T) {
	t.Parallel()

	a := fixtureAlert(t)
	a.Category = "unknown"

	if validateErr := a.Validate(); validateErr == nil {
		t.Error("expected error for invalid category, got nil")
	}
}

func TestAlert_Validate_InvalidSeverity(t *testing.T) {
	t.Parallel()

	a := fixtureAlert(t)
	a.Severity = "extreme"

	if validateErr := a.Validate(); validateErr == nil {
		t.Error("expected error for invalid severity, got nil")
	}
}

func TestAlert_Validate_EmptyScope(t *testing.T) {
	t.Parallel()

	a := fixtureAlert(t)
	a.Scope = nil

	if validateErr := a.Validate(); validateErr == nil {
		t.Error("expected error for empty scope, got nil")
	}
}

func TestAlert_Validate_ZeroIssuedAt(t *testing.T) {
	t.Parallel()

	a := fixtureAlert(t)
	a.IssuedAt = time.Time{}

	if validateErr := a.Validate(); validateErr == nil {
		t.Error("expected error for zero issued_at, got nil")
	}
}

func TestAlert_Validate_InvalidLifecycleState(t *testing.T) {
	t.Parallel()

	a := fixtureAlert(t)
	a.LifecycleState = "pending"

	if validateErr := a.Validate(); validateErr == nil {
		t.Error("expected error for invalid lifecycle_state, got nil")
	}
}

func TestAlert_Validate_EmptyTitle(t *testing.T) {
	t.Parallel()

	a := fixtureAlert(t)
	a.Title = ""

	if validateErr := a.Validate(); validateErr == nil {
		t.Error("expected error for empty title, got nil")
	}
}

func TestAlert_Validate_EmptySummary(t *testing.T) {
	t.Parallel()

	a := fixtureAlert(t)
	a.Summary = ""

	if validateErr := a.Validate(); validateErr == nil {
		t.Error("expected error for empty summary, got nil")
	}
}

func TestAlert_Validate_NoSources(t *testing.T) {
	t.Parallel()

	a := fixtureAlert(t)
	a.Sources = nil

	if validateErr := a.Validate(); validateErr == nil {
		t.Error("expected error for no sources, got nil")
	}
}

func TestAlert_Validate_InvalidParseQuality(t *testing.T) {
	t.Parallel()

	a := fixtureAlert(t)
	a.ParseQuality = "unknown"

	if validateErr := a.Validate(); validateErr == nil {
		t.Error("expected error for invalid parse_quality, got nil")
	}
}

func TestAlert_Validate_EmptyAlert(t *testing.T) {
	t.Parallel()

	a := domain.Alert{}

	if validateErr := a.Validate(); validateErr == nil {
		t.Error("expected errors for empty alert, got nil")
	}
}

func TestSeverityEnumCoverage(t *testing.T) {
	t.Parallel()

	severities := []domain.Severity{
		domain.SeverityInfo,
		domain.SeverityLow,
		domain.SeverityMedium,
		domain.SeverityHigh,
		domain.SeverityCritical,
	}

	for _, s := range severities {
		if s == "" {
			t.Errorf("severity constant is empty string")
		}
	}
}

func TestCategoryEnumCoverage(t *testing.T) {
	t.Parallel()

	categories := []domain.Category{
		domain.CategoryHarmReduction,
	}

	for _, c := range categories {
		if c == "" {
			t.Errorf("category constant is empty string")
		}
	}
}

func TestLifecycleStateEnumCoverage(t *testing.T) {
	t.Parallel()

	states := []domain.LifecycleState{
		domain.LifecycleActive,
		domain.LifecycleRescinded,
	}

	for _, s := range states {
		if s == "" {
			t.Errorf("lifecycle state constant is empty string")
		}
	}
}

func TestParseQualityEnumCoverage(t *testing.T) {
	t.Parallel()

	qualities := []domain.ParseQuality{
		domain.ParseClean,
		domain.ParseDegraded,
		domain.ParseFailed,
	}

	for _, q := range qualities {
		if q == "" {
			t.Errorf("parse quality constant is empty string")
		}
	}
}

func TestAlert_GoldenFile(t *testing.T) {
	t.Parallel()

	goldenData, readErr := os.ReadFile("testdata/golden_alert.json")
	if readErr != nil {
		t.Fatalf("read golden file: %v", readErr)
	}

	// Unmarshal golden into Alert, then re-marshal, then compare as JSON maps.
	var goldenAlert domain.Alert
	if unmarshalErr := json.Unmarshal(goldenData, &goldenAlert); unmarshalErr != nil {
		t.Fatalf("unmarshal golden: %v", unmarshalErr)
	}

	reMarshaled, marshalErr := json.Marshal(goldenAlert)
	if marshalErr != nil {
		t.Fatalf("re-marshal golden alert: %v", marshalErr)
	}

	// Compare as generic maps to avoid key-ordering sensitivity.
	var goldenMap, reMap map[string]any
	if jsonErr := json.Unmarshal(goldenData, &goldenMap); jsonErr != nil {
		t.Fatalf("parse golden as map: %v", jsonErr)
	}

	if jsonErr := json.Unmarshal(reMarshaled, &reMap); jsonErr != nil {
		t.Fatalf("parse re-marshaled as map: %v", jsonErr)
	}

	// Spot-check key fields from the golden.
	if goldenMap["id"] != reMap["id"] {
		t.Errorf("id mismatch: golden=%v re=%v", goldenMap["id"], reMap["id"])
	}

	if goldenMap["category"] != reMap["category"] {
		t.Errorf("category mismatch: golden=%v re=%v", goldenMap["category"], reMap["category"])
	}

	if goldenMap["severity"] != reMap["severity"] {
		t.Errorf("severity mismatch: golden=%v re=%v", goldenMap["severity"], reMap["severity"])
	}

	if goldenMap["lifecycle_state"] != reMap["lifecycle_state"] {
		t.Errorf("lifecycle_state mismatch: golden=%v re=%v", goldenMap["lifecycle_state"], reMap["lifecycle_state"])
	}

	if goldenMap["parse_quality"] != reMap["parse_quality"] {
		t.Errorf("parse_quality mismatch: golden=%v re=%v", goldenMap["parse_quality"], reMap["parse_quality"])
	}
}
