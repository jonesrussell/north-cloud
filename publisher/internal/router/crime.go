// publisher/internal/router/crime.go
package router

import "fmt"

// Crime relevance constants.
const (
	CrimeRelevanceNotCrime   = "not_crime"
	CrimeRelevancePeripheral = "peripheral_crime"
	CrimeRelevanceCoreStreet = "core_street_crime"
	SubLabelCriminalJustice  = "criminal_justice"
	SubLabelCrimeContext     = "crime_context"
)

// GenerateCrimeChannels returns the Redis channels for articles with crime classification.
// Returns channels for:
// - crime:homepage (if HomepageEligible is true for core_street_crime)
// - crime:category:{category} for each category page (core_street_crime)
// - crime:courts (peripheral_crime with criminal_justice sub-label)
// - crime:context (peripheral_crime with crime_context sub-label)
func GenerateCrimeChannels(article *Article) []string {
	channels := make([]string, 0)

	// Skip non-crime articles
	if article.CrimeRelevance == CrimeRelevanceNotCrime || article.CrimeRelevance == "" {
		return channels
	}

	// Handle peripheral_crime with sub-labels
	if article.CrimeRelevance == CrimeRelevancePeripheral {
		switch article.CrimeSubLabel {
		case SubLabelCriminalJustice:
			channels = append(channels, "crime:courts")
		case SubLabelCrimeContext:
			channels = append(channels, "crime:context")
		default:
			// Default to context if no sub-label
			channels = append(channels, "crime:context")
		}
		return channels
	}

	// Handle core_street_crime (existing logic)
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
