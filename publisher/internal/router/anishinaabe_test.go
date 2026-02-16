//nolint:testpackage // Testing internal router requires same package access
package router

import (
	"testing"
)

func TestGenerateAnishinaabeChannels_Core(t *testing.T) {
	t.Helper()
	article := &Article{
		Title: "First Nations governance",
		Anishinaabe: &AnishinaabeData{
			Relevance:  AnishinaabeRelevanceCore,
			Categories: []string{"culture", "governance"},
		},
	}

	channels := GenerateAnishinaabeChannels(article)
	if len(channels) < 2 {
		t.Fatalf("expected at least 2 channels (articles:anishinaabe + categories), got %d", len(channels))
	}
	hasArticles := false
	hasCategory := false
	for _, c := range channels {
		if c == "articles:anishinaabe" {
			hasArticles = true
		}
		if c == "anishinaabe:category:culture" || c == "anishinaabe:category:governance" {
			hasCategory = true
		}
	}
	if !hasArticles {
		t.Error("expected articles:anishinaabe channel")
	}
	if !hasCategory {
		t.Error("expected anishinaabe:category:* channel")
	}
}

func TestGenerateAnishinaabeChannels_Peripheral(t *testing.T) {
	t.Helper()
	article := &Article{
		Title: "Indigenous reconciliation",
		Anishinaabe: &AnishinaabeData{
			Relevance:  AnishinaabeRelevancePeripheral,
			Categories: []string{"education"},
		},
	}

	channels := GenerateAnishinaabeChannels(article)
	if len(channels) < 2 {
		t.Fatalf("expected at least 2 channels, got %d", len(channels))
	}
	hasArticles := false
	for _, c := range channels {
		if c == "articles:anishinaabe" {
			hasArticles = true
			break
		}
	}
	if !hasArticles {
		t.Error("expected articles:anishinaabe channel")
	}
}

func TestGenerateAnishinaabeChannels_NotAnishinaabe(t *testing.T) {
	t.Helper()
	article := &Article{
		Title: "Weather report",
		Anishinaabe: &AnishinaabeData{
			Relevance: AnishinaabeRelevanceNot,
		},
	}

	channels := GenerateAnishinaabeChannels(article)
	if len(channels) != 0 {
		t.Errorf("expected no channels, got %v", channels)
	}
}

func TestGenerateAnishinaabeChannels_NilAnishinaabe(t *testing.T) {
	t.Helper()
	article := &Article{Title: "No classification"}

	channels := GenerateAnishinaabeChannels(article)
	if len(channels) != 0 {
		t.Errorf("expected no channels, got %v", channels)
	}
}
