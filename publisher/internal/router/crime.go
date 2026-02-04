// publisher/internal/router/crime.go
package router

import "fmt"

// Crime relevance constants.
const (
	CrimeRelevanceNotCrime = "not_crime"
)

// GenerateCrimeChannels returns the Redis channels for articles with crime classification.
// Returns channels for:
// - crime:homepage (if HomepageEligible is true)
// - crime:category:{category} for each category page
func GenerateCrimeChannels(article *Article) []string {
	channels := make([]string, 0)

	// Skip non-crime articles
	if article.CrimeRelevance == CrimeRelevanceNotCrime || article.CrimeRelevance == "" {
		return channels
	}

	// Homepage channel if eligible
	if article.HomepageEligible {
		channels = append(channels, "crime:homepage")
	}

	// Category channels
	for _, category := range article.CategoryPages {
		channels = append(channels, fmt.Sprintf("crime:category:%s", category))
	}

	return channels
}
