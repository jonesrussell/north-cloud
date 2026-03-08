//nolint:testpackage // Testing internal router requires same package access
package router

import (
	"testing"
)

func TestGenerateIndigenousChannels_Core(t *testing.T) {
	t.Helper()
	item := &ContentItem{
		Title: "First Nations governance",
		Indigenous: &IndigenousData{
			Relevance:  IndigenousRelevanceCore,
			Categories: []string{"culture", "governance"},
		},
	}

	routes := NewIndigenousDomain().Routes(item)
	channels := routeChannelNames(routes)
	if len(channels) < 2 {
		t.Fatalf("expected at least 2 channels (content:indigenous + categories), got %d", len(channels))
	}
	hasArticles := false
	hasCategory := false
	for _, c := range channels {
		if c == "content:indigenous" {
			hasArticles = true
		}
		if c == "indigenous:category:culture" || c == "indigenous:category:governance" {
			hasCategory = true
		}
	}
	if !hasArticles {
		t.Error("expected content:indigenous channel")
	}
	if !hasCategory {
		t.Error("expected indigenous:category:* channel")
	}
}

func TestGenerateIndigenousChannels_Peripheral(t *testing.T) {
	t.Helper()
	item := &ContentItem{
		Title: "Indigenous reconciliation",
		Indigenous: &IndigenousData{
			Relevance:  IndigenousRelevancePeripheral,
			Categories: []string{"education"},
		},
	}

	routes := NewIndigenousDomain().Routes(item)
	channels := routeChannelNames(routes)
	if len(channels) < 2 {
		t.Fatalf("expected at least 2 channels, got %d", len(channels))
	}
	hasArticles := false
	for _, c := range channels {
		if c == "content:indigenous" {
			hasArticles = true
			break
		}
	}
	if !hasArticles {
		t.Error("expected content:indigenous channel")
	}
}

func TestGenerateIndigenousChannels_NotIndigenous(t *testing.T) {
	t.Helper()
	item := &ContentItem{
		Title: "Weather report",
		Indigenous: &IndigenousData{
			Relevance: IndigenousRelevanceNot,
		},
	}

	routes := NewIndigenousDomain().Routes(item)
	if len(routes) != 0 {
		t.Errorf("expected no channels, got %v", routeChannelNames(routes))
	}
}

func TestGenerateIndigenousChannels_WithRegion(t *testing.T) {
	t.Helper()
	item := &ContentItem{
		Title: "Māori iwi gather for hui",
		Indigenous: &IndigenousData{
			Relevance:  IndigenousRelevanceCore,
			Categories: []string{"culture"},
			Region:     "oceania",
		},
	}

	routes := NewIndigenousDomain().Routes(item)
	channels := routeChannelNames(routes)
	hasRegion := false
	for _, c := range channels {
		if c == "indigenous:region:oceania" {
			hasRegion = true
			break
		}
	}
	if !hasRegion {
		t.Errorf("expected indigenous:region:oceania channel, got %v", channels)
	}
}

func TestGenerateIndigenousChannels_NoRegion(t *testing.T) {
	t.Helper()
	item := &ContentItem{
		Title: "First Nations governance",
		Indigenous: &IndigenousData{
			Relevance:  IndigenousRelevanceCore,
			Categories: []string{"sovereignty"},
		},
	}

	routes := NewIndigenousDomain().Routes(item)
	channels := routeChannelNames(routes)
	for _, c := range channels {
		if len(c) > len("indigenous:region:") && c[:len("indigenous:region:")] == "indigenous:region:" {
			t.Errorf("expected no region channel when region is empty, got %s", c)
		}
	}
}

func TestGenerateIndigenousChannels_NilIndigenous(t *testing.T) {
	t.Helper()
	item := &ContentItem{Title: "No classification"}

	routes := NewIndigenousDomain().Routes(item)
	channels := routeChannelNames(routes)
	if len(channels) != 0 {
		t.Errorf("expected no channels, got %v", channels)
	}
}
