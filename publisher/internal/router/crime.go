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

// CrimeDomain routes crime-classified content items to crime:* channels.
// Routes channels:
// - crime:homepage (if HomepageEligible is true for core_street_crime)
// - crime:category:{category} for each category page (core_street_crime)
// - crime:courts (peripheral_crime with criminal_justice sub-label)
// - crime:context (peripheral_crime with crime_context sub-label)
type CrimeDomain struct{}

// NewCrimeDomain creates a CrimeDomain.
func NewCrimeDomain() *CrimeDomain { return &CrimeDomain{} }

// Name returns the domain identifier used in routing decision logs.
func (d *CrimeDomain) Name() string { return "crime" }

// Routes returns crime channels for the content item. Returns nil if the item
// is not crime-classified.
func (d *CrimeDomain) Routes(item *ContentItem) []ChannelRoute {
	// Skip non-crime content
	if item.CrimeRelevance == CrimeRelevanceNotCrime || item.CrimeRelevance == "" {
		return nil
	}

	channels := make([]string, 0)

	// Handle peripheral_crime with sub-labels
	if item.CrimeRelevance == CrimeRelevancePeripheral {
		switch item.CrimeSubLabel {
		case SubLabelCriminalJustice:
			channels = append(channels, "crime:courts")
		case SubLabelCrimeContext:
			channels = append(channels, "crime:context")
		default:
			// Default to context if no sub-label
			channels = append(channels, "crime:context")
		}
		return channelRoutesFromSlice(channels)
	}

	// Handle core_street_crime
	// Homepage channel if eligible
	if item.HomepageEligible {
		channels = append(channels, "crime:homepage")
	}

	// Category channels
	for _, category := range item.CategoryPages {
		channels = append(channels, fmt.Sprintf("crime:category:%s", category))
	}

	return channelRoutesFromSlice(channels)
}
