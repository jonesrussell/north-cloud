package enricher

import (
	"context"
	"fmt"

	"github.com/jonesrussell/north-cloud/enrichment/internal/api"
)

var companyTerms = []string{"profile", "operations", "leadership", "headquarters", "funding"}

type companyIntel struct {
	searcher Searcher
}

// NewCompanyIntel creates the company intelligence enricher.
func NewCompanyIntel(searcher Searcher) Enricher {
	return companyIntel{searcher: searcher}
}

func (e companyIntel) Type() string {
	return TypeCompanyIntel
}

func (e companyIntel) Enrich(ctx context.Context, request api.EnrichmentRequest) (Result, error) {
	hits, err := e.searcher.Search(ctx, SearchRequest{
		Indexes: searchIndexes(),
		Query:   evidenceQuery(request.CompanyName, request.Domain, request.Sector, companyTerms),
		Size:    defaultSearchSize,
	})
	if err != nil {
		result := errorResult(request, TypeCompanyIntel, fmt.Errorf("company intel search: %w", err))
		return result, err
	}
	if len(hits) == 0 {
		return emptyResult(request, TypeCompanyIntel), nil
	}

	return Result{
		LeadID:     request.LeadID,
		Type:       TypeCompanyIntel,
		Status:     StatusSuccess,
		Confidence: confidence(request, hits),
		Data: map[string]any{
			"company_name": request.CompanyName,
			"domain":       request.Domain,
			"sector":       request.Sector,
			"summary":      summarizeCompany(request, hits),
			"evidence":     evidenceItems(hits),
		},
	}, nil
}

func summarizeCompany(request api.EnrichmentRequest, hits []Hit) string {
	if len(hits) == 0 {
		return ""
	}
	title := toString(hits[0].Source["title"])
	if title == "" {
		return request.CompanyName
	}
	return title
}
