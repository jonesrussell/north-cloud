package router_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/publisher/internal/models"
	"github.com/jonesrussell/north-cloud/publisher/internal/router"
	"github.com/stretchr/testify/assert"
)


func TestGenerateLayer1Channels(t *testing.T) {
	t.Helper()

	testCases := []struct {
		name     string
		article  *router.Article
		expected []string
	}{
		{
			name: "generates channels for multiple topics",
			article: &router.Article{
				Topics: []string{"violent_crime", "local_news"},
			},
			expected: []string{"articles:violent_crime", "articles:local_news"},
		},
		{
			name: "generates single channel for single topic",
			article: &router.Article{
				Topics: []string{"property_crime"},
			},
			expected: []string{"articles:property_crime"},
		},
		{
			name: "returns empty for no topics",
			article: &router.Article{
				Topics: []string{},
			},
			expected: []string{},
		},
		{
			name: "mining topic skipped",
			article: &router.Article{
				Topics: []string{"news", "mining", "technology"},
			},
			expected: []string{"articles:news", "articles:technology"},
		},
		{
			name: "mining-only produces empty",
			article: &router.Article{
				Topics: []string{"mining"},
			},
			expected: []string{},
		},
		{
			name: "mining mixed with others excludes only mining",
			article: &router.Article{
				Topics: []string{"mining", "violent_crime"},
			},
			expected: []string{"articles:violent_crime"},
		},
		{
			name: "anishinaabe topic skipped",
			article: &router.Article{
				Topics: []string{"news", "anishinaabe", "local_news"},
			},
			expected: []string{"articles:news", "articles:local_news"},
		},
		{
			name: "anishinaabe-only produces empty",
			article: &router.Article{
				Topics: []string{"anishinaabe"},
			},
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			routes := router.NewTopicDomain().Routes(tc.article)
			channels := channelNames(routes)

			assert.Len(t, channels, len(tc.expected))
			for i, expected := range tc.expected {
				assert.Equal(t, expected, channels[i])
			}
		})
	}
}

func TestRulesMatches(t *testing.T) {
	t.Helper()

	testCases := []struct {
		name         string
		rules        models.Rules
		qualityScore int
		contentType  string
		topics       []string
		expected     bool
	}{
		{
			name:         "empty rules match everything",
			rules:        models.Rules{},
			qualityScore: 50,
			contentType:  "article",
			topics:       []string{"news"},
			expected:     true,
		},
		{
			name: "matches when quality score meets minimum",
			rules: models.Rules{
				MinQualityScore: 60,
			},
			qualityScore: 75,
			contentType:  "article",
			topics:       []string{"news"},
			expected:     true,
		},
		{
			name: "no match when quality score below minimum",
			rules: models.Rules{
				MinQualityScore: 60,
			},
			qualityScore: 40,
			contentType:  "article",
			topics:       []string{"news"},
			expected:     false,
		},
		{
			name: "matches when topic is included",
			rules: models.Rules{
				IncludeTopics: []string{"violent_crime", "property_crime"},
			},
			qualityScore: 50,
			contentType:  "article",
			topics:       []string{"violent_crime", "local_news"},
			expected:     true,
		},
		{
			name: "no match when topic not included",
			rules: models.Rules{
				IncludeTopics: []string{"violent_crime", "property_crime"},
			},
			qualityScore: 50,
			contentType:  "article",
			topics:       []string{"local_news"},
			expected:     false,
		},
		{
			name: "no match when topic is excluded",
			rules: models.Rules{
				IncludeTopics: []string{"violent_crime"},
				ExcludeTopics: []string{"criminal_justice"},
			},
			qualityScore: 50,
			contentType:  "article",
			topics:       []string{"violent_crime", "criminal_justice"},
			expected:     false,
		},
		{
			name: "matches content type",
			rules: models.Rules{
				ContentTypes: []string{"article"},
			},
			qualityScore: 50,
			contentType:  "article",
			topics:       []string{"news"},
			expected:     true,
		},
		{
			name: "no match wrong content type",
			rules: models.Rules{
				ContentTypes: []string{"article"},
			},
			qualityScore: 50,
			contentType:  "page",
			topics:       []string{"news"},
			expected:     false,
		},
		{
			name: "complex rules all match",
			rules: models.Rules{
				IncludeTopics:   []string{"violent_crime", "property_crime", "drug_crime"},
				ExcludeTopics:   []string{"criminal_justice"},
				MinQualityScore: 50,
				ContentTypes:    []string{"article"},
			},
			qualityScore: 75,
			contentType:  "article",
			topics:       []string{"violent_crime", "local_news"},
			expected:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.rules.Matches(tc.qualityScore, tc.contentType, tc.topics)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// channelNames extracts Channel strings from a []router.ChannelRoute.
func channelNames(routes []router.ChannelRoute) []string {
	names := make([]string, len(routes))
	for i, r := range routes {
		names[i] = r.Channel
	}
	return names
}
