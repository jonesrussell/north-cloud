package router

import (
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
)

// DBChannelDomain routes articles using database-configured custom channels (Layer 2).
// It is the only domain that produces ChannelRoute values with non-nil ChannelIDs,
// linking publish_history records back to the publisher.channels table.
type DBChannelDomain struct {
	channels []models.Channel
}

// NewDBChannelDomain creates a DBChannelDomain with the current channel configuration.
// channels should be refreshed from the database at each poll cycle.
func NewDBChannelDomain(channels []models.Channel) *DBChannelDomain {
	return &DBChannelDomain{channels: channels}
}

// Name returns the domain identifier.
func (d *DBChannelDomain) Name() string { return "db_channel" }

// Routes returns ChannelRoutes for each custom channel whose rules match the article.
// Each route carries a non-nil ChannelID referencing the publisher.channels DB row.
func (d *DBChannelDomain) Routes(a *Article) []ChannelRoute {
	routes := make([]ChannelRoute, 0, len(d.channels))
	for i := range d.channels {
		ch := &d.channels[i]
		if ch.Rules.Matches(a.QualityScore, a.ContentType, a.Topics) {
			id := ch.ID // copy to avoid loop variable address reuse
			routes = append(routes, ChannelRoute{
				Channel:   ch.RedisChannel,
				ChannelID: &id,
			})
		}
	}
	if len(routes) == 0 {
		return nil
	}
	return routes
}

// compile-time interface check
var _ RoutingDomain = (*DBChannelDomain)(nil)
