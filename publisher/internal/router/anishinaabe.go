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
