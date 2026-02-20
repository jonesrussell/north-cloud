package router

import "strings"

// Anishinaabe relevance constants.
const (
	AnishinaabeRelevanceNot        = "not_anishinaabe"
	AnishinaabeRelevancePeripheral = "peripheral_anishinaabe"
	AnishinaabeRelevanceCore       = "core_anishinaabe"
)

// AnishinaabeeDomain routes Anishinaabe-classified articles to anishinaabe:* channels.
// Channels produced:
//   - articles:anishinaabe (catch-all: all core + peripheral)
//   - anishinaabe:category:{slug} (per category, e.g. anishinaabe:category:culture)
type AnishinaabeeDomain struct{}

// NewAnishinaabeeDomain creates an AnishinaabeeDomain.
func NewAnishinaabeeDomain() *AnishinaabeeDomain { return &AnishinaabeeDomain{} }

// Name returns the domain identifier.
func (d *AnishinaabeeDomain) Name() string { return "anishinaabe" }

// Routes returns Anishinaabe channels for the article. Returns nil if the article
// is not Anishinaabe-classified.
func (d *AnishinaabeeDomain) Routes(a *Article) []ChannelRoute {
	if a.Anishinaabe == nil {
		return nil
	}

	rel := a.Anishinaabe.Relevance
	if rel == AnishinaabeRelevanceNot || rel == "" {
		return nil
	}

	channels := []string{"articles:anishinaabe"}

	for _, cat := range a.Anishinaabe.Categories {
		slug := strings.ToLower(strings.ReplaceAll(cat, " ", "-"))
		if slug != "" {
			channels = append(channels, "anishinaabe:category:"+slug)
		}
	}

	return channelRoutesFromSlice(channels)
}
