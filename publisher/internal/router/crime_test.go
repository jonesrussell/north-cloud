// publisher/internal/router/crime_test.go
//
//nolint:testpackage // Testing internal router requires same package access
package router

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArticle_UnmarshalNestedCrimeFields(t *testing.T) {
	t.Helper()

	esJSON := `{
		"title": "Man charged with murder in downtown Vancouver",
		"body": "A man has been charged...",
		"canonical_url": "https://example.com/article",
		"source": "example_com",
		"quality_score": 75,
		"content_type": "article",
		"is_crime_related": true,
		"crime": {
			"street_crime_relevance": "core_street_crime",
			"sub_label": "",
			"crime_types": ["violent_crime"],
			"location_specificity": "local_canada",
			"final_confidence": 0.95,
			"homepage_eligible": true,
			"category_pages": ["violent-crime", "crime"],
			"review_required": false
		},
		"location": {
			"city": "vancouver",
			"province": "BC",
			"country": "canada",
			"specificity": "city",
			"confidence": 0.85
		}
	}`

	var article Article
	err := json.Unmarshal([]byte(esJSON), &article)
	require.NoError(t, err)

	// Nested structs populated
	require.NotNil(t, article.Crime)
	assert.Equal(t, "core_street_crime", article.Crime.Relevance)
	assert.True(t, article.Crime.Homepage)
	assert.Equal(t, []string{"violent-crime", "crime"}, article.Crime.Categories)

	require.NotNil(t, article.Location)
	assert.Equal(t, "vancouver", article.Location.City)
	assert.Equal(t, "BC", article.Location.Province)
	assert.Equal(t, "canada", article.Location.Country)
}

func TestArticle_FullUnmarshalPipeline(t *testing.T) {
	t.Helper()

	esJSON := `{
		"title": "Drug bust in Toronto",
		"body": "Toronto police seized...",
		"canonical_url": "https://example.com/drug-bust",
		"source": "example_com",
		"quality_score": 80,
		"content_type": "article",
		"is_crime_related": true,
		"crime": {
			"street_crime_relevance": "core_street_crime",
			"sub_label": "",
			"crime_types": ["drug_crime"],
			"location_specificity": "local_canada",
			"final_confidence": 0.90,
			"homepage_eligible": true,
			"category_pages": ["drug-crime", "crime"],
			"review_required": false
		},
		"location": {
			"city": "toronto",
			"province": "ON",
			"country": "canada",
			"specificity": "city",
			"confidence": 0.90
		}
	}`

	var article Article
	err := json.Unmarshal([]byte(esJSON), &article)
	require.NoError(t, err)
	article.extractNestedFields()

	// Crime channels should now generate correctly
	crimeChannels := GenerateCrimeChannels(&article)
	assert.Contains(t, crimeChannels, "crime:homepage")
	assert.Contains(t, crimeChannels, "crime:category:drug-crime")
	assert.Contains(t, crimeChannels, "crime:category:crime")

	// Location channels should now generate correctly
	locationChannels := GenerateLocationChannels(&article)
	assert.Contains(t, locationChannels, "crime:local:toronto")
	assert.Contains(t, locationChannels, "crime:province:on")
	assert.Contains(t, locationChannels, "crime:canada")
}

func TestArticle_ExtractNestedFields_Crime(t *testing.T) {
	t.Helper()

	article := Article{
		Crime: &CrimeData{
			Relevance:  "core_street_crime",
			SubLabel:   "",
			CrimeTypes: []string{"violent_crime"},
			Confidence: 0.95,
			Homepage:   true,
			Categories: []string{"violent-crime", "crime"},
		},
		Location: &LocationData{
			City:        "vancouver",
			Province:    "BC",
			Country:     "canada",
			Specificity: "city",
			Confidence:  0.85,
		},
	}

	article.extractNestedFields()

	// Crime flat fields populated
	assert.Equal(t, "core_street_crime", article.CrimeRelevance)
	assert.Empty(t, article.CrimeSubLabel)
	assert.Equal(t, []string{"violent_crime"}, article.CrimeTypes)
	assert.True(t, article.HomepageEligible)
	assert.Equal(t, []string{"violent-crime", "crime"}, article.CategoryPages)

	// Location flat fields populated
	assert.Equal(t, "vancouver", article.LocationCity)
	assert.Equal(t, "BC", article.LocationProvince)
	assert.Equal(t, "canada", article.LocationCountry)
	assert.Equal(t, "city", article.LocationSpecificity)
	assert.InDelta(t, 0.85, article.LocationConfidence, 0.001)
}

func TestArticle_ExtractNestedFields_NilCrime(t *testing.T) {
	t.Helper()

	article := Article{
		CrimeRelevance: "should-not-change",
	}

	article.extractNestedFields()

	// Flat fields untouched when nested structs are nil
	assert.Equal(t, "should-not-change", article.CrimeRelevance)
}

func TestCrimeRouter_Route_HomepageEligible(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:               "test-1",
		Title:            "Murder suspect arrested",
		HomepageEligible: true,
		CrimeRelevance:   "core_street_crime",
		CategoryPages:    []string{"violent-crime", "crime"},
	}

	// Should publish to crime:homepage and category channels
	channels := GenerateCrimeChannels(article)

	if !containsChannel(channels, "crime:homepage") {
		t.Error("expected crime:homepage channel")
	}

	if !containsChannel(channels, "crime:category:violent-crime") {
		t.Error("expected crime:category:violent-crime channel")
	}
}

func TestCrimeRouter_Route_CoreNotHomepageEligible(t *testing.T) {
	t.Helper()

	// Core street crime that's not homepage eligible still gets category pages
	article := &Article{
		ID:               "test-2",
		Title:            "Minor incident",
		HomepageEligible: false,
		CrimeRelevance:   "core_street_crime",
		CategoryPages:    []string{"crime"},
	}

	channels := GenerateCrimeChannels(article)

	if containsChannel(channels, "crime:homepage") {
		t.Error("should not include homepage for non-eligible article")
	}

	if !containsChannel(channels, "crime:category:crime") {
		t.Error("expected crime:category:crime channel")
	}
}

func TestCrimeRouter_Route_NotCrime(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:             "test-3",
		Title:          "Weather forecast",
		CrimeRelevance: "not_crime",
	}

	channels := GenerateCrimeChannels(article)

	if len(channels) > 0 {
		t.Errorf("expected no channels for not_crime, got %v", channels)
	}
}

func TestCrimeRouter_Route_PeripheralCrime_CriminalJustice(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:             "test-sub-1",
		Title:          "Man sentenced to 10 years",
		CrimeRelevance: "peripheral_crime",
		CrimeSubLabel:  "criminal_justice",
	}

	channels := GenerateCrimeChannels(article)

	if len(channels) != 1 || channels[0] != "crime:courts" {
		t.Errorf("expected only crime:courts, got %v", channels)
	}
}

func TestCrimeRouter_Route_PeripheralCrime_CrimeContext(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:             "test-sub-2",
		Title:          "DOJ releases documents",
		CrimeRelevance: "peripheral_crime",
		CrimeSubLabel:  "crime_context",
	}

	channels := GenerateCrimeChannels(article)

	if len(channels) != 1 || channels[0] != "crime:context" {
		t.Errorf("expected only crime:context, got %v", channels)
	}
}

func TestCrimeRouter_Route_PeripheralCrime_NoSubLabel(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:             "test-sub-3",
		Title:          "Peripheral crime article",
		CrimeRelevance: "peripheral_crime",
		CrimeSubLabel:  "",
	}

	channels := GenerateCrimeChannels(article)

	if len(channels) != 1 || channels[0] != "crime:context" {
		t.Errorf("expected crime:context as default, got %v", channels)
	}
}

func containsChannel(channels []string, target string) bool {
	for _, ch := range channels {
		if ch == target {
			return true
		}
	}
	return false
}

// Location channel tests

func TestGenerateLocationChannels_CanadianCity(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:                  "test-loc-1",
		LocationCity:        "sudbury",
		LocationProvince:    "ON",
		LocationCountry:     "canada",
		LocationSpecificity: "city",
	}

	channels := GenerateLocationChannels(article)

	if !containsChannel(channels, "crime:local:sudbury") {
		t.Error("expected crime:local:sudbury channel")
	}

	if !containsChannel(channels, "crime:province:on") {
		t.Error("expected crime:province:on channel")
	}

	if !containsChannel(channels, "crime:canada") {
		t.Error("expected crime:canada channel")
	}
}

func TestGenerateLocationChannels_International(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:                  "test-loc-2",
		LocationCountry:     "united_states",
		LocationSpecificity: "country",
	}

	channels := GenerateLocationChannels(article)

	if len(channels) != 1 || channels[0] != "crime:international" {
		t.Errorf("expected only crime:international, got %v", channels)
	}
}

func TestGenerateLocationChannels_UnknownLocation(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:                  "test-loc-3",
		LocationCountry:     "unknown",
		LocationSpecificity: "unknown",
	}

	channels := GenerateLocationChannels(article)

	if len(channels) != 0 {
		t.Errorf("expected no channels for unknown location, got %v", channels)
	}
}

func TestGenerateLocationChannels_ProvinceOnly(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:                  "test-loc-4",
		LocationProvince:    "BC",
		LocationCountry:     "canada",
		LocationSpecificity: "province",
	}

	channels := GenerateLocationChannels(article)

	if containsChannel(channels, "crime:local:") {
		t.Error("should not include local channel for province-level specificity")
	}

	if !containsChannel(channels, "crime:province:bc") {
		t.Error("expected crime:province:bc channel")
	}

	if !containsChannel(channels, "crime:canada") {
		t.Error("expected crime:canada channel")
	}
}
