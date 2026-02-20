//nolint:testpackage // Testing internal router requires same package access
package router

import (
	"slices"
	"testing"
)

// --- Catch-all channel ---

func TestGenerateMiningChannels_CoreMining_AllLayers(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:    "test-mining-1",
		Title: "Gold drill results show high grade intercepts",
		Mining: &MiningData{
			Relevance:   MiningRelevanceCoreMining,
			MiningStage: "exploration",
			Commodities: []string{"gold"},
			Location:    "local_canada",
		},
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)

	expected := []string{
		"articles:mining",
		"mining:core",
		"mining:commodity:gold",
		"mining:stage:exploration",
		"mining:canada",
	}
	assertChannelsEqual(t, expected, channels)
}

func TestGenerateMiningChannels_PeripheralMining_MinimalFields(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:    "test-mining-2",
		Title: "Mining industry overview",
		Mining: &MiningData{
			Relevance: MiningRelevancePeripheral,
		},
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)

	expected := []string{"articles:mining", "mining:peripheral"}
	assertChannelsEqual(t, expected, channels)
}

// --- Relevance channels ---

func TestGenerateMiningChannels_CoreRelevanceChannel(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining: &MiningData{Relevance: MiningRelevanceCoreMining},
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)
	assertContains(t, channels, "mining:core")
	assertNotContains(t, channels, "mining:peripheral")
}

func TestGenerateMiningChannels_PeripheralRelevanceChannel(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining: &MiningData{Relevance: MiningRelevancePeripheral},
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)
	assertContains(t, channels, "mining:peripheral")
	assertNotContains(t, channels, "mining:core")
}

// --- Commodity channels ---

func TestGenerateMiningChannels_MultipleCommodities(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining: &MiningData{
			Relevance:   MiningRelevanceCoreMining,
			Commodities: []string{"gold", "copper", "lithium"},
		},
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)
	assertContains(t, channels, "mining:commodity:gold")
	assertContains(t, channels, "mining:commodity:copper")
	assertContains(t, channels, "mining:commodity:lithium")
}

func TestGenerateMiningChannels_CommodityUnderscoreToHyphen(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining: &MiningData{
			Relevance:   MiningRelevanceCoreMining,
			Commodities: []string{"iron_ore", "rare_earths"},
		},
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)
	assertContains(t, channels, "mining:commodity:iron-ore")
	assertContains(t, channels, "mining:commodity:rare-earths")
}

func TestGenerateMiningChannels_NoCommodities(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining: &MiningData{Relevance: MiningRelevanceCoreMining},
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)
	for _, c := range channels {
		if len(c) > len("mining:commodity:") && c[:len("mining:commodity:")] == "mining:commodity:" {
			t.Errorf("unexpected commodity channel: %s", c)
		}
	}
}

// --- Stage channels ---

func TestGenerateMiningChannels_StageExploration(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining: &MiningData{
			Relevance:   MiningRelevanceCoreMining,
			MiningStage: "exploration",
		},
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)
	assertContains(t, channels, "mining:stage:exploration")
}

func TestGenerateMiningChannels_StageProduction(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining: &MiningData{
			Relevance:   MiningRelevanceCoreMining,
			MiningStage: "production",
		},
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)
	assertContains(t, channels, "mining:stage:production")
}

func TestGenerateMiningChannels_StageDevelopment(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining: &MiningData{
			Relevance:   MiningRelevanceCoreMining,
			MiningStage: "development",
		},
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)
	assertContains(t, channels, "mining:stage:development")
}

func TestGenerateMiningChannels_StageUnspecifiedSkipped(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining: &MiningData{
			Relevance:   MiningRelevanceCoreMining,
			MiningStage: "unspecified",
		},
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)
	assertNotContains(t, channels, "mining:stage:unspecified")
}

func TestGenerateMiningChannels_StageEmptySkipped(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining: &MiningData{
			Relevance:   MiningRelevanceCoreMining,
			MiningStage: "",
		},
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)
	for _, c := range channels {
		if len(c) > len("mining:stage:") && c[:len("mining:stage:")] == "mining:stage:" {
			t.Errorf("unexpected stage channel: %s", c)
		}
	}
}

// --- Location channels ---

func TestGenerateMiningChannels_LocationLocalCanada(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining: &MiningData{
			Relevance: MiningRelevanceCoreMining,
			Location:  MiningLocationLocalCanada,
		},
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)
	assertContains(t, channels, "mining:canada")
	assertNotContains(t, channels, "mining:international")
}

func TestGenerateMiningChannels_LocationNationalCanada(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining: &MiningData{
			Relevance: MiningRelevanceCoreMining,
			Location:  MiningLocationNationalCanada,
		},
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)
	assertContains(t, channels, "mining:canada")
}

func TestGenerateMiningChannels_LocationInternational(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining: &MiningData{
			Relevance: MiningRelevanceCoreMining,
			Location:  MiningLocationInternational,
		},
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)
	assertContains(t, channels, "mining:international")
	assertNotContains(t, channels, "mining:canada")
}

func TestGenerateMiningChannels_LocationNotSpecifiedSkipped(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining: &MiningData{
			Relevance: MiningRelevanceCoreMining,
			Location:  "not_specified",
		},
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)
	assertNotContains(t, channels, "mining:canada")
	assertNotContains(t, channels, "mining:international")
}

// --- Guard clauses ---

func TestGenerateMiningChannels_NotMining(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:    "test-mining-3",
		Title: "Weather forecast",
		Mining: &MiningData{
			Relevance: MiningRelevanceNotMining,
		},
	}

	routes := NewMiningDomain().Routes(article)

	if len(routes) != 0 {
		t.Errorf("expected no channels for not_mining, got %v", routeChannelNames(routes))
	}
}

func TestGenerateMiningChannels_NilMining(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:    "test-mining-4",
		Title: "Regular news article",
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)

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

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)

	if len(channels) != 0 {
		t.Errorf("expected no channels for empty relevance, got %v", channels)
	}
}

// --- Full integration scenario ---

func TestGenerateMiningChannels_PeripheralWithAllFields(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining: &MiningData{
			Relevance:   MiningRelevancePeripheral,
			MiningStage: "production",
			Commodities: []string{"nickel", "uranium"},
			Location:    MiningLocationInternational,
		},
	}

	routes := NewMiningDomain().Routes(article)
	channels := routeChannelNames(routes)

	expected := []string{
		"articles:mining",
		"mining:peripheral",
		"mining:commodity:nickel",
		"mining:commodity:uranium",
		"mining:stage:production",
		"mining:international",
	}
	assertChannelsEqual(t, expected, channels)
}

// --- Test helpers ---

func assertChannelsEqual(t *testing.T, expected, actual []string) {
	t.Helper()

	if len(expected) != len(actual) {
		t.Errorf("channel count mismatch: expected %d %v, got %d %v", len(expected), expected, len(actual), actual)

		return
	}

	for _, ch := range expected {
		if !slices.Contains(actual, ch) {
			t.Errorf("missing expected channel %q in %v", ch, actual)
		}
	}
}

func assertContains(t *testing.T, channels []string, expected string) {
	t.Helper()

	if !slices.Contains(channels, expected) {
		t.Errorf("expected channel %q in %v", expected, channels)
	}
}

func assertNotContains(t *testing.T, channels []string, unexpected string) {
	t.Helper()

	if slices.Contains(channels, unexpected) {
		t.Errorf("unexpected channel %q in %v", unexpected, channels)
	}
}
