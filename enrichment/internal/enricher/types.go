package enricher

import (
	"context"

	"github.com/jonesrussell/north-cloud/enrichment/internal/api"
)

const (
	TypeCompanyIntel = "company_intel"
	TypeTechStack    = "tech_stack"
	TypeHiring       = "hiring"

	StatusSuccess = "success"
	StatusEmpty   = "empty"
	StatusError   = "error"
	StatusSkipped = "skipped"
)

// Enricher runs one enrichment strategy for a validated request.
type Enricher interface {
	Type() string
	Enrich(ctx context.Context, request api.EnrichmentRequest) (Result, error)
}

// Result is the enrichment output produced by one enricher.
type Result struct {
	LeadID     string         `json:"lead_id"`
	Type       string         `json:"type"`
	Status     string         `json:"status"`
	Confidence float64        `json:"confidence"`
	Data       map[string]any `json:"data,omitempty"`
	Error      string         `json:"error,omitempty"`
}

func emptyResult(request api.EnrichmentRequest, enrichmentType string) Result {
	return Result{
		LeadID:     request.LeadID,
		Type:       enrichmentType,
		Status:     StatusEmpty,
		Confidence: emptyConfidence,
		Data:       map[string]any{"reason": "no supporting evidence found"},
	}
}

func errorResult(request api.EnrichmentRequest, enrichmentType string, err error) Result {
	return Result{
		LeadID: request.LeadID,
		Type:   enrichmentType,
		Status: StatusError,
		Error:  err.Error(),
	}
}

// UnknownResult returns a skipped result for an unsupported enrichment type.
func UnknownResult(request api.EnrichmentRequest, enrichmentType string) Result {
	return Result{
		LeadID: request.LeadID,
		Type:   enrichmentType,
		Status: StatusSkipped,
		Data:   map[string]any{"reason": "unknown enrichment type"},
	}
}
