// classifier/internal/classifier/crime_rules_test.go
//
//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCrimeRules_ClassifyByRules_ViolentCrime(t *testing.T) {
	t.Helper()

	tests := []struct {
		name              string
		title             string
		expectedRelevance string
		expectedTypes     []string
	}{
		{
			name:              "murder",
			title:             "Man charged with murder after stabbing",
			expectedRelevance: "core_street_crime",
			expectedTypes:     []string{"violent_crime"},
		},
		{
			name:              "shooting",
			title:             "Police respond to downtown shooting",
			expectedRelevance: "core_street_crime",
			expectedTypes:     []string{"violent_crime"},
		},
		{
			name:              "assault with arrest",
			title:             "Suspect arrested for assault in park",
			expectedRelevance: "core_street_crime",
			expectedTypes:     []string{"violent_crime"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")

			if result.relevance != tt.expectedRelevance {
				t.Errorf("relevance: got %s, want %s", result.relevance, tt.expectedRelevance)
			}

			for _, expectedType := range tt.expectedTypes {
				if !slices.Contains(result.crimeTypes, expectedType) {
					t.Errorf("missing crime type %s in %v", expectedType, result.crimeTypes)
				}
			}
		})
	}
}

func TestCrimeRules_ClassifyByRules_Exclusions(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
	}{
		{"job posting", "Full-Time Position Available"},
		{"directory", "Listings By Category"},
		{"sports", "Local Sports Updates"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")

			if result.relevance != "not_crime" {
				t.Errorf("expected not_crime for excluded content, got %s", result.relevance)
			}
		})
	}
}

func TestCrimeRules_ClassifyByRules_NotCrime(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
	}{
		{"restaurant", "New restaurant opens downtown"},
		{"weather", "Weekend forecast looks sunny"},
		{"hockey", "Hockey team wins championship"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")

			if result.relevance != "not_crime" {
				t.Errorf("expected not_crime, got %s", result.relevance)
			}
		})
	}
}

func TestCrimeRules_ClassifyByRules_OpinionExclusions(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
	}{
		{"opinion prefix", "Opinion: Crime rates are a political tool"},
		{"editorial prefix", "Editorial: Why policing needs reform"},
		{"commentary prefix", "Commentary: The murder rate debate"},
		{"column prefix", "Column: My thoughts on gang violence"},
		{"op-ed prefix", "Op-Ed: Drug policy has failed us"},
		{"letters prefix", "Letters: Readers respond to shooting coverage"},
		{"first person opinion", "I think the police response was inadequate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")
			assert.Equal(t, "not_crime", result.relevance,
				"title %q should be excluded as opinion", tt.title)
		})
	}
}

func TestCrimeRules_ClassifyByRules_LifestyleExclusions(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
	}{
		{"renovation", "7 best house renovation contractors in the area"},
		{"tournament", "PUBG online tournament finals this weekend"},
		{"travel guide", "A new lifeline for anyone travelling through BC"},
		{"recipe", "Best recipe for a killer BBQ sauce"},
		{"best-of list", "Best contractors in the Vancouver area"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")
			assert.Equal(t, "not_crime", result.relevance,
				"title %q should be excluded as lifestyle", tt.title)
		})
	}
}

func TestCrimeRules_ClassifyByRules_RequiresAuthority(t *testing.T) {
	t.Helper()

	// These should STILL be core_street_crime (have authority indicators)
	coreTests := []struct {
		name  string
		title string
	}{
		{"murder with police", "Police investigate murder in downtown Toronto"},
		{"shooting with RCMP", "RCMP respond to shooting at mall"},
		{"stabbing with arrest", "Man arrested after stabbing outside bar"},
		{"drug bust", "Police drug bust seizes fentanyl in Vancouver"},
		{"sexual assault charged", "Suspect charged with sexual assault"},
		{"found dead investigation", "Woman found dead, police launch investigation"},
		{"sentenced murder", "Man sentenced to life for murder of wife"},
	}

	for _, tt := range coreTests {
		t.Run("core: "+tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")
			assert.Equal(t, "core_street_crime", result.relevance,
				"title %q should be core_street_crime", tt.title)
		})
	}

	// These should NOT be core_street_crime (no authority indicator)
	nonCoreTests := []struct {
		name  string
		title string
	}{
		{"murder fiction", "Murder on the Orient Express returns to stage"},
		{"shooting metaphor", "Shooting for the stars: local athlete's journey"},
		{"stabbing game", "Stabbing mechanics in new action RPG reviewed"},
	}

	for _, tt := range nonCoreTests {
		t.Run("not-core: "+tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")
			assert.NotEqual(t, "core_street_crime", result.relevance,
				"title %q should NOT be core_street_crime", tt.title)
		})
	}
}

func TestCrimeRules_ClassifyByRules_MissingPatterns(t *testing.T) {
	t.Helper()

	tests := []struct {
		name              string
		title             string
		expectedRelevance string
		expectedTypes     []string
	}{
		{
			name:              "robbery with arrest",
			title:             "Repeat offender among two arrested in store robbery",
			expectedRelevance: "core_street_crime",
			expectedTypes:     []string{"violent_crime"},
		},
		{
			name:              "armed robbery",
			title:             "Armed robbery at downtown convenience store, police investigating",
			expectedRelevance: "core_street_crime",
			expectedTypes:     []string{"violent_crime"},
		},
		{
			name:              "robbery with RCMP",
			title:             "RCMP investigating bank robbery in Sudbury",
			expectedRelevance: "core_street_crime",
			expectedTypes:     []string{"violent_crime"},
		},
		{
			name:              "carjacking",
			title:             "Police arrest suspect in violent carjacking incident",
			expectedRelevance: "core_street_crime",
			expectedTypes:     []string{"violent_crime"},
		},
		{
			name:              "kidnapping",
			title:             "Man charged with kidnapping after Amber Alert",
			expectedRelevance: "core_street_crime",
			expectedTypes:     []string{"violent_crime"},
		},
		{
			name:              "abduction",
			title:             "Police searching for suspect in child abduction",
			expectedRelevance: "core_street_crime",
			expectedTypes:     []string{"violent_crime"},
		},
		{
			name:              "hostage",
			title:             "Hostage situation ends with arrest by tactical unit",
			expectedRelevance: "core_street_crime",
			expectedTypes:     []string{"violent_crime"},
		},
		{
			name:              "custody authority indicator",
			title:             "Suspect taken into custody after downtown stabbing",
			expectedRelevance: "core_street_crime",
			expectedTypes:     []string{"violent_crime"},
		},
		{
			name:              "manhunt authority indicator",
			title:             "Manhunt underway after shooting in North Bay",
			expectedRelevance: "core_street_crime",
			expectedTypes:     []string{"violent_crime"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")

			if result.relevance != tt.expectedRelevance {
				t.Errorf("relevance: got %s, want %s for title: %s",
					result.relevance, tt.expectedRelevance, tt.title)
			}

			for _, expectedType := range tt.expectedTypes {
				if !slices.Contains(result.crimeTypes, expectedType) {
					t.Errorf("missing crime type %s in %v for title: %s",
						expectedType, result.crimeTypes, tt.title)
				}
			}
		})
	}
}

func TestCrimeRules_ClassifyByRules_CourtOutcomes(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
	}{
		{"sentenced", "Man sentenced to 15 years in prison for armed robbery"},
		{"convicted", "Jury convicts accused in deadly shooting case"},
		{"found guilty", "Woman found guilty of fraud by judge"},
		{"pleaded guilty", "Teen pleaded guilty to assault charges in court"},
		{"prison term", "Judge hands down prison term for drug trafficking ring leader"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")
			assert.Equal(t, "core_street_crime", result.relevance,
				"title %q should be core_street_crime", tt.title)
			assert.Contains(t, result.crimeTypes, "criminal_justice",
				"title %q should have criminal_justice crime type", tt.title)
		})
	}
}

// TestCrimeRules_TitleAndBodyPrefix verifies that crime signals in the body
// trigger rule-based core_street_crime when the title is vague.
func TestCrimeRules_TitleAndBodyPrefix(t *testing.T) {
	t.Helper()

	t.Run("vague title with robbery and arrested in body", func(t *testing.T) {
		title := "Two charged"
		body := "Police said the two suspects were arrested after an armed robbery at a convenience store. The incident occurred Tuesday night."
		result := classifyByRules(title, body)
		assert.Equal(t, relevanceCoreStreetCrime, result.relevance,
			"expected core_street_crime when body contains 'arrested' and 'armed robbery'")
		assert.Contains(t, result.crimeTypes, "violent_crime",
			"expected violent_crime type from robbery pattern")
	})

	t.Run("exclusion remains title-only", func(t *testing.T) {
		title := "Register for updates"
		body := "Police arrested a man after a shooting downtown. The suspect is in custody."
		result := classifyByRules(title, body)
		assert.Equal(t, relevanceNotCrime, result.relevance,
			"exclusion matched on title only; body crime text should not override")
	})
}
