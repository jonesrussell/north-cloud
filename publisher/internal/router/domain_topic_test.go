// publisher/internal/router/domain_topic_test.go
package router_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/publisher/internal/router"
	"github.com/stretchr/testify/assert"
)

func TestTopicDomain_Name(t *testing.T) {
	t.Helper()
	assert.Equal(t, "topic", router.NewTopicDomain().Name())
}

func TestTopicDomain_Routes(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		topics   []string
		expected []string
	}{
		{
			name:     "multiple topics produce articles: channels",
			topics:   []string{"violent_crime", "local_news"},
			expected: []string{"articles:violent_crime", "articles:local_news"},
		},
		{
			name:     "empty topics produce no channels",
			topics:   []string{},
			expected: nil,
		},
		{
			name:     "mining topic is skipped",
			topics:   []string{"news", "mining", "technology"},
			expected: []string{"articles:news", "articles:technology"},
		},
		{
			name:     "anishinaabe topic is skipped",
			topics:   []string{"news", "anishinaabe"},
			expected: []string{"articles:news"},
		},
		{
			name:     "coforge topic is skipped",
			topics:   []string{"news", "coforge"},
			expected: []string{"articles:news"},
		},
		{
			name:     "all skip topics produce no channels",
			topics:   []string{"mining", "anishinaabe", "coforge"},
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			article := &router.Article{Topics: tc.topics}
			routes := router.NewTopicDomain().Routes(article)
			var names []string
			for _, r := range routes {
				names = append(names, r.Channel)
			}
			assert.Equal(t, tc.expected, names)
		})
	}
}
