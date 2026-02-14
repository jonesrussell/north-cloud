// publisher/internal/router/location.go
package router

import (
	"fmt"
	"strings"
)

// Location constants.
const (
	LocationCountryCanada   = "canada"
	LocationCountryUnknown  = "unknown"
	LocationSpecificityCity = "city"
)

// GenerateLocationChannels creates geographic channels based on article location.
// For each active domain classifier (crime, entertainment), generates:
//   - {prefix}:local:{city} for city-specific Canadian content
//   - {prefix}:province:{code} for province-level Canadian content
//   - {prefix}:canada for national Canadian content
//   - {prefix}:international for non-Canadian content
func GenerateLocationChannels(article *Article) []string {
	// Skip unknown or empty locations
	if article.LocationCountry == LocationCountryUnknown || article.LocationCountry == "" {
		return nil
	}

	prefixes := activeTopicPrefixes(article)
	if len(prefixes) == 0 {
		return nil
	}

	// International (non-Canadian) — one channel per prefix
	if article.LocationCountry != LocationCountryCanada {
		channels := make([]string, 0, len(prefixes))
		for _, prefix := range prefixes {
			channels = append(channels, prefix+":international")
		}
		return channels
	}

	// Canadian locations — build from most specific to least specific per prefix
	return generateCanadianChannels(article, prefixes)
}

// generateCanadianChannels builds location channels for Canadian content.
func generateCanadianChannels(article *Article, prefixes []string) []string {
	// Estimate capacity: up to 3 channels (local, province, canada) per prefix
	const maxChannelsPerPrefix = 3
	channels := make([]string, 0, len(prefixes)*maxChannelsPerPrefix)

	for _, prefix := range prefixes {
		if article.LocationSpecificity == LocationSpecificityCity && article.LocationCity != "" {
			channels = append(channels, fmt.Sprintf("%s:local:%s", prefix, article.LocationCity))
		}
		if article.LocationProvince != "" {
			channels = append(channels, fmt.Sprintf("%s:province:%s", prefix, strings.ToLower(article.LocationProvince)))
		}
		channels = append(channels, prefix+":canada")
	}

	return channels
}

// activeTopicPrefixes returns the channel prefixes for domain classifiers
// that are active on this article. Mining is excluded because Layer 5
// (GenerateMiningChannels) already generates mining:canada/mining:international.
func activeTopicPrefixes(article *Article) []string {
	const maxPrefixes = 2
	prefixes := make([]string, 0, maxPrefixes)

	if article.CrimeRelevance != CrimeRelevanceNotCrime && article.CrimeRelevance != "" {
		prefixes = append(prefixes, "crime")
	}
	if article.Entertainment != nil && article.Entertainment.Relevance != EntertainmentRelevanceNot && article.Entertainment.Relevance != "" {
		prefixes = append(prefixes, "entertainment")
	}

	return prefixes
}
