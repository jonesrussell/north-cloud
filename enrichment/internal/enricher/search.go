package enricher

import "context"

const (
	defaultSearchSize       = 5
	defaultContentIndex     = "classified_content"
	defaultSourceIndex      = "raw_content"
	emptyConfidence         = 0.1
	baseConfidence          = 0.35
	maxSearchScore          = 10.0
	scoreWeight             = 0.45
	evidenceWeight          = 0.1
	domainMatchBoost        = 0.1
	sectorMatchBoost        = 0.05
	highConfidenceThreshold = 0.95
)

// Searcher is the narrow Elasticsearch surface enrichers need.
type Searcher interface {
	Search(ctx context.Context, request SearchRequest) ([]Hit, error)
}

// SearchRequest captures an Elasticsearch query without coupling enrichers to a concrete client.
type SearchRequest struct {
	Indexes []string
	Query   map[string]any
	Size    int
}

// Hit is the subset of an Elasticsearch hit used by enrichment scoring and output.
type Hit struct {
	ID     string
	Score  float64
	Source map[string]any
}

func searchIndexes() []string {
	return []string{defaultContentIndex, defaultSourceIndex}
}

func evidenceQuery(companyName string, domain string, sector string, terms []string) map[string]any {
	should := []map[string]any{
		matchQuery("company_name", companyName),
		matchQuery("title", companyName),
		matchQuery("body", companyName),
		matchQuery("content", companyName),
	}

	if domain != "" {
		should = append(should, matchQuery("domain", domain), matchQuery("url", domain))
	}
	if sector != "" {
		should = append(should, matchQuery("sector", sector), matchQuery("body", sector))
	}
	for _, term := range terms {
		should = append(should, matchQuery("body", term), matchQuery("content", term), matchQuery("title", term))
	}

	return map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"should":               should,
				"minimum_should_match": 1,
			},
		},
		"sort": []map[string]any{
			{"_score": map[string]any{"order": "desc"}},
		},
	}
}

func matchQuery(field string, value string) map[string]any {
	return map[string]any{
		"match": map[string]any{
			field: value,
		},
	}
}
