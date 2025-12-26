package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/jonesrussell/north-cloud/search/internal/config"
	"github.com/jonesrussell/north-cloud/search/internal/domain"
	"github.com/jonesrussell/north-cloud/search/internal/elasticsearch"
	"github.com/jonesrussell/north-cloud/search/internal/logger"
)

// SearchService orchestrates search operations
type SearchService struct {
	esClient     *elasticsearch.Client
	queryBuilder *elasticsearch.QueryBuilder
	config       *config.Config
	logger       *logger.Logger
}

// NewSearchService creates a new search service
func NewSearchService(esClient *elasticsearch.Client, cfg *config.Config, log *logger.Logger) *SearchService {
	return &SearchService{
		esClient:     esClient,
		queryBuilder: elasticsearch.NewQueryBuilder(&cfg.Elasticsearch),
		config:       cfg,
		logger:       log,
	}
}

// Search executes a search query
func (s *SearchService) Search(ctx context.Context, req *domain.SearchRequest) (*domain.SearchResponse, error) {
	startTime := time.Now()

	// Validate request
	if err := req.Validate(s.config.Service.MaxPageSize, s.config.Service.DefaultPageSize, s.config.Service.MaxQueryLength); err != nil {
		s.logger.Warn("Invalid search request", "error", err)
		return nil, fmt.Errorf("validation error: %w", err)
	}

	s.logger.Info("Executing search",
		"query", req.Query,
		"page", req.Pagination.Page,
		"size", req.Pagination.Size,
		"filters", req.Filters,
	)

	// Build Elasticsearch query
	esQuery := s.queryBuilder.Build(req)

	// Execute search
	res, err := s.executeSearch(ctx, esQuery)
	if err != nil {
		s.logger.Error("Search execution failed", "error", err, "query", req.Query)
		return nil, err
	}
	defer res.Body.Close()

	// Parse response
	response, err := s.parseSearchResponse(res.Body, req)
	if err != nil {
		s.logger.Error("Failed to parse search response", "error", err)
		return nil, err
	}

	// Calculate execution time
	response.TookMs = time.Since(startTime).Milliseconds()

	s.logger.Info("Search completed",
		"query", req.Query,
		"total_hits", response.TotalHits,
		"took_ms", response.TookMs,
	)

	return response, nil
}

// executeSearch performs the Elasticsearch search request
func (s *SearchService) executeSearch(ctx context.Context, query map[string]interface{}) (*esapi.Response, error) {
	// Marshal query to JSON
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, fmt.Errorf("failed to encode query: %w", err)
	}

	// Log query in debug mode
	if s.config.Service.Debug {
		s.logger.Debug("Elasticsearch query", "query", buf.String())
	}

	// Execute search
	esClient := s.esClient.GetESClient()
	res, err := esClient.Search(
		esClient.Search.WithContext(ctx),
		esClient.Search.WithIndex(s.config.Elasticsearch.ClassifiedContentPattern),
		esClient.Search.WithBody(&buf),
		esClient.Search.WithTimeout(s.config.Service.SearchTimeout),
		esClient.Search.WithTrackTotalHits(true),
	)

	if err != nil {
		return nil, fmt.Errorf("elasticsearch search failed: %w", err)
	}

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return nil, fmt.Errorf("elasticsearch returned error [%d]: %s", res.StatusCode, string(body))
	}

	return res, nil
}

// aggregationBucket represents a single bucket in an aggregation
type aggregationBucket struct {
	Key      interface{} `json:"key"`
	DocCount int64       `json:"doc_count"`
}

// aggregation represents an aggregation with buckets
type aggregation struct {
	Buckets []aggregationBucket `json:"buckets"`
}

// parseSearchResponse parses the Elasticsearch response
func (s *SearchService) parseSearchResponse(body io.Reader, req *domain.SearchRequest) (*domain.SearchResponse, error) {
	var esResponse struct {
		Took int64 `json:"took"`
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID        string                   `json:"_id"`
				Score     float64                  `json:"_score"`
				Source    domain.ClassifiedContent `json:"_source"`
				Highlight map[string][]string      `json:"highlight,omitempty"`
			} `json:"hits"`
		} `json:"hits"`
		Aggregations map[string]aggregation `json:"aggregations,omitempty"`
	}

	if err := json.NewDecoder(body).Decode(&esResponse); err != nil {
		return nil, fmt.Errorf("failed to decode elasticsearch response: %w", err)
	}

	// Build response
	response := &domain.SearchResponse{
		Query:       req.Query,
		TotalHits:   esResponse.Hits.Total.Value,
		CurrentPage: req.Pagination.Page,
		PageSize:    req.Pagination.Size,
		Hits:        make([]*domain.SearchHit, 0, len(esResponse.Hits.Hits)),
	}

	// Calculate total pages
	response.TotalPages = int(math.Ceil(float64(response.TotalHits) / float64(response.PageSize)))

	// Convert hits
	for _, hit := range esResponse.Hits.Hits {
		// Set ID if not present in source
		if hit.Source.ID == "" {
			hit.Source.ID = hit.ID
		}

		searchHit := hit.Source.ToSearchHit(hit.Score, hit.Highlight)
		response.Hits = append(response.Hits, searchHit)
	}

	// Parse facets if requested
	if req.Options.IncludeFacets && len(esResponse.Aggregations) > 0 {
		response.Facets = s.parseFacets(esResponse.Aggregations)
	}

	return response, nil
}

// parseFacets parses aggregations into facets
func (s *SearchService) parseFacets(aggs map[string]aggregation) *domain.Facets {
	facets := &domain.Facets{}

	// Topics facet
	if topicsAgg, ok := aggs["topics"]; ok {
		facets.Topics = make([]domain.FacetBucket, 0, len(topicsAgg.Buckets))
		for _, bucket := range topicsAgg.Buckets {
			facets.Topics = append(facets.Topics, domain.FacetBucket{
				Key:   fmt.Sprint(bucket.Key),
				Count: bucket.DocCount,
			})
		}
	}

	// Content types facet
	if contentTypesAgg, ok := aggs["content_types"]; ok {
		facets.ContentTypes = make([]domain.FacetBucket, 0, len(contentTypesAgg.Buckets))
		for _, bucket := range contentTypesAgg.Buckets {
			facets.ContentTypes = append(facets.ContentTypes, domain.FacetBucket{
				Key:   fmt.Sprint(bucket.Key),
				Count: bucket.DocCount,
			})
		}
	}

	// Sources facet
	if sourcesAgg, ok := aggs["sources"]; ok {
		facets.Sources = make([]domain.FacetBucket, 0, len(sourcesAgg.Buckets))
		for _, bucket := range sourcesAgg.Buckets {
			facets.Sources = append(facets.Sources, domain.FacetBucket{
				Key:   fmt.Sprint(bucket.Key),
				Count: bucket.DocCount,
			})
		}
	}

	// Quality ranges facet
	if qualityRangesAgg, ok := aggs["quality_ranges"]; ok {
		facets.QualityRanges = make([]domain.FacetBucket, 0, len(qualityRangesAgg.Buckets))
		for _, bucket := range qualityRangesAgg.Buckets {
			facets.QualityRanges = append(facets.QualityRanges, domain.FacetBucket{
				Key:   fmt.Sprint(bucket.Key),
				Count: bucket.DocCount,
			})
		}
	}

	return facets
}

// HealthCheck checks the health of the search service and its dependencies
func (s *SearchService) HealthCheck(ctx context.Context) *domain.HealthStatus {
	status := &domain.HealthStatus{
		Status:       "healthy",
		Timestamp:    time.Now(),
		Version:      s.config.Service.Version,
		Dependencies: make(map[string]string),
	}

	// Check Elasticsearch
	if err := s.esClient.HealthCheck(ctx); err != nil {
		status.Status = "unhealthy"
		status.Dependencies["elasticsearch"] = "unhealthy: " + err.Error()
	} else {
		status.Dependencies["elasticsearch"] = "healthy"
	}

	return status
}
