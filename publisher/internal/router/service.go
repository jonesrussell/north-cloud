package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/redis/go-redis/v9"
)

// Service handles the routing of articles from Elasticsearch to Redis channels
type Service struct {
	repo          *database.Repository
	esClient      *elasticsearch.Client
	redisClient   *redis.Client
	logger        infralogger.Logger
	checkInterval time.Duration
	batchSize     int
}

// Config holds router service configuration
type Config struct {
	CheckInterval time.Duration
	BatchSize     int
}

// NewService creates a new router service
func NewService(
	repo *database.Repository,
	esClient *elasticsearch.Client,
	redisClient *redis.Client,
	cfg Config,
	logger infralogger.Logger,
) *Service {
	const defaultCheckInterval = 5 * time.Minute
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = defaultCheckInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 100
	}

	return &Service{
		repo:          repo,
		esClient:      esClient,
		redisClient:   redisClient,
		logger:        logger,
		checkInterval: cfg.CheckInterval,
		batchSize:     cfg.BatchSize,
	}
}

// Start begins the router service loop
func (s *Service) Start(ctx context.Context) error {
	s.logger.Info("Router service starting...")

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	// Run immediately on start
	if err := s.processRoutes(ctx); err != nil {
		s.logger.Error("Error processing routes on startup", infralogger.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Router service stopping...")
			return ctx.Err()
		case <-ticker.C:
			if err := s.processRoutes(ctx); err != nil {
				s.logger.Error("Error processing routes", infralogger.Error(err))
			}
		}
	}
}

// processRoutes processes all enabled routes
func (s *Service) processRoutes(ctx context.Context) error {
	s.logger.Debug("Processing routes...")

	// Get all enabled routes with details
	routes, err := s.repo.ListRoutesWithDetails(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to list routes: %w", err)
	}

	if len(routes) == 0 {
		s.logger.Debug("No enabled routes found")
		return nil
	}

	s.logger.Debug("Processing enabled routes",
		infralogger.Int("route_count", len(routes)),
	)

	// Process each route
	for i := range routes {
		if routeErr := s.processRoute(ctx, &routes[i]); routeErr != nil {
			s.logger.Error("Error processing route",
				infralogger.String("route_id", routes[i].ID.String()),
				infralogger.String("source_name", routes[i].SourceName),
				infralogger.String("channel_name", routes[i].ChannelName),
				infralogger.Error(routeErr),
			)
			// Continue processing other routes even if one fails
			continue
		}
	}

	s.logger.Debug("Finished processing routes")
	return nil
}

// processRoute processes a single route.
// Returns nil on errors to continue processing other routes.
//
//nolint:unparam // Intentional: always returns nil to continue processing other routes
func (s *Service) processRoute(ctx context.Context, route *models.RouteWithDetails) error {
	s.logger.Debug("Processing route",
		infralogger.String("source_name", route.SourceName),
		infralogger.String("channel_name", route.ChannelName),
		infralogger.Int("min_quality_score", route.MinQualityScore),
		infralogger.Strings("topics", route.Topics),
	)

	// Fetch articles from Elasticsearch
	articles, fetchErr := s.fetchArticles(ctx, route)
	if fetchErr != nil {
		// Log error but don't fail the entire route processing
		s.logger.Error("Error fetching articles for route",
			infralogger.String("source_name", route.SourceName),
			infralogger.String("channel_name", route.ChannelName),
			infralogger.Error(fetchErr),
		)
		// Return nil to continue processing other routes (unparam warning is acceptable here)
		return nil
	}

	if len(articles) == 0 {
		s.logger.Debug("No articles found for route",
			infralogger.String("source_name", route.SourceName),
			infralogger.String("channel_name", route.ChannelName),
		)
		return nil
	}

	s.logger.Debug("Found articles for route",
		infralogger.String("source_name", route.SourceName),
		infralogger.String("channel_name", route.ChannelName),
		infralogger.Int("article_count", len(articles)),
	)

	publishedCount := 0
	skippedCount := 0

	// Process each article
	for i := range articles {
		// Check if already published to this channel
		published, checkErr := s.repo.CheckArticlePublished(ctx, articles[i].ID, route.ChannelName)
		if checkErr != nil {
			s.logger.Error("Error checking if article is published",
				infralogger.String("article_id", articles[i].ID),
				infralogger.Error(checkErr),
			)
			continue
		}

		if published {
			skippedCount++
			continue
		}

		// Publish article to Redis channel
		if publishErr := s.publishArticle(ctx, route, &articles[i]); publishErr != nil {
			s.logger.Error("Error publishing article",
				infralogger.String("article_id", articles[i].ID),
				infralogger.Error(publishErr),
			)
			continue
		}

		// Record in publish history
		historyReq := &models.PublishHistoryCreateRequest{
			RouteID:      route.ID,
			ArticleID:    articles[i].ID,
			ArticleTitle: articles[i].Title,
			ArticleURL:   articles[i].URL,
			ChannelName:  route.ChannelName,
			QualityScore: articles[i].QualityScore,
			Topics:       articles[i].Topics,
		}

		if _, historyErr := s.repo.CreatePublishHistory(ctx, historyReq); historyErr != nil {
			s.logger.Error("Error recording publish history",
				infralogger.String("article_id", articles[i].ID),
				infralogger.Error(historyErr),
			)
			// Continue even if history recording fails
		}

		publishedCount++
	}

	s.logger.Info("Route processing completed",
		infralogger.String("source_name", route.SourceName),
		infralogger.String("channel_name", route.ChannelName),
		infralogger.Int("published_count", publishedCount),
		infralogger.Int("skipped_count", skippedCount),
	)

	return nil
}

// Article represents an article from Elasticsearch
type Article struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Body          string    `json:"body"` // Alias for raw_text
	RawText       string    `json:"raw_text"`
	RawHTML       string    `json:"raw_html"`
	URL           string    `json:"canonical_url"`
	Source        string    `json:"source"` // Original article URL
	PublishedDate time.Time `json:"published_date"`

	// Classification metadata
	QualityScore     int      `json:"quality_score"`
	Topics           []string `json:"topics"`
	ContentType      string   `json:"content_type"`
	IsCrimeRelated   bool     `json:"is_crime_related"`
	SourceReputation int      `json:"source_reputation"`
	Confidence       float64  `json:"confidence"`

	// Open Graph metadata
	OGTitle       string `json:"og_title"`
	OGDescription string `json:"og_description"`
	OGImage       string `json:"og_image"`
	OGURL         string `json:"og_url"`

	// Additional fields
	Intro       string   `json:"intro"`
	Description string   `json:"description"`
	WordCount   int      `json:"word_count"`
	Category    string   `json:"category"`
	Section     string   `json:"section"`
	Keywords    []string `json:"keywords"`
}

// fetchArticles fetches articles from Elasticsearch for a route
func (s *Service) fetchArticles(ctx context.Context, route *models.RouteWithDetails) ([]Article, error) {
	// Build Elasticsearch query
	query := s.buildESQuery(route)

	// Execute search
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	res, err := s.esClient.Search(
		s.esClient.Search.WithContext(ctx),
		s.esClient.Search.WithIndex(route.SourceIndexPattern),
		s.esClient.Search.WithBody(bytes.NewReader(queryJSON)),
		s.esClient.Search.WithSize(s.batchSize),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		// Check if it's a 404 (index not found) - this is expected for new sources
		const httpStatusNotFound = 404
		if res.StatusCode == httpStatusNotFound {
			s.logger.Debug("Index not found (this is normal for new sources)",
				infralogger.String("index_pattern", route.SourceIndexPattern),
			)
			return []Article{}, nil // Return empty slice, not an error
		}
		// Read error response body for better error messages
		errorBody, readErr := io.ReadAll(res.Body)
		if readErr != nil {
			return nil, fmt.Errorf("elasticsearch error (status %d): failed to read error body: %w", res.StatusCode, readErr)
		}
		return nil, fmt.Errorf("elasticsearch error (status %d): %s", res.StatusCode, string(errorBody))
	}

	// Read response body into buffer first to detect truncation
	bodyBytes, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read response body: %w", readErr)
	}

	// Parse response
	var esResponse struct {
		Hits struct {
			Hits []struct {
				ID     string          `json:"_id"`
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if decodeErr := json.Unmarshal(bodyBytes, &esResponse); decodeErr != nil {
		// Log partial response for debugging (truncate if too long)
		const maxErrorBodyLength = 1000
		errorPreview := string(bodyBytes)
		if len(errorPreview) > maxErrorBodyLength {
			errorPreview = errorPreview[:maxErrorBodyLength] + "... (truncated)"
		}
		s.logger.Error("Failed to decode Elasticsearch response",
			infralogger.String("index_pattern", route.SourceIndexPattern),
			infralogger.Int("response_length", len(bodyBytes)),
			infralogger.String("response_preview", errorPreview),
			infralogger.Error(decodeErr),
		)
		return nil, fmt.Errorf("failed to decode response from index %s (response length: %d bytes): %w", route.SourceIndexPattern, len(bodyBytes), decodeErr)
	}

	// Convert to articles
	articles := make([]Article, 0, len(esResponse.Hits.Hits))
	for _, hit := range esResponse.Hits.Hits {
		var article Article
		if unmarshalErr := json.Unmarshal(hit.Source, &article); unmarshalErr != nil {
			s.logger.Error("Error unmarshaling article",
				infralogger.String("article_id", hit.ID),
				infralogger.Error(unmarshalErr),
			)
			continue
		}
		article.ID = hit.ID
		articles = append(articles, article)
	}

	return articles, nil
}

// buildESQuery builds an Elasticsearch query for a route
func (s *Service) buildESQuery(route *models.RouteWithDetails) map[string]any {
	mustClauses := []map[string]any{}

	// Filter by quality score and content_type
	mustClauses = append(mustClauses,
		map[string]any{
			"range": map[string]any{
				"quality_score": map[string]any{
					"gte": route.MinQualityScore,
				},
			},
		},
		map[string]any{
			"term": map[string]any{
				"content_type": "article",
			},
		},
	)

	// Filter by topics if specified
	if len(route.Topics) > 0 {
		mustClauses = append(mustClauses, map[string]any{
			"terms": map[string]any{
				"topics": route.Topics,
			},
		})
	}

	// Build sort clause - use crawled_at as primary since it's always present
	// Some indexes may not have published_date in their mapping
	sortClause := []map[string]any{
		{
			"crawled_at": map[string]any{
				"order": "desc",
			},
		},
	}

	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": mustClauses,
			},
		},
		"sort": sortClause,
	}

	return query
}

// publishArticle publishes an article to a Redis channel
func (s *Service) publishArticle(ctx context.Context, route *models.RouteWithDetails, article *Article) error {
	// Build message payload
	payload := map[string]any{
		"publisher": map[string]any{
			"route_id":     route.ID,
			"published_at": time.Now().Format(time.RFC3339),
			"channel":      route.ChannelName,
		},
		// Article fields
		"id":                article.ID,
		"title":             article.Title,
		"body":              article.Body,
		"raw_text":          article.RawText,
		"raw_html":          article.RawHTML,
		"canonical_url":     article.URL,
		"source":            article.Source,
		"published_date":    article.PublishedDate.Format(time.RFC3339),
		"quality_score":     article.QualityScore,
		"topics":            article.Topics,
		"content_type":      article.ContentType,
		"is_crime_related":  article.IsCrimeRelated,
		"source_reputation": article.SourceReputation,
		"confidence":        article.Confidence,
		"og_title":          article.OGTitle,
		"og_description":    article.OGDescription,
		"og_image":          article.OGImage,
		"og_url":            article.OGURL,
		"intro":             article.Intro,
		"description":       article.Description,
		"word_count":        article.WordCount,
		"category":          article.Category,
		"section":           article.Section,
		"keywords":          article.Keywords,
	}

	// Marshal to JSON
	messageJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Publish to Redis channel
	if publishErr := s.redisClient.Publish(ctx, route.ChannelName, messageJSON).Err(); publishErr != nil {
		return fmt.Errorf("failed to publish to Redis: %w", publishErr)
	}

	s.logger.Debug("Published article to channel",
		infralogger.String("article_id", article.ID),
		infralogger.String("channel_name", route.ChannelName),
	)
	return nil
}
