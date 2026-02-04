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

func TestCrimeRouter_Route_NotHomepageEligible(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:               "test-2",
		Title:            "Minor incident",
		HomepageEligible: false,
		CrimeRelevance:   "peripheral_crime",
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

func containsChannel(channels []string, target string) bool {
	for _, ch := range channels {
		if ch == target {
			return true
		}
	}
	return false
}
