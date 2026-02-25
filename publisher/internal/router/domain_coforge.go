package router

import "strings"

// Coforge relevance constants.
const (
	CoforgeRelevanceNotRelevant = "not_relevant"
	CoforgeRelevanceCore        = "core_coforge"
	CoforgeRelevancePeripheral  = "peripheral"
)

// CoforgeDomain routes Coforge-classified content items to coforge:* channels.
//
// Coforge is a product-specific routing domain — not a public topic domain.
// It does NOT produce a catch-all content:coforge channel. Entry points are
// coforge:core and coforge:peripheral, plus audience, topic, and industry sub-channels.
type CoforgeDomain struct{}

// NewCoforgeDomain creates a CoforgeDomain.
func NewCoforgeDomain() *CoforgeDomain { return &CoforgeDomain{} }

// Name returns the domain identifier.
func (d *CoforgeDomain) Name() string { return "coforge" }

// Routes returns Coforge channels for the content item.
// Returns nil if Coforge data is absent or relevance is not_relevant.
func (d *CoforgeDomain) Routes(item *ContentItem) []ChannelRoute {
	if item.Coforge == nil {
		return nil
	}

	rel := item.Coforge.Relevance
	if rel == CoforgeRelevanceNotRelevant || rel == "" {
		return nil
	}

	const maxCoforgeFixedChannels = 2 // relevance anchor + audience
	names := make([]string, 0, maxCoforgeFixedChannels+len(item.Coforge.Topics)+len(item.Coforge.Industries))

	// Relevance channel — coforge:core or coforge:peripheral
	switch rel {
	case CoforgeRelevanceCore:
		names = append(names, "coforge:core")
	case CoforgeRelevancePeripheral:
		names = append(names, "coforge:peripheral")
	default:
		// Unknown relevance value — return nil to prevent partial routing.
		// Known-irrelevant values are caught by the guard above.
		return nil
	}

	// Audience channel — slug-normalized (lowercase, spaces and underscores to hyphens)
	if item.Coforge.Audience != "" {
		slug := strings.ToLower(strings.ReplaceAll(
			strings.ReplaceAll(item.Coforge.Audience, "_", "-"),
			" ", "-",
		))
		names = append(names, "coforge:audience:"+slug)
	}

	// Topic channels — underscores converted to hyphens for slug format
	for _, topic := range item.Coforge.Topics {
		slug := strings.ToLower(strings.ReplaceAll(topic, "_", "-"))
		if slug != "" {
			names = append(names, "coforge:topic:"+slug)
		}
	}

	// Industry channels — underscores converted to hyphens for slug format
	for _, industry := range item.Coforge.Industries {
		slug := strings.ToLower(strings.ReplaceAll(industry, "_", "-"))
		if slug != "" {
			names = append(names, "coforge:industry:"+slug)
		}
	}

	return channelRoutesFromSlice(names)
}

// compile-time interface check
var _ RoutingDomain = (*CoforgeDomain)(nil)
