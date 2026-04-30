package enricher

import (
	"context"
	"fmt"

	"github.com/jonesrussell/north-cloud/enrichment/internal/api"
)

var hiringTerms = []string{"hiring", "jobs", "careers", "recruiting", "position", "vacancy"}

type hiring struct {
	searcher Searcher
}

// NewHiring creates the hiring intelligence enricher.
func NewHiring(searcher Searcher) Enricher {
	return hiring{searcher: searcher}
}

func (e hiring) Type() string {
	return TypeHiring
}

func (e hiring) Enrich(ctx context.Context, request api.EnrichmentRequest) (Result, error) {
	hits, err := e.searcher.Search(ctx, SearchRequest{
		Indexes: searchIndexes(),
		Query:   evidenceQuery(request.CompanyName, request.Domain, request.Sector, hiringTerms),
		Size:    defaultSearchSize,
	})
	if err != nil {
		result := errorResult(request, TypeHiring, fmt.Errorf("hiring search: %w", err))
		return result, err
	}
	if len(hits) == 0 {
		return emptyResult(request, TypeHiring), nil
	}

	return Result{
		LeadID:     request.LeadID,
		Type:       TypeHiring,
		Status:     StatusSuccess,
		Confidence: confidence(request, hits),
		Data: map[string]any{
			"hiring_signal_count": len(hits),
			"evidence":            evidenceItems(hits),
		},
	}, nil
}
