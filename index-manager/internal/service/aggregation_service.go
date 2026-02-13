package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jonesrussell/north-cloud/index-manager/internal/domain"
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	topCitiesLimit     = 10
	topCrimeTypesLimit = 10
	qualityHighMin     = 70
	qualityMediumMin   = 40
	maxSourceBuckets   = 500

	rawContentSuffix        = "_raw_content"
	classifiedContentSuffix = "_classified_content"
)

// AggregationService provides aggregation operations on classified content
type AggregationService struct {
	esClient     *elasticsearch.Client
	queryBuilder *elasticsearch.DocumentQueryBuilder
	logger       infralogger.Logger
}

// NewAggregationService creates a new aggregation service
func NewAggregationService(esClient *elasticsearch.Client, logger infralogger.Logger) *AggregationService {
	return &AggregationService{
		esClient:     esClient,
		queryBuilder: elasticsearch.NewDocumentQueryBuilder(),
		logger:       logger,
	}
}

// GetCrimeAggregation returns crime distribution statistics
func (s *AggregationService) GetCrimeAggregation(
	ctx context.Context,
	req *domain.AggregationRequest,
) (*domain.CrimeAggregation, error) {
	query := s.buildAggregationQuery(req, map[string]any{
		"by_sub_label": map[string]any{
			"terms": map[string]any{
				"field": "crime.sub_label",
				"size":  topCitiesLimit,
			},
		},
		"by_relevance": map[string]any{
			"terms": map[string]any{
				"field": "crime.relevance",
				"size":  topCitiesLimit,
			},
		},
		"by_crime_type": map[string]any{
			"terms": map[string]any{
				"field": "crime.crime_types",
				"size":  topCrimeTypesLimit,
			},
		},
		"crime_related": map[string]any{
			"filter": map[string]any{
				"terms": map[string]any{
					"crime.relevance": []string{"core_street_crime", "peripheral_crime"},
				},
			},
		},
	})

	res, err := s.esClient.SearchAllClassifiedContent(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute aggregation: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	var esResp aggregationResponse
	if decodeErr := json.NewDecoder(res.Body).Decode(&esResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	return &domain.CrimeAggregation{
		BySubLabel:        extractBuckets(esResp.Aggregations["by_sub_label"]),
		ByRelevance:       extractBuckets(esResp.Aggregations["by_relevance"]),
		ByCrimeType:       extractBuckets(esResp.Aggregations["by_crime_type"]),
		TotalCrimeRelated: extractFilterCount(esResp.Aggregations["crime_related"]),
		TotalDocuments:    esResp.Hits.Total.Value,
	}, nil
}

// GetLocationAggregation returns geographic distribution statistics
func (s *AggregationService) GetLocationAggregation(
	ctx context.Context,
	req *domain.AggregationRequest,
) (*domain.LocationAggregation, error) {
	query := s.buildAggregationQuery(req, map[string]any{
		"by_country": map[string]any{
			"terms": map[string]any{
				"field": "location.country",
				"size":  topCitiesLimit,
			},
		},
		"by_province": map[string]any{
			"terms": map[string]any{
				"field": "location.province",
				"size":  topCitiesLimit,
			},
		},
		"by_city": map[string]any{
			"terms": map[string]any{
				"field": "location.city",
				"size":  topCitiesLimit,
			},
		},
		"by_specificity": map[string]any{
			"terms": map[string]any{
				"field": "location.specificity",
				"size":  topCitiesLimit,
			},
		},
	})

	res, err := s.esClient.SearchAllClassifiedContent(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute aggregation: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	var esResp aggregationResponse
	if decodeErr := json.NewDecoder(res.Body).Decode(&esResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	return &domain.LocationAggregation{
		ByCountry:     extractBuckets(esResp.Aggregations["by_country"]),
		ByProvince:    extractBuckets(esResp.Aggregations["by_province"]),
		ByCity:        extractBuckets(esResp.Aggregations["by_city"]),
		BySpecificity: extractBuckets(esResp.Aggregations["by_specificity"]),
	}, nil
}

// GetOverviewAggregation returns high-level pipeline statistics
func (s *AggregationService) GetOverviewAggregation(
	ctx context.Context,
	req *domain.AggregationRequest,
) (*domain.OverviewAggregation, error) {
	query := s.buildAggregationQuery(req, map[string]any{
		"top_cities": map[string]any{
			"terms": map[string]any{
				"field": "location.city",
				"size":  topCitiesLimit,
			},
		},
		"top_crime_types": map[string]any{
			"terms": map[string]any{
				"field": "crime.crime_types",
				"size":  topCrimeTypesLimit,
			},
		},
		"crime_related": map[string]any{
			"filter": map[string]any{
				"terms": map[string]any{
					"crime.relevance": []string{"core_street_crime", "peripheral_crime"},
				},
			},
		},
		"quality_high": map[string]any{
			"filter": map[string]any{
				"range": map[string]any{
					"quality_score": map[string]any{"gte": qualityHighMin},
				},
			},
		},
		"quality_medium": map[string]any{
			"filter": map[string]any{
				"range": map[string]any{
					"quality_score": map[string]any{"gte": qualityMediumMin, "lt": qualityHighMin},
				},
			},
		},
		"quality_low": map[string]any{
			"filter": map[string]any{
				"range": map[string]any{
					"quality_score": map[string]any{"lt": qualityMediumMin},
				},
			},
		},
	})

	res, err := s.esClient.SearchAllClassifiedContent(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute aggregation: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	var esResp aggregationResponse
	if decodeErr := json.NewDecoder(res.Body).Decode(&esResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	return &domain.OverviewAggregation{
		TotalDocuments:    esResp.Hits.Total.Value,
		TotalCrimeRelated: extractFilterCount(esResp.Aggregations["crime_related"]),
		TopCities:         extractBucketKeys(esResp.Aggregations["top_cities"]),
		TopCrimeTypes:     extractBucketKeys(esResp.Aggregations["top_crime_types"]),
		QualityDistribution: domain.QualityBuckets{
			High:   extractFilterCount(esResp.Aggregations["quality_high"]),
			Medium: extractFilterCount(esResp.Aggregations["quality_medium"]),
			Low:    extractFilterCount(esResp.Aggregations["quality_low"]),
		},
	}, nil
}

// GetMiningAggregation returns mining distribution statistics
func (s *AggregationService) GetMiningAggregation(
	ctx context.Context,
	req *domain.AggregationRequest,
) (*domain.MiningAggregation, error) {
	query := s.buildAggregationQuery(req, map[string]any{
		"by_relevance": map[string]any{
			"terms": map[string]any{
				"field": "mining.relevance",
				"size":  topCitiesLimit,
			},
		},
		"by_mining_stage": map[string]any{
			"terms": map[string]any{
				"field": "mining.mining_stage",
				"size":  topCitiesLimit,
			},
		},
		"by_commodity": map[string]any{
			"terms": map[string]any{
				"field": "mining.commodities",
				"size":  topCrimeTypesLimit,
			},
		},
		"by_location": map[string]any{
			"terms": map[string]any{
				"field": "mining.location",
				"size":  topCitiesLimit,
			},
		},
		"mining_related": map[string]any{
			"filter": map[string]any{
				"terms": map[string]any{
					"mining.relevance": []string{"core_mining", "peripheral_mining"},
				},
			},
		},
	})

	res, err := s.esClient.SearchAllClassifiedContent(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute mining aggregation: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	var esResp aggregationResponse
	if decodeErr := json.NewDecoder(res.Body).Decode(&esResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode mining aggregation response: %w", decodeErr)
	}

	return &domain.MiningAggregation{
		ByRelevance:    extractBuckets(esResp.Aggregations["by_relevance"]),
		ByMiningStage:  extractBuckets(esResp.Aggregations["by_mining_stage"]),
		ByCommodity:    extractBuckets(esResp.Aggregations["by_commodity"]),
		ByLocation:     extractBuckets(esResp.Aggregations["by_location"]),
		TotalMining:    extractFilterCount(esResp.Aggregations["mining_related"]),
		TotalDocuments: esResp.Hits.Total.Value,
	}, nil
}

// GetSourceHealth returns per-source pipeline health metrics by combining
// index doc counts with classified content aggregations.
func (s *AggregationService) GetSourceHealth(ctx context.Context) (*domain.SourceHealthResponse, error) {
	docCounts, err := s.esClient.GetAllIndexDocCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get index doc counts: %w", err)
	}

	rawCounts, classifiedCounts := s.buildSourceCountMaps(docCounts)
	sources := s.mergeSourceNames(rawCounts, classifiedCounts)

	qualityMap, deltaMap := s.fetchClassifiedAggregations(ctx)

	result := s.assembleSourceHealthList(sources, rawCounts, classifiedCounts, qualityMap, deltaMap)

	return &domain.SourceHealthResponse{
		Sources: result,
		Total:   len(result),
	}, nil
}

// buildSourceCountMaps splits index doc counts into raw and classified maps keyed by source name.
func (s *AggregationService) buildSourceCountMaps(
	docCounts []elasticsearch.IndexDocCount,
) (raw, classified map[string]int64) {
	raw = make(map[string]int64)
	classified = make(map[string]int64)

	for _, idx := range docCounts {
		switch {
		case len(idx.Name) > len(rawContentSuffix) &&
			idx.Name[len(idx.Name)-len(rawContentSuffix):] == rawContentSuffix:
			source := idx.Name[:len(idx.Name)-len(rawContentSuffix)]
			raw[source] = idx.DocCount
		case len(idx.Name) > len(classifiedContentSuffix) &&
			idx.Name[len(idx.Name)-len(classifiedContentSuffix):] == classifiedContentSuffix:
			source := idx.Name[:len(idx.Name)-len(classifiedContentSuffix)]
			classified[source] = idx.DocCount
		}
	}

	return raw, classified
}

// mergeSourceNames returns a deduplicated sorted slice of source names found in either map.
func (s *AggregationService) mergeSourceNames(raw, classified map[string]int64) []string {
	seen := make(map[string]bool, len(raw)+len(classified))
	for k := range raw {
		seen[k] = true
	}
	for k := range classified {
		seen[k] = true
	}

	sources := make([]string, 0, len(seen))
	for k := range seen {
		sources = append(sources, k)
	}

	return sources
}

// fetchClassifiedAggregations queries classified content for per-source avg quality and 24h delta.
func (s *AggregationService) fetchClassifiedAggregations(
	ctx context.Context,
) (qualityMap map[string]float64, deltaMap map[string]int64) {
	qualityMap = make(map[string]float64)
	deltaMap = make(map[string]int64)

	query := map[string]any{
		"size": 0,
		"aggs": map[string]any{
			"by_source": map[string]any{
				"terms": map[string]any{
					"field": "source_name.keyword",
					"size":  maxSourceBuckets,
				},
				"aggs": map[string]any{
					"avg_quality": map[string]any{
						"avg": map[string]any{"field": "quality_score"},
					},
					"recent_24h": map[string]any{
						"filter": map[string]any{
							"range": map[string]any{
								"classified_at": map[string]any{"gte": "now-24h"},
							},
						},
					},
				},
			},
		},
	}

	res, err := s.esClient.SearchAllClassifiedContent(ctx, query)
	if err != nil {
		s.logger.Warn("failed to fetch classified aggregations for source health",
			infralogger.Error(err))
		return qualityMap, deltaMap
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		s.logger.Warn("ES error in source health aggregation",
			infralogger.Int("status_code", res.StatusCode))
		return qualityMap, deltaMap
	}

	var esResp sourceHealthAggResponse
	if decodeErr := json.NewDecoder(res.Body).Decode(&esResp); decodeErr != nil {
		s.logger.Warn("failed to decode source health aggregation response",
			infralogger.Error(decodeErr))
		return qualityMap, deltaMap
	}

	for _, bucket := range esResp.Aggregations.BySource.Buckets {
		if bucket.AvgQuality.Value != nil {
			qualityMap[bucket.Key] = *bucket.AvgQuality.Value
		}
		deltaMap[bucket.Key] = bucket.Recent24h.DocCount
	}

	return qualityMap, deltaMap
}

// assembleSourceHealthList combines all data into the final SourceHealth slice.
func (s *AggregationService) assembleSourceHealthList(
	sources []string,
	rawCounts, classifiedCounts map[string]int64,
	qualityMap map[string]float64,
	deltaMap map[string]int64,
) []domain.SourceHealth {
	result := make([]domain.SourceHealth, 0, len(sources))
	for _, source := range sources {
		raw := rawCounts[source]
		classified := classifiedCounts[source]
		backlog := max(raw-classified, 0)
		result = append(result, domain.SourceHealth{
			Source:          source,
			RawCount:        raw,
			ClassifiedCount: classified,
			Backlog:         backlog,
			Delta24h:        deltaMap[source],
			AvgQuality:      qualityMap[source],
		})
	}
	return result
}

// sourceHealthAggResponse is the typed ES response for the source health aggregation query.
type sourceHealthAggResponse struct {
	Aggregations struct {
		BySource struct {
			Buckets []struct {
				Key        string `json:"key"`
				AvgQuality struct {
					Value *float64 `json:"value"`
				} `json:"avg_quality"`
				Recent24h struct {
					DocCount int64 `json:"doc_count"`
				} `json:"recent_24h"`
			} `json:"buckets"`
		} `json:"by_source"`
	} `json:"aggregations"`
}

// buildAggregationQuery constructs an ES aggregation query with optional filters
func (s *AggregationService) buildAggregationQuery(
	req *domain.AggregationRequest,
	aggs map[string]any,
) map[string]any {
	query := map[string]any{
		"size":             0,
		"track_total_hits": true,
		"aggs":             aggs,
	}

	// Add filters if provided
	if req != nil && req.Filters != nil {
		// Use query builder to construct filter query
		boolQuery := map[string]any{
			"filter": s.queryBuilder.BuildFiltersOnly(req.Filters),
		}
		query["query"] = map[string]any{"bool": boolQuery}
	}

	return query
}

// aggregationResponse represents the ES aggregation response structure
type aggregationResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
	} `json:"hits"`
	Aggregations map[string]json.RawMessage `json:"aggregations"`
}

// bucketAggResult represents a terms aggregation result
type bucketAggResult struct {
	Buckets []struct {
		Key      string `json:"key"`
		DocCount int64  `json:"doc_count"`
	} `json:"buckets"`
}

// filterAggResult represents a filter aggregation result
type filterAggResult struct {
	DocCount int64 `json:"doc_count"`
}

// extractBuckets extracts key-count pairs from a terms aggregation
func extractBuckets(raw json.RawMessage) map[string]int64 {
	result := make(map[string]int64)
	var agg bucketAggResult
	if err := json.Unmarshal(raw, &agg); err != nil {
		return result
	}
	for _, bucket := range agg.Buckets {
		result[bucket.Key] = bucket.DocCount
	}
	return result
}

// extractBucketKeys extracts just the keys from a terms aggregation
func extractBucketKeys(raw json.RawMessage) []string {
	var agg bucketAggResult
	if err := json.Unmarshal(raw, &agg); err != nil {
		return nil
	}
	keys := make([]string, 0, len(agg.Buckets))
	for _, bucket := range agg.Buckets {
		keys = append(keys, bucket.Key)
	}
	return keys
}

// extractFilterCount extracts doc_count from a filter aggregation
func extractFilterCount(raw json.RawMessage) int64 {
	var agg filterAggResult
	if err := json.Unmarshal(raw, &agg); err != nil {
		return 0
	}
	return agg.DocCount
}
