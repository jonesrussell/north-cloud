//nolint:testpackage // Testing internal router requires same package access
package router

import (
	"testing"
)

func TestGenerateEntertainmentChannels_CoreHomepage(t *testing.T) {
	t.Helper()
	article := &Article{
		Title: "Film review",
		Entertainment: &EntertainmentData{
			Relevance:        EntertainmentRelevanceCore,
			Categories:       []string{"film", "reviews"},
			HomepageEligible: true,
		},
	}

	channels := GenerateEntertainmentChannels(article)
	if len(channels) < 2 {
		t.Fatalf("expected at least 2 channels (homepage + categories), got %d", len(channels))
	}
	hasHomepage := false
	hasCategory := false
	for _, c := range channels {
		if c == "entertainment:homepage" {
			hasHomepage = true
		}
		if c == "entertainment:category:film" || c == "entertainment:category:reviews" {
			hasCategory = true
		}
	}
	if !hasHomepage {
		t.Error("expected entertainment:homepage channel")
	}
	if !hasCategory {
		t.Error("expected entertainment:category:* channel")
	}
}

func TestGenerateEntertainmentChannels_Peripheral(t *testing.T) {
	t.Helper()
	article := &Article{
		Title: "Arts news",
		Entertainment: &EntertainmentData{
			Relevance: EntertainmentRelevancePeripheral,
		},
	}

	channels := GenerateEntertainmentChannels(article)
	if len(channels) != 1 || channels[0] != "entertainment:peripheral" {
		t.Errorf("expected [entertainment:peripheral], got %v", channels)
	}
}

func TestGenerateEntertainmentChannels_NotEntertainment(t *testing.T) {
	t.Helper()
	article := &Article{
		Title: "Weather report",
		Entertainment: &EntertainmentData{
			Relevance: EntertainmentRelevanceNot,
		},
	}

	channels := GenerateEntertainmentChannels(article)
	if len(channels) != 0 {
		t.Errorf("expected no channels, got %v", channels)
	}
}

func TestGenerateEntertainmentChannels_NilEntertainment(t *testing.T) {
	t.Helper()
	article := &Article{Title: "No classification"}

	channels := GenerateEntertainmentChannels(article)
	if len(channels) != 0 {
		t.Errorf("expected no channels, got %v", channels)
	}
}
