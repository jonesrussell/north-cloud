package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/jonesrussell/north-cloud/search/internal/config"
	"github.com/jonesrussell/north-cloud/search/internal/domain"
	"github.com/jonesrussell/north-cloud/search/internal/elasticsearch"
	"github.com/north-cloud/infrastructure/clickurl"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// SearchService orchestrates search operations
type SearchService struct {
	esClient     *elasticsearch.Client
	queryBuilder *elasticsearch.QueryBuilder
	config       *config.Config
	logger       infralogger.Logger
	clickSigner  *clickurl.Signer // nil if disabled
}

// NewSearchService creates a new search service
func NewSearchService(
	esClient *elasticsearch.Client,
	cfg *config.Config,
	log infralogger.Logger,
	clickSigner *clickurl.Signer,
) *SearchService {
	return &SearchService{
		esClient:     esClient,
		queryBuilder: elasticsearch.NewQueryBuilder(&cfg.Elasticsearch),
		config:       cfg,
		logger:       log,
		clickSigner:  clickSigner,
	}
}

// Search executes a search query
func (s *SearchService) Search(ctx context.Context, req *domain.SearchRequest) (*domain.SearchResponse, error) {
	startTime := time.Now()

	// Validate request
	if err := req.Validate(s.config.Service.MaxPageSize, s.config.Service.DefaultPageSize, s.config.Service.MaxQueryLength); err != nil {
		s.logger.Warn("Invalid search request",
			infralogger.Error(err),
		)
		return nil, fmt.Errorf("validation error: %w", err)
	}

	s.logger.Info("Executing search",
		infralogger.String("query", req.Query),
		infralogger.Int("page", req.Pagination.Page),
		infralogger.Int("size", req.Pagination.Size),
	)

	// Build Elasticsearch query
	esQuery := s.queryBuilder.Build(req)

	// Execute search
	res, err := s.executeSearch(ctx, esQuery)
	if err != nil {
		s.logger.Error("Search execution failed",
			infralogger.Error(err),
			infralogger.String("query", req.Query),
		)
		return nil, err
	}
	defer func() {
		_ = res.Body.Close()
	}()

	// Parse response
	response, err := s.parseSearchResponse(res.Body, req)
	if err != nil {
		s.logger.Error("Failed to parse search response",
			infralogger.Error(err),
		)
		return nil, err
	}

	// Calculate execution time
	response.TookMs = time.Since(startTime).Milliseconds()

	s.logger.Info("Search completed",
		infralogger.String("query", req.Query),
		infralogger.Int64("total_hits", response.TotalHits),
		infralogger.Int64("took_ms", response.TookMs),
	)

	return response, nil
}

const (
	suggestMaxSize   = 15
	suggestReturn    = 10
	suggestMinLength = 2
	publicFeedSize   = 20
	snippetMaxLength = 200

	pipelineFeedMinQuality = 60
	topicFeedMinQuality    = 50
	defaultFeedLimit       = 10
	maxFeedLimit           = 20
)

// Suggest returns autocomplete suggestions based on title prefix match
func (s *SearchService) Suggest(ctx context.Context, q string) (*domain.SuggestResponse, error) {
	q = strings.TrimSpace(q)
	if len(q) < suggestMinLength {
		return &domain.SuggestResponse{Suggestions: []string{}}, nil
	}

	esQuery := map[string]any{
		"size":    suggestMaxSize,
		"_source": []string{"title"},
		"query": map[string]any{
			"match_phrase_prefix": map[string]any{
				"title": map[string]any{
					"query": q,
					"slop":  0,
				},
			},
		},
	}

	res, err := s.executeSearch(ctx, esQuery)
	if err != nil {
		s.logger.Warn("Suggest execution failed",
			infralogger.Error(err),
			infralogger.String("query", q),
		)
		return &domain.SuggestResponse{Suggestions: []string{}}, nil
	}
	defer func() {
		_ = res.Body.Close()
	}()

	suggestions, parseErr := s.parseSuggestResponse(res.Body)
	if parseErr != nil {
		return &domain.SuggestResponse{Suggestions: []string{}}, nil
	}

	return &domain.SuggestResponse{Suggestions: suggestions}, nil
}

// parseSuggestResponse extracts unique title strings from a minimal search response
func (s *SearchService) parseSuggestResponse(body io.Reader) ([]string, error) {
	var esResponse struct {
		Hits struct {
			Hits []struct {
				Source struct {
					Title string `json:"title"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(body).Decode(&esResponse); err != nil {
		return nil, fmt.Errorf("decode suggest response: %w", err)
	}

	seen := make(map[string]struct{}, suggestReturn)
	out := make([]string, 0, suggestReturn)
	for _, hit := range esResponse.Hits.Hits {
		t := strings.TrimSpace(hit.Source.Title)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
		if len(out) >= suggestReturn {
			break
		}
	}
	return out, nil
}

// executeSearch performs the Elasticsearch search request
func (s *SearchService) executeSearch(ctx context.Context, query map[string]any) (*esapi.Response, error) {
	// Marshal query to JSON
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, fmt.Errorf("failed to encode query: %w", err)
	}

	// Log query in debug mode
	if s.config.Service.Debug {
		s.logger.Debug("Elasticsearch query",
			infralogger.String("query", buf.String()),
		)
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
		_ = res.Body.Close()
		return nil, fmt.Errorf("elasticsearch returned error [%d]: %s", res.StatusCode, string(body))
	}

	return res, nil
}

// aggregationBucket represents a single bucket in an aggregation
type aggregationBucket struct {
	Key      any   `json:"key"`
	DocCount int64 `json:"doc_count"`
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
	for i := range esResponse.Hits.Hits {
		hit := &esResponse.Hits.Hits[i]
		// Set ID if not present in source
		if hit.Source.ID == "" {
			hit.Source.ID = hit.ID
		}

		searchHit := hit.Source.ToSearchHit(hit.Score, hit.Highlight)
		response.Hits = append(response.Hits, searchHit)
	}

	// Add click URLs if signer is configured
	if s.clickSigner != nil {
		queryID := generateQueryID()
		s.addClickURLs(response.Hits, queryID, req.Pagination.Page)
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

const queryIDLength = 8

func (s *SearchService) addClickURLs(hits []*domain.SearchHit, queryID string, page int) {
	baseURL := strings.TrimRight(s.config.ClickTracker.BaseURL, "/")
	now := time.Now().Unix()

	for i, hit := range hits {
		if hit.URL == "" {
			continue
		}
		position := i + 1
		params := clickurl.ClickParams{
			QueryID:        queryID,
			ResultID:       hit.ID,
			Position:       position,
			Page:           page,
			Timestamp:      now,
			DestinationURL: hit.URL,
		}
		sig := s.clickSigner.Sign(params.Message())
		hit.ClickURL = fmt.Sprintf(
			"%s/click?q=%s&r=%s&p=%d&pg=%d&t=%d&u=%s&sig=%s",
			baseURL, url.QueryEscape(queryID), url.QueryEscape(hit.ID),
			position, page, now,
			url.QueryEscape(hit.URL), sig,
		)
	}
}

func generateQueryID() string {
	b := make([]byte, queryIDLength)
	if _, err := rand.Read(b); err != nil {
		return "q_" + fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return "q_" + hex.EncodeToString(b)[:queryIDLength]
}

// LatestArticles returns the most recent classified articles for the public feed.
// No auth; used by static sites at build time. Size is fixed (publicFeedSize).
func (s *SearchService) LatestArticles(ctx context.Context) ([]domain.PublicFeedArticle, error) {
	query := map[string]any{
		"query": map[string]any{"match_all": map[string]any{}},
		"size":  publicFeedSize,
		"sort": []any{
			map[string]any{"published_date": map[string]any{"order": "desc", "missing": "_last"}},
			map[string]any{"crawled_at": map[string]any{"order": "desc", "missing": "_last"}},
		},
		"_source": []string{
			"id", "title", "url", "source_name",
			"published_date", "crawled_at", "raw_text", "topics",
		},
	}
	res, err := s.executeSearch(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = res.Body.Close()
	}()
	return s.parseLatestArticlesResponse(res.Body)
}

// feedFilterForSlug maps a feed slug to topic filters and minimum quality score.
func feedFilterForSlug(slug string) (topics []string, minQuality int) {
	switch slug {
	case "crime":
		return []string{"violent_crime", "property_crime", "drug_crime", "organized_crime", "criminal_justice"}, topicFeedMinQuality
	case "mining":
		return []string{"mining"}, topicFeedMinQuality
	case "entertainment":
		return []string{"entertainment"}, topicFeedMinQuality
	default:
		return nil, pipelineFeedMinQuality
	}
}

// TopicFeed returns recent articles filtered by feed slug (topic + quality).
func (s *SearchService) TopicFeed(ctx context.Context, slug string, limit int) ([]domain.PublicFeedArticle, error) {
	if limit <= 0 || limit > maxFeedLimit {
		limit = defaultFeedLimit
	}

	topics, minQuality := feedFilterForSlug(slug)

	filters := []map[string]any{
		{"term": map[string]any{"content_type.keyword": "article"}},
		{"range": map[string]any{"quality_score": map[string]any{"gte": minQuality}}},
	}
	if len(topics) > 0 {
		filters = append(filters, map[string]any{
			"terms": map[string]any{"topics.keyword": topics},
		})
	}

	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"filter": filters,
			},
		},
		"size": limit,
		"sort": []any{
			map[string]any{"published_date": map[string]any{"order": "desc", "missing": "_last"}},
			map[string]any{"crawled_at": map[string]any{"order": "desc", "missing": "_last"}},
		},
		"_source": []string{
			"id", "title", "url", "source_name",
			"published_date", "crawled_at", "raw_text", "topics",
		},
	}

	res, err := s.executeSearch(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = res.Body.Close()
	}()

	return s.parseLatestArticlesResponse(res.Body)
}

// parseLatestArticlesResponse parses ES response into PublicFeedArticle slice.
func (s *SearchService) parseLatestArticlesResponse(body io.Reader) ([]domain.PublicFeedArticle, error) {
	var esResponse struct {
		Hits struct {
			Hits []struct {
				ID     string `json:"_id"`
				Source struct {
					ID            string     `json:"id"`
					Title         string     `json:"title"`
					URL           string     `json:"url"`
					SourceName    string     `json:"source_name"`
					PublishedDate *time.Time `json:"published_date"`
					CrawledAt     *time.Time `json:"crawled_at"`
					RawText       string     `json:"raw_text"`
					Topics        []string   `json:"topics"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(body).Decode(&esResponse); err != nil {
		return nil, fmt.Errorf("decode latest articles response: %w", err)
	}
	out := make([]domain.PublicFeedArticle, 0, len(esResponse.Hits.Hits))
	for i := range esResponse.Hits.Hits {
		hit := &esResponse.Hits.Hits[i]
		id := hit.Source.ID
		if id == "" {
			id = hit.ID
		}
		pubAt := time.Time{}
		if hit.Source.PublishedDate != nil {
			pubAt = *hit.Source.PublishedDate
		} else if hit.Source.CrawledAt != nil {
			pubAt = *hit.Source.CrawledAt
		}
		snippet := hit.Source.RawText
		if len(snippet) > snippetMaxLength {
			snippet = snippet[:snippetMaxLength] + "..."
		}
		sourceName := hit.Source.SourceName
		if sourceName == "" {
			sourceName = "pipeline"
		}
		out = append(out, domain.PublicFeedArticle{
			ID:          id,
			Title:       hit.Source.Title,
			Slug:        slugFromTitle(hit.Source.Title, id),
			URL:         hit.Source.URL,
			Snippet:     snippet,
			PublishedAt: pubAt,
			Topics:      append([]string(nil), hit.Source.Topics...),
			Source:      sourceName,
		})
	}
	return out, nil
}

// slugFromTitle returns a URL-safe slug; falls back to id if title is empty.
func slugFromTitle(title, id string) string {
	if title == "" {
		return id
	}
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if (r == ' ' || r == '-' || r == '_') && !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	result := strings.Trim(strings.Trim(b.String(), "-"), " ")
	if result == "" {
		return id
	}
	return result
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
