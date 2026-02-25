package router

import "strings"

// Entertainment relevance constants.
const (
	EntertainmentRelevanceNot        = "not_entertainment"
	EntertainmentRelevancePeripheral = "peripheral_entertainment"
	EntertainmentRelevanceCore       = "core_entertainment"
)

// EntertainmentDomain routes entertainment-classified content items to entertainment:* channels.
// Core + homepage eligible → entertainment:homepage; each category → entertainment:category:{slug};
// peripheral → entertainment:peripheral.
type EntertainmentDomain struct{}

// NewEntertainmentDomain creates an EntertainmentDomain.
func NewEntertainmentDomain() *EntertainmentDomain { return &EntertainmentDomain{} }

// Name returns the domain identifier.
func (d *EntertainmentDomain) Name() string { return "entertainment" }

// Routes returns entertainment channels for the content item. Returns nil if the item
// is not entertainment-classified.
func (d *EntertainmentDomain) Routes(item *ContentItem) []ChannelRoute {
	if item.Entertainment == nil {
		return nil
	}

	rel := item.Entertainment.Relevance
	if rel == EntertainmentRelevanceNot || rel == "" {
		return nil
	}

	var channels []string
	if rel == EntertainmentRelevanceCore && item.Entertainment.HomepageEligible {
		channels = append(channels, "entertainment:homepage")
	}
	for _, cat := range item.Entertainment.Categories {
		slug := strings.ToLower(strings.ReplaceAll(cat, " ", "-"))
		if slug != "" {
			channels = append(channels, "entertainment:category:"+slug)
		}
	}
	if rel == EntertainmentRelevancePeripheral {
		channels = append(channels, "entertainment:peripheral")
	}

	return channelRoutesFromSlice(channels)
}
