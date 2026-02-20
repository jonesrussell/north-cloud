package router

import "github.com/google/uuid"

// ChannelRoute represents a routing decision: a Redis channel name and an optional
// DB channel ID. ChannelID is nil for all auto-generated channels; only
// DBChannelDomain sets it (to link back to the publisher.channels table row).
type ChannelRoute struct {
	Channel   string
	ChannelID *uuid.UUID
}

// RoutingDomain is implemented by each routing layer.
// Routes returns the channels this domain produces for the given article.
// Returning nil or empty means the domain does not apply to this article.
type RoutingDomain interface {
	Name() string
	Routes(a *Article) []ChannelRoute
}

// channelRoutesFromSlice converts a slice of channel name strings to []ChannelRoute
// with nil ChannelIDs. Use this in all domains except DBChannelDomain.
func channelRoutesFromSlice(names []string) []ChannelRoute {
	if len(names) == 0 {
		return nil
	}
	routes := make([]ChannelRoute, len(names))
	for i, name := range names {
		routes[i] = ChannelRoute{Channel: name}
	}
	return routes
}
