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

func TestIndigenousRules_ConfidenceScoring(t *testing.T) {
	t.Helper()

	t.Run("core_confidence_above_base", func(t *testing.T) {
		t.Helper()
		result := classifyIndigenousByRules("Inuit hunters report changes", "")
		if result.confidence < indigenousConfidenceCoreBase {
			t.Errorf("expected confidence >= %f, got %f", indigenousConfidenceCoreBase, result.confidence)
		}
	})

	t.Run("multiple_core_hits_higher", func(t *testing.T) {
		t.Helper()
		single := classifyIndigenousByRules("First Nations leaders discuss issues", "")
		multi := classifyIndigenousByRules("First Nations and Métis leaders discuss treaty rights", "")
		if multi.confidence < single.confidence {
			t.Errorf("expected multi-hit confidence %f >= single %f", multi.confidence, single.confidence)
		}
	})

	t.Run("peripheral_lower_than_core", func(t *testing.T) {
		t.Helper()
		core := classifyIndigenousByRules("Anishinaabe community celebrates culture", "")
		periph := classifyIndigenousByRules("Indigenous art exhibit opens", "")
		if periph.confidence >= core.confidence {
			t.Errorf("expected peripheral %f < core %f", periph.confidence, core.confidence)
		}
	})

	t.Run("not_indigenous_confidence", func(t *testing.T) {
		t.Helper()
		result := classifyIndigenousByRules("Stock market report for today", "")
		if result.confidence != indigenousConfidenceNotIndigenous {
			t.Errorf("expected %f, got %f", indigenousConfidenceNotIndigenous, result.confidence)
		}
	})

	t.Run("core_capped_at_max", func(t *testing.T) {
		t.Helper()
		// Trigger many patterns at once
		result := classifyIndigenousByRules(
			"First Nations Métis Inuit treaty rights residential school Anishinaabe grand council", "")
		if result.confidence > indigenousConfidenceCoreMax {
			t.Errorf("expected confidence <= %f, got %f", indigenousConfidenceCoreMax, result.confidence)
		}
	})
}

func TestIndigenousRules_CategoryKeywordCoverage(t *testing.T) {
	t.Helper()

	// Verify each category has keywords in the map
	for _, cat := range IndigenousCategories {
		t.Run(cat, func(t *testing.T) {
			t.Helper()
			keywords, ok := indigenousCategoryKeywords[cat]
			if !ok {
				t.Errorf("missing keyword list for category %q", cat)
			}
			if len(keywords) == 0 {
				t.Errorf("empty keyword list for category %q", cat)
			}
		})
	}
}

func TestIndigenousRules_SpanishNotIndigenous(t *testing.T) {
	t.Helper()

	result := classifyIndigenousByRules("El clima de hoy es soleado", "")
	if result.relevance != indigenousRelevanceNot {
		t.Errorf("expected not_indigenous for Spanish weather, got %s", result.relevance)
	}
}

func TestIndigenousRules_FrenchNotIndigenous(t *testing.T) {
	t.Helper()

	result := classifyIndigenousByRules("La météo prévoit du beau temps", "")
	if result.relevance != indigenousRelevanceNot {
		t.Errorf("expected not_indigenous for French weather, got %s", result.relevance)
	}
}
