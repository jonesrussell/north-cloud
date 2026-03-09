package router_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
	"github.com/jonesrussell/north-cloud/publisher/internal/router"
	"github.com/stretchr/testify/assert"
)

// TestLayer1RoutingScenarios tests various Layer 1 (topic-based) routing scenarios
func TestLayer1RoutingScenarios(t *testing.T) {
	testCases := []struct {
		name             string
		item             *router.ContentItem
		expectedChannels []string
	}{
		{
			name: "crime item routes to crime sub-category channels",
			item: &router.ContentItem{
				ID:           "item-1",
				Title:        "Armed robbery reported downtown",
				Topics:       []string{"violent_crime", "local_news"},
				QualityScore: 75,
				ContentType:  "article",
			},
			expectedChannels: []string{"content:violent_crime", "content:local_news"},
		},
		{
			name: "property crime item routes correctly",
			item: &router.ContentItem{
				ID:           "item-2",
				Title:        "Car theft on the rise",
				Topics:       []string{"property_crime"},
				QualityScore: 65,
				ContentType:  "article",
			},
			expectedChannels: []string{"content:property_crime"},
		},
		{
			name: "multi-topic item routes to all topic channels",
			item: &router.ContentItem{
				ID:           "item-3",
				Title:        "Drug bust leads to arrests",
				Topics:       []string{"drug_crime", "criminal_justice", "local_news"},
				QualityScore: 80,
				ContentType:  "article",
			},
			expectedChannels: []string{"content:drug_crime", "content:criminal_justice", "content:local_news"},
		},
		{
			name: "non-crime news item routes to topic channel",
			item: &router.ContentItem{
				ID:           "item-4",
				Title:        "New park opens in city center",
				Topics:       []string{"local_news"},
				QualityScore: 60,
				ContentType:  "article",
			},
			expectedChannels: []string{"content:local_news"},
		},
		{
			name: "item with no topics generates no Layer 1 channels",
			item: &router.ContentItem{
				ID:           "item-5",
				Title:        "Unclassified item",
				Topics:       []string{},
				QualityScore: 50,
				ContentType:  "article",
			},
			expectedChannels: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			routes := router.NewTopicDomain().Routes(tc.item)
			channels := channelNames(routes)

			assert.Len(t, channels, len(tc.expectedChannels), "unexpected number of channels")
			for i, expected := range tc.expectedChannels {
				assert.Equal(t, expected, channels[i], "channel mismatch at index %d", i)
			}
		})
	}
}

// TestLayer2RoutingScenarios tests various Layer 2 (custom channel with rules) routing scenarios
func TestLayer2RoutingScenarios(t *testing.T) {
	// Define test channels with various rule configurations
	crimeAggregatorChannel := createTestChannel("crime-aggregator",
		"content:crime:all",
		models.Rules{
			IncludeTopics:   []string{"violent_crime", "property_crime", "drug_crime", "organized_crime", "criminal_justice"},
			MinQualityScore: 50,
			ContentTypes:    []string{"article"},
		})

	highQualityNewsChannel := createTestChannel("premium-news",
		"content:premium",
		models.Rules{
			MinQualityScore: 80,
			ContentTypes:    []string{"article"},
		})

	violentCrimeOnlyChannel := createTestChannel("violent-crime-only",
		"content:violent:exclusive",
		models.Rules{
			IncludeTopics: []string{"violent_crime"},
			ExcludeTopics: []string{"criminal_justice"}, // Exclude court proceedings
			ContentTypes:  []string{"article"},
		})

	catchAllChannel := createTestChannel("catch-all",
		"content:all",
		models.Rules{}) // Empty rules = matches everything

	testCases := []struct {
		name            string
		item            *router.ContentItem
		channels        []models.Channel
		expectedMatches []string // Redis channel names that should match
	}{
		{
			name: "violent crime item matches crime aggregator and violent-only",
			item: &router.ContentItem{
				ID:           "item-1",
				Title:        "Armed robbery reported",
				Topics:       []string{"violent_crime"},
				QualityScore: 75,
				ContentType:  "article",
			},
			channels: []models.Channel{crimeAggregatorChannel, violentCrimeOnlyChannel, highQualityNewsChannel},
			expectedMatches: []string{
				"content:crime:all",         // Crime aggregator
				"content:violent:exclusive", // Violent crime only
				// NOT premium (score 75 < 80)
			},
		},
		{
			name: "high quality news matches premium channel",
			item: &router.ContentItem{
				ID:           "item-2",
				Title:        "Major tech breakthrough",
				Topics:       []string{"technology", "business"},
				QualityScore: 90,
				ContentType:  "article",
			},
			channels:        []models.Channel{crimeAggregatorChannel, highQualityNewsChannel},
			expectedMatches: []string{"content:premium"}, // Only premium (not crime)
		},
		{
			name: "violent crime with court proceedings excluded from violent-only channel",
			item: &router.ContentItem{
				ID:           "item-3",
				Title:        "Sentencing in murder case",
				Topics:       []string{"violent_crime", "criminal_justice"},
				QualityScore: 70,
				ContentType:  "article",
			},
			channels: []models.Channel{crimeAggregatorChannel, violentCrimeOnlyChannel},
			expectedMatches: []string{
				"content:crime:all", // Crime aggregator (doesn't exclude justice)
				// NOT violent-only (excluded by criminal_justice topic)
			},
		},
		{
			name: "low quality item fails premium threshold",
			item: &router.ContentItem{
				ID:           "item-4",
				Title:        "Quick news update",
				Topics:       []string{"local_news"},
				QualityScore: 45,
				ContentType:  "article",
			},
			channels:        []models.Channel{highQualityNewsChannel, crimeAggregatorChannel},
			expectedMatches: []string{}, // Neither (not crime, not high quality)
		},
		{
			name: "page content type excluded by article-only rules",
			item: &router.ContentItem{
				ID:           "item-5",
				Title:        "About Us",
				Topics:       []string{"violent_crime"}, // Even if topic matches
				QualityScore: 100,
				ContentType:  "page", // Wrong content type
			},
			channels:        []models.Channel{crimeAggregatorChannel, highQualityNewsChannel},
			expectedMatches: []string{}, // None match due to content type
		},
		{
			name: "catch-all channel matches everything",
			item: &router.ContentItem{
				ID:           "item-6",
				Title:        "Random item",
				Topics:       []string{"misc"},
				QualityScore: 30,
				ContentType:  "listing", // Unusual content type
			},
			channels:        []models.Channel{catchAllChannel, crimeAggregatorChannel},
			expectedMatches: []string{"content:all"}, // Only catch-all
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var matchedChannels []string

			for i := range tc.channels {
				ch := &tc.channels[i]
				if ch.Rules.Matches(tc.item.QualityScore, tc.item.ContentType, tc.item.Topics) {
					matchedChannels = append(matchedChannels, ch.RedisChannel)
				}
			}

			assert.ElementsMatch(t, tc.expectedMatches, matchedChannels,
				"unexpected matched channels")
		})
	}
}

// TestCombinedLayerRoutingScenarios tests that both layers work correctly together
func TestCombinedLayerRoutingScenarios(t *testing.T) {
	// Layer 2 custom channels
	crimeChannel := createTestChannel("crime-aggregator",
		"custom:crime:all",
		models.Rules{
			IncludeTopics:   []string{"violent_crime", "property_crime", "drug_crime"},
			MinQualityScore: 60,
		})

	premiumChannel := createTestChannel("premium",
		"custom:premium",
		models.Rules{
			MinQualityScore: 85,
		})

	testCases := []struct {
		name              string
		item              *router.ContentItem
		customChannels    []models.Channel
		expectedLayer1    []string
		expectedLayer2    []string
		expectedTotalPubs int
	}{
		{
			name: "crime item publishes to both layers",
			item: &router.ContentItem{
				ID:           "item-1",
				Topics:       []string{"violent_crime", "local_news"},
				QualityScore: 75,
				ContentType:  "article",
			},
			customChannels: []models.Channel{crimeChannel, premiumChannel},
			expectedLayer1: []string{"content:violent_crime", "content:local_news"},
			expectedLayer2: []string{"custom:crime:all"},
			// 2 Layer 1 + 1 Layer 2 = 3 total
			expectedTotalPubs: 3,
		},
		{
			name: "high-quality crime item hits all channels",
			item: &router.ContentItem{
				ID:           "item-2",
				Topics:       []string{"drug_crime"},
				QualityScore: 90,
				ContentType:  "article",
			},
			customChannels:    []models.Channel{crimeChannel, premiumChannel},
			expectedLayer1:    []string{"content:drug_crime"},
			expectedLayer2:    []string{"custom:crime:all", "custom:premium"},
			expectedTotalPubs: 3, // 1 Layer 1 + 2 Layer 2
		},
		{
			name: "non-crime item skips crime channel",
			item: &router.ContentItem{
				ID:           "item-3",
				Topics:       []string{"technology"},
				QualityScore: 65,
				ContentType:  "article",
			},
			customChannels:    []models.Channel{crimeChannel, premiumChannel},
			expectedLayer1:    []string{"content:technology"},
			expectedLayer2:    []string{}, // No matches
			expectedTotalPubs: 1,          // Just Layer 1
		},
		{
			name: "no topics means Layer 1 only has Layer 2 contributions",
			item: &router.ContentItem{
				ID:           "item-4",
				Topics:       []string{},
				QualityScore: 95,
				ContentType:  "article",
			},
			customChannels:    []models.Channel{crimeChannel, premiumChannel},
			expectedLayer1:    []string{},
			expectedLayer2:    []string{"custom:premium"}, // Still matches premium by quality
			expectedTotalPubs: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Layer 1 channels
			layer1Routes := router.NewTopicDomain().Routes(tc.item)
			layer1Channels := channelNames(layer1Routes)
			assert.ElementsMatch(t, tc.expectedLayer1, layer1Channels, "Layer 1 mismatch")

			// Layer 2 channels
			var layer2Channels []string
			for i := range tc.customChannels {
				ch := &tc.customChannels[i]
				if ch.Rules.Matches(tc.item.QualityScore, tc.item.ContentType, tc.item.Topics) {
					layer2Channels = append(layer2Channels, ch.RedisChannel)
				}
			}
			assert.ElementsMatch(t, tc.expectedLayer2, layer2Channels, "Layer 2 mismatch")

			// Total publications
			totalPubs := len(layer1Channels) + len(layer2Channels)
			assert.Equal(t, tc.expectedTotalPubs, totalPubs, "total publications mismatch")
		})
	}
}

// TestRulesEdgeCases tests edge cases in rule matching
func TestRulesEdgeCases(t *testing.T) {
	testCases := []struct {
		name         string
		rules        models.Rules
		qualityScore int
		contentType  string
		topics       []string
		expected     bool
	}{
		{
			name:         "zero quality score with min requirement fails",
			rules:        models.Rules{MinQualityScore: 1},
			qualityScore: 0,
			contentType:  "article",
			topics:       []string{"news"},
			expected:     false,
		},
		{
			name:         "exact quality score threshold passes",
			rules:        models.Rules{MinQualityScore: 50},
			qualityScore: 50,
			contentType:  "article",
			topics:       []string{"news"},
			expected:     true,
		},
		{
			name:         "one below quality threshold fails",
			rules:        models.Rules{MinQualityScore: 50},
			qualityScore: 49,
			contentType:  "article",
			topics:       []string{"news"},
			expected:     false,
		},
		{
			name:         "maximum quality score passes any threshold",
			rules:        models.Rules{MinQualityScore: 100},
			qualityScore: 100,
			contentType:  "article",
			topics:       []string{"news"},
			expected:     true,
		},
		{
			name:         "empty topics with include requirement fails",
			rules:        models.Rules{IncludeTopics: []string{"crime"}},
			qualityScore: 50,
			contentType:  "article",
			topics:       []string{},
			expected:     false,
		},
		{
			name:         "nil topics treated as empty",
			rules:        models.Rules{IncludeTopics: []string{"crime"}},
			qualityScore: 50,
			contentType:  "article",
			topics:       nil,
			expected:     false,
		},
		{
			name:         "empty content type with requirement fails",
			rules:        models.Rules{ContentTypes: []string{"article"}},
			qualityScore: 50,
			contentType:  "",
			topics:       []string{"news"},
			expected:     false,
		},
		{
			name:         "case sensitive topic matching",
			rules:        models.Rules{IncludeTopics: []string{"Crime"}},
			qualityScore: 50,
			contentType:  "article",
			topics:       []string{"crime"}, // lowercase
			expected:     false,             // Should not match due to case
		},
		{
			name:         "exclude takes precedence over include",
			rules:        models.Rules{IncludeTopics: []string{"crime"}, ExcludeTopics: []string{"crime"}},
			qualityScore: 50,
			contentType:  "article",
			topics:       []string{"crime"},
			expected:     false,
		},
		{
			name: "all rules must pass (AND logic)",
			rules: models.Rules{
				IncludeTopics:   []string{"crime"},
				MinQualityScore: 80,
				ContentTypes:    []string{"article"},
			},
			qualityScore: 70, // Fails quality
			contentType:  "article",
			topics:       []string{"crime"},
			expected:     false,
		},
		{
			name:         "multiple content types - matches any (OR logic)",
			rules:        models.Rules{ContentTypes: []string{"article", "video", "podcast"}},
			qualityScore: 50,
			contentType:  "video",
			topics:       []string{"news"},
			expected:     true,
		},
		{
			name:         "multiple include topics - matches any (OR logic)",
			rules:        models.Rules{IncludeTopics: []string{"violent_crime", "property_crime"}},
			qualityScore: 50,
			contentType:  "article",
			topics:       []string{"violent_crime"}, // Only matches one
			expected:     true,
		},
		{
			name:         "multiple exclude topics - excluded by any (OR logic)",
			rules:        models.Rules{ExcludeTopics: []string{"spam", "advertisement"}},
			qualityScore: 50,
			contentType:  "article",
			topics:       []string{"news", "spam"}, // Has one excluded topic
			expected:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.rules.Matches(tc.qualityScore, tc.contentType, tc.topics)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestCrimeSubCategoryRouting tests routing for the 5 crime sub-categories
func TestCrimeSubCategoryRouting(t *testing.T) {
	crimeSubCategories := []string{
		"violent_crime",
		"property_crime",
		"drug_crime",
		"organized_crime",
		"criminal_justice",
	}

	for _, category := range crimeSubCategories {
		t.Run("Layer1_routes_"+category, func(t *testing.T) {
			item := &router.ContentItem{
				ID:     "test-item",
				Topics: []string{category},
			}

			routes := router.NewTopicDomain().Routes(item)
			channels := channelNames(routes)

			assert.Len(t, channels, 1)
			assert.Equal(t, "content:"+category, channels[0])
		})
	}

	// Test item with all crime sub-categories
	t.Run("item_with_all_crime_categories", func(t *testing.T) {
		item := &router.ContentItem{
			ID:     "multi-crime-item",
			Topics: crimeSubCategories,
		}

		routes := router.NewTopicDomain().Routes(item)
		channels := channelNames(routes)

		assert.Len(t, channels, len(crimeSubCategories))
		for i, category := range crimeSubCategories {
			assert.Equal(t, "content:"+category, channels[i])
		}
	})
}

// createTestChannel is a helper to create test Channel instances
func createTestChannel(slug, redisChannel string, rules models.Rules) models.Channel {
	return models.Channel{
		ID:           uuid.New(),
		Name:         slug,
		Slug:         slug,
		RedisChannel: redisChannel,
		Rules:        rules,
		RulesVersion: 1,
		Enabled:      true,
	}
}

// TestAllDomainsProduce_FullyClassifiedContentItem verifies that each domain in the
// routing pipeline produces at least one route when given a fully-classified content item.
// This catches domains accidentally omitted from routeContentItem's domain slice,
// or domains whose entry conditions are misconfigured.
func TestAllDomainsProduce_FullyClassifiedContentItem(t *testing.T) {
	// Build a fully-classified content item that every domain should match.
	// CrimeDomain and LocationDomain read flat fields; all others read nested pointers.
	item := &router.ContentItem{
		Topics:                        []string{"news"},     // TopicDomain: content:news
		QualityScore:                  80,                   // DBChannelDomain: meets min threshold
		ContentType:                   "article",            // DBChannelDomain: content type match
		CrimeRelevance:                "core_street_crime",  // CrimeDomain
		HomepageEligible:              true,                 // CrimeDomain: crime:homepage
		LocationCountry:               "canada",             // LocationDomain: non-empty, non-unknown
		LocationSpecificity:           "national_canada",    // LocationDomain: crime:canada prefix
		EntertainmentRelevance:        "core_entertainment", // LocationDomain entertainment prefix
		EntertainmentHomepageEligible: true,
		Mining: &router.MiningData{ // MiningDomain
			Relevance: "core_mining",
			Location:  "national_canada",
		},
		Entertainment: &router.EntertainmentData{ // EntertainmentDomain
			Relevance:        "core_entertainment",
			HomepageEligible: true,
			Categories:       []string{"film"},
		},
		Indigenous: &router.IndigenousData{ // IndigenousDomain
			Relevance:       "core_indigenous",
			Categories:      []string{"culture"},
			FinalConfidence: 0.85,
		},
		Coforge: &router.CoforgeData{ // CoforgeDomain
			Relevance: "core_coforge",
			Audience:  "developer",
		},
	}

	dbChannel := models.Channel{
		ID:           uuid.New(),
		RedisChannel: "content:premium",
		Rules:        models.Rules{MinQualityScore: 50, ContentTypes: []string{"article"}},
		Enabled:      true,
	}

	// domains in the same order as routeContentItem constructs them
	domainCases := []struct {
		name   string
		domain router.RoutingDomain
	}{
		{"topic", router.NewTopicDomain()},
		{"db_channel", router.NewDBChannelDomain([]models.Channel{dbChannel})},
		{"crime", router.NewCrimeDomain()},
		{"location", router.NewLocationDomain()},
		{"mining", router.NewMiningDomain()},
		{"entertainment", router.NewEntertainmentDomain()},
		{"indigenous", router.NewIndigenousDomain()},
		{"coforge", router.NewCoforgeDomain()},
	}

	for _, dc := range domainCases {
		t.Run(dc.name, func(t *testing.T) {
			routes := dc.domain.Routes(item)
			assert.NotEmpty(t, routes,
				"domain %q must produce routes for a fully-classified content item; "+
					"check that the item fixture matches this domain's entry conditions", dc.name)
		})
	}
}
