//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"testing"
)

func TestIndigenousRules_EnglishCore(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
		body  string
	}{
		{"anishinaabe", "Anishinaabe community gathers", ""},
		{"first_nations", "First Nations leaders meet", ""},
		{"metis", "Métis nation celebrates heritage", ""},
		{"inuit", "Inuit hunters adapt to climate change", ""},
		{"treaty_rights", "Treaty rights affirmed by court", ""},
		{"maori", "Māori iwi gather for annual hui", ""},
		{"aboriginal_australian", "Aboriginal Australian elders share stories", ""},
		{"native_hawaiian", "Native Hawaiian sovereignty movement grows", ""},
		{"tribal_sovereignty", "Tribal sovereignty affirmed in ruling", ""},
		{"sami_people", "Sami people protest mining expansion", ""},
		{"tangata_whenua", "Tangata whenua speak at hearing", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			result := classifyIndigenousByRules(tt.title, tt.body)
			if result.relevance != indigenousRelevanceCore {
				t.Errorf("expected core_indigenous for %q, got %s", tt.title, result.relevance)
			}
		})
	}
}

func TestIndigenousRules_SpanishCore(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
	}{
		{"pueblos_indigenas", "Pueblos indígenas exigen derechos"},
		{"territorio_ancestral", "Territorio ancestral bajo amenaza"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			result := classifyIndigenousByRules(tt.title, "")
			if result.relevance != indigenousRelevanceCore {
				t.Errorf("expected core_indigenous for %q, got %s", tt.title, result.relevance)
			}
		})
	}
}

func TestIndigenousRules_FrenchCore(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
	}{
		{"peuples_autochtones", "Les peuples autochtones manifestent"},
		{"premieres_nations", "Les premières nations signent un accord"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			result := classifyIndigenousByRules(tt.title, "")
			if result.relevance != indigenousRelevanceCore {
				t.Errorf("expected core_indigenous for %q, got %s", tt.title, result.relevance)
			}
		})
	}
}

func TestIndigenousRules_PortugueseCore(t *testing.T) {
	t.Helper()

	result := classifyIndigenousByRules("Povos indígenas lutam pela demarcação", "")
	if result.relevance != indigenousRelevanceCore {
		t.Errorf("expected core_indigenous, got %s", result.relevance)
	}
}

func TestIndigenousRules_NordicCore(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
	}{
		{"samefolket", "Samefolket kämpar för rättigheter"},
		{"urfolk", "Urfolk i Norden organiserar"},
		{"sapmi", "Sápmi region faces new challenges"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			result := classifyIndigenousByRules(tt.title, "")
			if result.relevance != indigenousRelevanceCore {
				t.Errorf("expected core_indigenous for %q, got %s", tt.title, result.relevance)
			}
		})
	}
}

func TestIndigenousRules_JapaneseCore(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
	}{
		{"ainu", "アイヌ民族の文化復興運動"},
		{"senjuminzoku", "先住民族の権利に関する宣言"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			result := classifyIndigenousByRules(tt.title, "")
			if result.relevance != indigenousRelevanceCore {
				t.Errorf("expected core_indigenous for %q, got %s", tt.title, result.relevance)
			}
		})
	}
}

func TestIndigenousRules_Peripheral(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
	}{
		{"indigenous_generic", "Indigenous art exhibit opens"},
		{"reconciliation", "Reconciliation efforts continue"},
		{"autochtone", "Autochtone community event"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			result := classifyIndigenousByRules(tt.title, "")
			if result.relevance != indigenousRelevancePeripheral {
				t.Errorf("expected peripheral_indigenous for %q, got %s", tt.title, result.relevance)
			}
		})
	}
}

func TestIndigenousRules_NotIndigenous(t *testing.T) {
	t.Helper()

	result := classifyIndigenousByRules("Weather forecast: sunny skies", "Expected high of 25 degrees.")
	if result.relevance != indigenousRelevanceNot {
		t.Errorf("expected not_indigenous, got %s", result.relevance)
	}
}

func TestIndigenousRules_BodyTruncation(t *testing.T) {
	t.Helper()

	// Core pattern at position > 500 chars in body should be ignored
	longBody := string(make([]byte, indigenousRuleMaxBodyChars+100))
	longBody += " Anishinaabe"
	result := classifyIndigenousByRules("Weather news", longBody)
	if result.relevance != indigenousRelevanceNot {
		t.Errorf("expected not_indigenous when pattern is beyond truncation limit, got %s", result.relevance)
	}
}

func TestIndigenousCategoryTaxonomy(t *testing.T) {
	t.Helper()

	if len(IndigenousCategories) != indigenousCategoryCount {
		t.Errorf("expected %d categories, got %d", indigenousCategoryCount, len(IndigenousCategories))
	}

	expected := map[string]bool{
		"culture": true, "language": true, "land_rights": true,
		"environment": true, "sovereignty": true, "education": true,
		"health": true, "justice": true, "history": true, "community": true,
	}

	for _, cat := range IndigenousCategories {
		if !expected[cat] {
			t.Errorf("unexpected category %q in IndigenousCategories", cat)
		}
	}

	// Verify no duplicates
	seen := make(map[string]bool, indigenousCategoryCount)
	for _, cat := range IndigenousCategories {
		if seen[cat] {
			t.Errorf("duplicate category %q", cat)
		}
		seen[cat] = true
	}
}
