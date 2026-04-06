package adapter

import "context"

// Signal represents a lead signal detected by a source adapter.
// Field names and JSON tags match the NorthOps /api/signals ingest contract.
type Signal struct {
	// Required by NorthOps
	SignalType string `json:"signal_type"`
	ExternalID string `json:"external_id"`
	SourceName string `json:"source"`
	Label      string `json:"label"`

	// Optional fields
	SourceURL      string `json:"source_url,omitempty"`
	SignalStrength int    `json:"strength"`
	Sector         string `json:"sector,omitempty"`
	Notes          string `json:"notes,omitempty"`

	// Funding-specific fields (zero values for non-funding signals)
	FundingStatus    string `json:"funding_status,omitempty"`
	OrganizationType string `json:"organization_type,omitempty"`
}

// Source is the interface that all signal adapters implement.
type Source interface {
	// Name returns a short identifier for this source (e.g. "hn", "funding").
	Name() string

	// Scan fetches raw items from the source and returns scored signals.
	Scan(ctx context.Context) ([]Signal, error)
}
