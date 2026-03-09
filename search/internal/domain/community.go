package domain

// CommunitySearchResponse is the response for community autocomplete/search.
// Consumed by Minoo's CommunityAutocompleteClient which expects a "hits" array.
type CommunitySearchResponse struct {
	Hits []CommunityHit `json:"hits"`
}

// CommunityHit represents a single community search result.
type CommunityHit struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	CommunityType string `json:"community_type"`
	Province      string `json:"province"`
}
