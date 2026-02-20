package router

import "strings"

// Anishinaabe relevance constants.
const (
	AnishinaabeRelevanceNot        = "not_anishinaabe"
	AnishinaabeRelevancePeripheral = "peripheral_anishinaabe"
	AnishinaabeRelevanceCore       = "core_anishinaabe"
)

// GenerateAnishinaabeChannels returns the Redis channels for articles with Anishinaabe classification.
// Channels generated:
//   - articles:anishinaabe (catch-all: all core + peripheral)
//   - anishinaabe:category:{slug} (per category, e.g. anishinaabe:category:culture)
func GenerateAnishinaabeChannels(article *Article) []string {
	if article.Anishinaabe == nil {
		return nil
	}

	rel := article.Anishinaabe.Relevance
	if rel == AnishinaabeRelevanceNot || rel == "" {
		return nil
	}

	channels := []string{"articles:anishinaabe"}

	for _, cat := range article.Anishinaabe.Categories {
		slug := strings.ToLower(strings.ReplaceAll(cat, " ", "-"))
		if slug != "" {
			channels = append(channels, "anishinaabe:category:"+slug)
		}
	}

	return channels
}

// AnishinaabeeDomain routes Anishinaabe-classified articles to anishinaabe:* channels.
type AnishinaabeeDomain struct{}

// NewAnishinaabeeDomain creates an AnishinaabeeDomain.
func NewAnishinaabeeDomain() *AnishinaabeeDomain { return &AnishinaabeeDomain{} }

// Name returns the domain identifier.
func (d *AnishinaabeeDomain) Name() string { return "anishinaabe" }

// Routes returns Anishinaabe channels for the article. Returns nil if the article
// is not Anishinaabe-classified. Delegates to GenerateAnishinaabeChannels.
func (d *AnishinaabeeDomain) Routes(a *Article) []ChannelRoute {
	return channelRoutesFromSlice(GenerateAnishinaabeChannels(a))
}
