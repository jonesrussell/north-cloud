package enricher

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jonesrussell/north-cloud/enrichment/internal/api"
)

var techTerms = []string{"technology", "platform", "software", "cloud", "crm", "erp", "analytics"}

type techStack struct {
	searcher Searcher
}

// NewTechStack creates the technology stack enricher.
func NewTechStack(searcher Searcher) Enricher {
	return techStack{searcher: searcher}
}

func (e techStack) Type() string {
	return TypeTechStack
}

func (e techStack) Enrich(ctx context.Context, request api.EnrichmentRequest) (Result, error) {
	hits, err := e.searcher.Search(ctx, SearchRequest{
		Indexes: searchIndexes(),
		Query:   evidenceQuery(request.CompanyName, request.Domain, request.Sector, techTerms),
		Size:    defaultSearchSize,
	})
	if err != nil {
		result := errorResult(request, TypeTechStack, fmt.Errorf("tech stack search: %w", err))
		return result, err
	}
	if len(hits) == 0 {
		return emptyResult(request, TypeTechStack), nil
	}

	return Result{
		LeadID:     request.LeadID,
		Type:       TypeTechStack,
		Status:     StatusSuccess,
		Confidence: confidence(request, hits),
		Data: map[string]any{
			"technologies": detectedTerms(hits, techTerms),
			"evidence":     evidenceItems(hits),
		},
	}, nil
}

func detectedTerms(hits []Hit, terms []string) []string {
	seen := make(map[string]struct{}, len(terms))
	for _, hit := range hits {
		text := strings.ToLower(toString(hit.Source["title"]) + " " + toString(hit.Source["body"]) + " " + toString(hit.Source["content"]))
		for _, term := range terms {
			if strings.Contains(text, term) {
				seen[term] = struct{}{}
			}
		}
	}

	out := make([]string, 0, len(seen))
	for term := range seen {
		out = append(out, term)
	}
	sort.Strings(out)
	return out
}
