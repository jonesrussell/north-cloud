package router

import "strings"

// RFPDomain routes RFP-classified content to rfp:* channels.
// Channels produced:
//   - content:rfps (catch-all)
//   - rfp:country:{code} (per country)
//   - rfp:province:{code} (per province)
//   - rfp:sector:{slug} (per category)
//   - rfp:type:{slug} (per procurement type)
type RFPDomain struct{}

// NewRFPDomain creates an RFPDomain.
func NewRFPDomain() *RFPDomain { return &RFPDomain{} }

// Name returns the domain identifier.
func (d *RFPDomain) Name() string { return "rfp" }

// Routes returns RFP channels for the content item.
func (d *RFPDomain) Routes(item *ContentItem) []ChannelRoute {
	if item.RFP == nil {
		return nil
	}

	channels := []string{"content:rfps"}

	if item.RFP.Country != "" {
		channels = append(channels, "rfp:country:"+strings.ToLower(item.RFP.Country))
	}

	if item.RFP.Province != "" {
		channels = append(channels, "rfp:province:"+strings.ToLower(item.RFP.Province))
	}

	for _, category := range item.RFP.Categories {
		slug := strings.ToLower(strings.ReplaceAll(category, " ", "-"))
		channels = append(channels, "rfp:sector:"+slug)
	}

	if item.RFP.ProcurementType != "" {
		slug := strings.ToLower(strings.ReplaceAll(item.RFP.ProcurementType, " ", "-"))
		channels = append(channels, "rfp:type:"+slug)
	}

	return channelRoutesFromSlice(channels)
}

// compile-time interface check
var _ RoutingDomain = (*RFPDomain)(nil)
