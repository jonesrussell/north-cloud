package router

import "strings"

// Indigenous relevance constants.
const (
	IndigenousRelevanceNot        = "not_indigenous"
	IndigenousRelevancePeripheral = "peripheral_indigenous"
	IndigenousRelevanceCore       = "core_indigenous"
)

// IndigenousDomain routes Indigenous-classified content items to indigenous:* channels.
// Channels produced:
//   - content:indigenous (catch-all: all core + peripheral)
//   - indigenous:category:{slug} (per category, e.g. indigenous:category:culture)
type IndigenousDomain struct{}

// NewIndigenousDomain creates an IndigenousDomain.
func NewIndigenousDomain() *IndigenousDomain { return &IndigenousDomain{} }

// Name returns the domain identifier.
func (d *IndigenousDomain) Name() string { return "indigenous" }

// Routes returns Indigenous channels for the content item. Returns nil if the item
// is not Indigenous-classified.
func (d *IndigenousDomain) Routes(item *ContentItem) []ChannelRoute {
	if item.Indigenous == nil {
		return nil
	}

	rel := item.Indigenous.Relevance
	if rel == IndigenousRelevanceNot || rel == "" {
		return nil
	}

	channels := []string{"content:indigenous"}

	for _, cat := range item.Indigenous.Categories {
		slug := strings.ToLower(strings.ReplaceAll(cat, " ", "-"))
		if slug != "" {
			channels = append(channels, "indigenous:category:"+slug)
		}
	}

	return channelRoutesFromSlice(channels)
}
