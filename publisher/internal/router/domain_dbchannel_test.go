// publisher/internal/router/domain_dbchannel_test.go
package router_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
	"github.com/jonesrussell/north-cloud/publisher/internal/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBChannelDomain_Name(t *testing.T) {
	assert.Equal(t, "db_channel", router.NewDBChannelDomain(nil).Name())
}

func TestDBChannelDomain_Routes(t *testing.T) {
	crimeChannel := models.Channel{
		ID:           uuid.New(),
		RedisChannel: "content:crime:all",
		Rules: models.Rules{
			IncludeTopics:   []string{"violent_crime", "property_crime"},
			MinQualityScore: 50,
			ContentTypes:    []string{"article"},
		},
		Enabled: true,
	}
	premiumChannel := models.Channel{
		ID:           uuid.New(),
		RedisChannel: "content:premium",
		Rules:        models.Rules{MinQualityScore: 80},
		Enabled:      true,
	}
	disabledChannel := models.Channel{
		ID:           uuid.New(),
		RedisChannel: "content:disabled",
		Rules:        models.Rules{MinQualityScore: 0},
		Enabled:      false,
	}

	tests := []struct {
		name             string
		article          *router.ContentItem
		channels         []models.Channel
		expectedChannels []string
		expectChannelIDs bool
	}{
		{
			name: "matching channel produces route with ChannelID set",
			article: &router.ContentItem{
				Topics:       []string{"violent_crime"},
				QualityScore: 75,
				ContentType:  "article",
			},
			channels:         []models.Channel{crimeChannel},
			expectedChannels: []string{"content:crime:all"},
			expectChannelIDs: true,
		},
		{
			name: "no match returns nil",
			article: &router.ContentItem{
				Topics:       []string{"technology"},
				QualityScore: 40,
				ContentType:  "article",
			},
			channels:         []models.Channel{crimeChannel, premiumChannel},
			expectedChannels: nil,
		},
		{
			name: "multiple matching channels",
			article: &router.ContentItem{
				Topics:       []string{"violent_crime"},
				QualityScore: 90,
				ContentType:  "article",
			},
			channels:         []models.Channel{crimeChannel, premiumChannel},
			expectedChannels: []string{"content:crime:all", "content:premium"},
			expectChannelIDs: true,
		},
		{
			name:             "nil channel list returns nil",
			article:          &router.ContentItem{Topics: []string{"news"}},
			channels:         nil,
			expectedChannels: nil,
		},
		{
			name: "disabled channel is not matched",
			article: &router.ContentItem{
				Topics:       []string{"news"},
				QualityScore: 90,
				ContentType:  "article",
			},
			channels:         []models.Channel{disabledChannel},
			expectedChannels: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			domain := router.NewDBChannelDomain(tc.channels)
			routes := domain.Routes(tc.article)

			if tc.expectedChannels == nil {
				assert.Nil(t, routes)
				return
			}

			require.Len(t, routes, len(tc.expectedChannels))
			for i, r := range routes {
				assert.Equal(t, tc.expectedChannels[i], r.Channel)
				if tc.expectChannelIDs {
					// tc.channels[i] aligns with routes[i] because Routes preserves
					// channel slice order and all channels in expectChannelIDs cases fully match.
					require.NotNil(t, r.ChannelID, "ChannelID must be set by DBChannelDomain")
					assert.Equal(t, tc.channels[i].ID, *r.ChannelID,
						"ChannelID must match the source channel's ID")
				}
			}
		})
	}
}
