package adapter

import "context"

// Signal represents a lead signal detected by a source adapter.
type Signal struct {
	// Common fields (all sources)
	Label          string `json:"label"`
	SourceURL      string `json:"source_url"`
	ExternalID     string `json:"-"` // Used for dedup, not sent to NorthOps
	SignalStrength int    `json:"signal_strength"`
	Sector         string `json:"sector,omitempty"`
	Notes          string `json:"notes,omitempty"`

	// Funding-specific fields (zero values for non-funding signals)
	FundingStatus    string `json:"funding_status,omitempty"`
	OrganizationType string `json:"organization_type,omitempty"`
}

// Endpoint returns the NorthOps ingest endpoint path for this signal.
// Funding signals go to /api/leads/ingest/funding, others to /api/leads/ingest/signal.
func (s Signal) Endpoint() string {
	if s.FundingStatus != "" {
		return "/api/leads/ingest/funding"
	}
	return "/api/leads/ingest/signal"
}

// Source is the interface that all signal adapters implement.
type Source interface {
	// Name returns a short identifier for this source (e.g. "hn", "funding").
	Name() string

	// Scan fetches raw items from the source and returns scored signals.
	Scan(ctx context.Context) ([]Signal, error)
}
