// publisher/internal/router/crime.go
package router

import (
	"fmt"
	"strings"
)

// Crime relevance constants.
const (
	CrimeRelevanceNotCrime   = "not_crime"
	CrimeRelevancePeripheral = "peripheral_crime"
	CrimeRelevanceCoreStreet = "core_street_crime"
	SubLabelCriminalJustice  = "criminal_justice"
	SubLabelCrimeContext     = "crime_context"
	LocationCountryCanada    = "canada"
	LocationCountryUnknown   = "unknown"
	LocationSpecificityCity  = "city"
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

// GenerateLocationChannels creates geographic channels based on article location.
// Returns channels for:
// - crime:local:{city} for city-specific Canadian content
// - crime:province:{code} for province-level Canadian content
// - crime:canada for national Canadian content
// - crime:international for non-Canadian content
func GenerateLocationChannels(article *Article) []string {
	channels := make([]string, 0)

	// Skip unknown or empty locations
	if article.LocationCountry == LocationCountryUnknown || article.LocationCountry == "" {
		return channels
	}

	// International (non-Canadian)
	if article.LocationCountry != LocationCountryCanada {
		return []string{"crime:international"}
	}

	// Canadian locations - build from most specific to least specific
	if article.LocationSpecificity == LocationSpecificityCity && article.LocationCity != "" {
		channels = append(channels, fmt.Sprintf("crime:local:%s", article.LocationCity))
	}

	if article.LocationProvince != "" {
		channels = append(channels, fmt.Sprintf("crime:province:%s", strings.ToLower(article.LocationProvince)))
	}

	channels = append(channels, "crime:canada")

	return channels
}
