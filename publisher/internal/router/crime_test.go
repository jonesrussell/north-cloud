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

func TestContentItem_UnmarshalNestedCrimeFields(t *testing.T) {
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

	var item ContentItem
	err := json.Unmarshal([]byte(esJSON), &item)
	require.NoError(t, err)

	// Nested structs populated
	require.NotNil(t, item.Crime)
	assert.Equal(t, "core_street_crime", item.Crime.Relevance)
	assert.True(t, item.Crime.Homepage)
	assert.Equal(t, []string{"violent-crime", "crime"}, item.Crime.Categories)

	require.NotNil(t, item.Location)
	assert.Equal(t, "vancouver", item.Location.City)
	assert.Equal(t, "BC", item.Location.Province)
	assert.Equal(t, "canada", item.Location.Country)
}

func TestContentItem_FullUnmarshalPipeline(t *testing.T) {
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

	var item ContentItem
	err := json.Unmarshal([]byte(esJSON), &item)
	require.NoError(t, err)
	item.extractNestedFields()

	// Crime channels should now generate correctly
	routes := NewCrimeDomain().Routes(&item)
	crimeChannels := routeChannelNames(routes)
	assert.Contains(t, crimeChannels, "crime:homepage")
	assert.Contains(t, crimeChannels, "crime:category:drug-crime")
	assert.Contains(t, crimeChannels, "crime:category:crime")

	// Location channels should now generate correctly
	locationRoutes := NewLocationDomain().Routes(&item)
	locationChannels := routeChannelNames(locationRoutes)
	assert.Contains(t, locationChannels, "crime:local:toronto")
	assert.Contains(t, locationChannels, "crime:province:on")
	assert.Contains(t, locationChannels, "crime:canada")
}

func TestContentItem_ExtractNestedFields_Crime(t *testing.T) {
	t.Helper()

	item := ContentItem{
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

	item.extractNestedFields()

	// Crime flat fields populated
	assert.Equal(t, "core_street_crime", item.CrimeRelevance)
	assert.Empty(t, item.CrimeSubLabel)
	assert.Equal(t, []string{"violent_crime"}, item.CrimeTypes)
	assert.True(t, item.HomepageEligible)
	assert.Equal(t, []string{"violent-crime", "crime"}, item.CategoryPages)

	// Location flat fields populated
	assert.Equal(t, "vancouver", item.LocationCity)
	assert.Equal(t, "BC", item.LocationProvince)
	assert.Equal(t, "canada", item.LocationCountry)
	assert.Equal(t, "city", item.LocationSpecificity)
	assert.InDelta(t, 0.85, item.LocationConfidence, 0.001)
}

func TestContentItem_ExtractNestedFields_NilCrime(t *testing.T) {
	t.Helper()

	item := ContentItem{
		CrimeRelevance: "should-not-change",
	}

	item.extractNestedFields()

	// Flat fields untouched when nested structs are nil
	assert.Equal(t, "should-not-change", item.CrimeRelevance)
}

func TestCrimeRouter_Route_HomepageEligible(t *testing.T) {
	t.Helper()

	item := &ContentItem{
		ID:               "test-1",
		Title:            "Murder suspect arrested",
		HomepageEligible: true,
		CrimeRelevance:   "core_street_crime",
		CategoryPages:    []string{"violent-crime", "crime"},
	}

	routes := NewCrimeDomain().Routes(item)
	channels := routeChannelNames(routes)

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
	item := &ContentItem{
		ID:               "test-2",
		Title:            "Minor incident",
		HomepageEligible: false,
		CrimeRelevance:   "core_street_crime",
		CategoryPages:    []string{"crime"},
	}

	routes := NewCrimeDomain().Routes(item)
	channels := routeChannelNames(routes)

	if containsChannel(channels, "crime:homepage") {
		t.Error("should not include homepage for non-eligible item")
	}

	if !containsChannel(channels, "crime:category:crime") {
		t.Error("expected crime:category:crime channel")
	}
}

func TestCrimeRouter_Route_NotCrime(t *testing.T) {
	t.Helper()

	item := &ContentItem{
		ID:             "test-3",
		Title:          "Weather forecast",
		CrimeRelevance: "not_crime",
	}

	routes := NewCrimeDomain().Routes(item)
	channels := routeChannelNames(routes)

	if len(channels) > 0 {
		t.Errorf("expected no channels for not_crime, got %v", channels)
	}
}

func TestCrimeRouter_Route_PeripheralCrime_CriminalJustice(t *testing.T) {
	t.Helper()

	item := &ContentItem{
		ID:             "test-sub-1",
		Title:          "Man sentenced to 10 years",
		CrimeRelevance: "peripheral_crime",
		CrimeSubLabel:  "criminal_justice",
	}

	routes := NewCrimeDomain().Routes(item)
	channels := routeChannelNames(routes)

	if len(channels) != 1 || channels[0] != "crime:courts" {
		t.Errorf("expected only crime:courts, got %v", channels)
	}
}

func TestCrimeRouter_Route_PeripheralCrime_CrimeContext(t *testing.T) {
	t.Helper()

	item := &ContentItem{
		ID:             "test-sub-2",
		Title:          "DOJ releases documents",
		CrimeRelevance: "peripheral_crime",
		CrimeSubLabel:  "crime_context",
	}

	routes := NewCrimeDomain().Routes(item)
	channels := routeChannelNames(routes)

	if len(channels) != 1 || channels[0] != "crime:context" {
		t.Errorf("expected only crime:context, got %v", channels)
	}
}

func TestCrimeRouter_Route_PeripheralCrime_NoSubLabel(t *testing.T) {
	t.Helper()

	item := &ContentItem{
		ID:             "test-sub-3",
		Title:          "Peripheral crime item",
		CrimeRelevance: "peripheral_crime",
		CrimeSubLabel:  "",
	}

	routes := NewCrimeDomain().Routes(item)
	channels := routeChannelNames(routes)

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
