// publisher/internal/router/crime_test.go
//
//nolint:testpackage // Testing internal router requires same package access
package router

import (
	"testing"
)

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
