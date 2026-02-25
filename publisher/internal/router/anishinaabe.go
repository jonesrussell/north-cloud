package router

import "strings"

// Anishinaabe relevance constants.
const (
	AnishinaabeRelevanceNot        = "not_anishinaabe"
	AnishinaabeRelevancePeripheral = "peripheral_anishinaabe"
	AnishinaabeRelevanceCore       = "core_anishinaabe"
)

// AnishinaabeeDomain routes Anishinaabe-classified content items to anishinaabe:* channels.
// Channels produced:
//   - content:anishinaabe (catch-all: all core + peripheral)
//   - anishinaabe:category:{slug} (per category, e.g. anishinaabe:category:culture)
type AnishinaabeeDomain struct{}

// NewAnishinaabeeDomain creates an AnishinaabeeDomain.
func NewAnishinaabeeDomain() *AnishinaabeeDomain { return &AnishinaabeeDomain{} }

// Name returns the domain identifier.
func (d *AnishinaabeeDomain) Name() string { return "anishinaabe" }

// Routes returns Anishinaabe channels for the content item. Returns nil if the item
// is not Anishinaabe-classified.
func (d *AnishinaabeeDomain) Routes(item *ContentItem) []ChannelRoute {
	if item.Anishinaabe == nil {
		return nil
	}

	rel := item.Anishinaabe.Relevance
	if rel == AnishinaabeRelevanceNot || rel == "" {
		return nil
	}

	channels := []string{"content:anishinaabe"}

	for _, cat := range item.Anishinaabe.Categories {
		slug := strings.ToLower(strings.ReplaceAll(cat, " ", "-"))
		if slug != "" {
			channels = append(channels, "anishinaabe:category:"+slug)
		}
	}

	return channelRoutesFromSlice(channels)
}
