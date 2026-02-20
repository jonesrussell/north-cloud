// publisher/internal/router/domain_coforge_test.go
package router_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/publisher/internal/router"
	"github.com/stretchr/testify/assert"
)

func TestCoforgeDomain_Name(t *testing.T) {
	t.Helper()
	assert.Equal(t, "coforge", router.NewCoforgeDomain().Name())
}

func TestCoforgeDomain_Routes(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		article  *router.Article
		expected []string
	}{
		{
			name:     "nil coforge data returns nil",
			article:  &router.Article{},
			expected: nil,
		},
		{
			name: "not_relevant returns nil",
			article: &router.Article{
				Coforge: &router.CoforgeData{Relevance: "not_relevant"},
			},
			expected: nil,
		},
		{
			name: "empty relevance returns nil",
			article: &router.Article{
				Coforge: &router.CoforgeData{Relevance: ""},
			},
			expected: nil,
		},
		{
			name: "core_coforge produces coforge:core",
			article: &router.Article{
				Coforge: &router.CoforgeData{
					Relevance: "core_coforge",
					Audience:  "developer",
				},
			},
			expected: []string{"coforge:core", "coforge:audience:developer"},
		},
		{
			name: "peripheral produces coforge:peripheral",
			article: &router.Article{
				Coforge: &router.CoforgeData{
					Relevance: "peripheral",
					Audience:  "entrepreneur",
				},
			},
			expected: []string{"coforge:peripheral", "coforge:audience:entrepreneur"},
		},
		{
			name: "hybrid audience",
			article: &router.Article{
				Coforge: &router.CoforgeData{
					Relevance: "core_coforge",
					Audience:  "hybrid",
				},
			},
			expected: []string{"coforge:core", "coforge:audience:hybrid"},
		},
		{
			name: "topics produce slugified channels",
			article: &router.Article{
				Coforge: &router.CoforgeData{
					Relevance: "core_coforge",
					Audience:  "developer",
					Topics:    []string{"framework_release", "open_source"},
				},
			},
			expected: []string{
				"coforge:core",
				"coforge:audience:developer",
				"coforge:topic:framework-release",
				"coforge:topic:open-source",
			},
		},
		{
			name: "industries produce slugified channels",
			article: &router.Article{
				Coforge: &router.CoforgeData{
					Relevance:  "core_coforge",
					Audience:   "hybrid",
					Industries: []string{"ai_ml", "saas"},
				},
			},
			expected: []string{
				"coforge:core",
				"coforge:audience:hybrid",
				"coforge:industry:ai-ml",
				"coforge:industry:saas",
			},
		},
		{
			name: "full classification produces all channels",
			article: &router.Article{
				Coforge: &router.CoforgeData{
					Relevance:  "core_coforge",
					Audience:   "hybrid",
					Topics:     []string{"funding_round", "devtools"},
					Industries: []string{"saas", "ai_ml"},
				},
			},
			expected: []string{
				"coforge:core",
				"coforge:audience:hybrid",
				"coforge:topic:funding-round",
				"coforge:topic:devtools",
				"coforge:industry:saas",
				"coforge:industry:ai-ml",
			},
		},
		{
			name: "no articles:coforge catch-all is produced",
			article: &router.Article{
				Coforge: &router.CoforgeData{Relevance: "core_coforge", Audience: "developer"},
			},
			expected: []string{"coforge:core", "coforge:audience:developer"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			routes := router.NewCoforgeDomain().Routes(tc.article)
			if tc.expected == nil {
				assert.Nil(t, routes)
				return
			}
			var names []string
			for _, r := range routes {
				names = append(names, r.Channel)
			}
			assert.Equal(t, tc.expected, names)
			// Verify no catch-all channel is generated
			for _, name := range names {
				assert.NotEqual(t, "articles:coforge", name,
					"CoforgeDomain must not produce a catch-all articles:coforge channel")
			}
		})
	}
}
