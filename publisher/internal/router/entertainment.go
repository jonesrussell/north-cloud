package router

import "strings"

// Entertainment relevance constants.
const (
	EntertainmentRelevanceNot        = "not_entertainment"
	EntertainmentRelevancePeripheral = "peripheral_entertainment"
	EntertainmentRelevanceCore       = "core_entertainment"
)

// GenerateEntertainmentChannels returns the Redis channels for articles with entertainment classification.
// Core + homepage eligible → entertainment:homepage; each category → entertainment:category:{slug};
// peripheral → entertainment:peripheral.
func GenerateEntertainmentChannels(article *Article) []string {
	if article.Entertainment == nil {
		return nil
	}

	rel := article.Entertainment.Relevance
	if rel == EntertainmentRelevanceNot || rel == "" {
		return nil
	}

	var channels []string
	if rel == EntertainmentRelevanceCore && article.Entertainment.HomepageEligible {
		channels = append(channels, "entertainment:homepage")
	}
	for _, cat := range article.Entertainment.Categories {
		slug := strings.ToLower(strings.ReplaceAll(cat, " ", "-"))
		if slug != "" {
			channels = append(channels, "entertainment:category:"+slug)
		}
	}
	if rel == EntertainmentRelevancePeripheral {
		channels = append(channels, "entertainment:peripheral")
	}

	return channels
}
