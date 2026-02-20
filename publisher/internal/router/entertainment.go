package router

import "strings"

// Entertainment relevance constants.
const (
	EntertainmentRelevanceNot        = "not_entertainment"
	EntertainmentRelevancePeripheral = "peripheral_entertainment"
	EntertainmentRelevanceCore       = "core_entertainment"
)

// EntertainmentDomain routes entertainment-classified articles to entertainment:* channels.
// Core + homepage eligible → entertainment:homepage; each category → entertainment:category:{slug};
// peripheral → entertainment:peripheral.
type EntertainmentDomain struct{}

// NewEntertainmentDomain creates an EntertainmentDomain.
func NewEntertainmentDomain() *EntertainmentDomain { return &EntertainmentDomain{} }

// Name returns the domain identifier.
func (d *EntertainmentDomain) Name() string { return "entertainment" }

// Routes returns entertainment channels for the article. Returns nil if the article
// is not entertainment-classified.
func (d *EntertainmentDomain) Routes(a *Article) []ChannelRoute {
	if a.Entertainment == nil {
		return nil
	}

	rel := a.Entertainment.Relevance
	if rel == EntertainmentRelevanceNot || rel == "" {
		return nil
	}

	var channels []string
	if rel == EntertainmentRelevanceCore && a.Entertainment.HomepageEligible {
		channels = append(channels, "entertainment:homepage")
	}
	for _, cat := range a.Entertainment.Categories {
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
