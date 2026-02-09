//nolint:testpackage // Testing internal router requires same package access
package router

import (
	"testing"
)

func TestGenerateMiningChannels_CoreMining(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:    "test-mining-1",
		Title: "Gold drill results show high grade intercepts",
		Mining: &MiningData{
			Relevance:   "core_mining",
			MiningStage: "exploration",
			Commodities: []string{"gold"},
			Location:    "local_canada",
		},
	}

	channels := GenerateMiningChannels(article)

	if len(channels) != 1 || channels[0] != "articles:mining" {
		t.Errorf("expected [articles:mining], got %v", channels)
	}
}

func TestGenerateMiningChannels_PeripheralMining(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:    "test-mining-2",
		Title: "Mining industry overview",
		Mining: &MiningData{
			Relevance: "peripheral_mining",
		},
	}

	channels := GenerateMiningChannels(article)

	if len(channels) != 1 || channels[0] != "articles:mining" {
		t.Errorf("expected [articles:mining], got %v", channels)
	}
}

func TestGenerateMiningChannels_NotMining(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:    "test-mining-3",
		Title: "Weather forecast",
		Mining: &MiningData{
			Relevance: "not_mining",
		},
	}

	channels := GenerateMiningChannels(article)

	if len(channels) != 0 {
		t.Errorf("expected no channels for not_mining, got %v", channels)
	}
}

func TestGenerateMiningChannels_NilMining(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:    "test-mining-4",
		Title: "Regular news article",
	}

	channels := GenerateMiningChannels(article)

	if len(channels) != 0 {
		t.Errorf("expected no channels for nil mining, got %v", channels)
	}
}

func TestGenerateMiningChannels_EmptyRelevance(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:     "test-mining-5",
		Title:  "Article with empty mining",
		Mining: &MiningData{},
	}

	channels := GenerateMiningChannels(article)

	if len(channels) != 0 {
		t.Errorf("expected no channels for empty relevance, got %v", channels)
	}
}
