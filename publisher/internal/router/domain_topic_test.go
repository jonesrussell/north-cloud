// publisher/internal/router/domain_topic_test.go
package router_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/publisher/internal/router"
	"github.com/stretchr/testify/assert"
)

func TestTopicDomain_Name(t *testing.T) {
	assert.Equal(t, "topic", router.NewTopicDomain().Name())
}

func TestTopicDomain_Routes(t *testing.T) {
	tests := []struct {
		name     string
		topics   []string
		expected []string
	}{
		{
			name:     "multiple topics produce content: channels",
			topics:   []string{"violent_crime", "local_news"},
			expected: []string{"content:violent_crime", "content:local_news"},
		},
		{
			name:     "empty topics produce no channels",
			topics:   []string{},
			expected: nil,
		},
		{
			name:     "mining topic is skipped",
			topics:   []string{"news", "mining", "technology"},
			expected: []string{"content:news", "content:technology"},
		},
		{
			name:     "anishinaabe topic is skipped",
			topics:   []string{"news", "anishinaabe"},
			expected: []string{"content:news"},
		},
		{
			name:     "coforge topic is skipped",
			topics:   []string{"news", "coforge"},
			expected: []string{"content:news"},
		},
		{
			name:     "all skip topics produce no channels",
			topics:   []string{"mining", "anishinaabe", "coforge"},
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := &router.ContentItem{Topics: tc.topics}
			routes := router.NewTopicDomain().Routes(item)
			var names []string
			for _, r := range routes {
				names = append(names, r.Channel)
			}
			assert.Equal(t, tc.expected, names)
		})
	}
}
