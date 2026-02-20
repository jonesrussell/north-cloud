package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	"github.com/jonesrussell/north-cloud/publisher/internal/discovery"
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/pipeline"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	PollInterval      time.Duration
	DiscoveryInterval time.Duration
	BatchSize         int
}

// Service handles routing articles to Redis channels using two-layer routing
type Service struct {
	repo        *database.Repository
	discovery   *discovery.Service
	esClient    *elasticsearch.Client
	redisClient *redis.Client
	logger      infralogger.Logger
	config      Config
	lastSort    []any
	pipeline    *pipeline.Client
}

// NewService creates a new router service
func NewService(
	repo *database.Repository,
	disc *discovery.Service,
	esClient *elasticsearch.Client,
	redisClient *redis.Client,
	cfg Config,
	logger infralogger.Logger,
	pipelineClient *pipeline.Client,
) *Service {
	// Apply defaults
	const (
		defaultPollInterval      = 30 * time.Second
		defaultDiscoveryInterval = 5 * time.Minute
		defaultBatchSize         = 100
	)

	if cfg.PollInterval == 0 {
		cfg.PollInterval = defaultPollInterval
	}
	if cfg.DiscoveryInterval == 0 {
		cfg.DiscoveryInterval = defaultDiscoveryInterval
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = defaultBatchSize
	}

	return &Service{
		repo:        repo,
		discovery:   disc,
		esClient:    esClient,
		redisClient: redisClient,
		logger:      logger,
		config:      cfg,
		lastSort:    []any{},
		pipeline:    pipelineClient,
	}
}

// Start begins the router service loop
func (s *Service) Start(ctx context.Context) error {
	s.logger.Info("Router service starting (routing v2)...")

	// Load cursor from database
	cursor, err := s.repo.GetCursor(ctx)
	if err != nil {
		s.logger.Warn("Failed to load cursor, starting fresh", infralogger.Error(err))
	} else {
		s.lastSort = cursor
	}

	// Initial discovery
	if _, discErr := s.discovery.DiscoverIndexes(ctx); discErr != nil {
		s.logger.Error("Initial index discovery failed", infralogger.Error(discErr))
	}

	discoveryTicker := time.NewTicker(s.config.DiscoveryInterval)
	pollTicker := time.NewTicker(s.config.PollInterval)
	defer discoveryTicker.Stop()
	defer pollTicker.Stop()

	// Run immediately
	s.pollAndRoute(ctx)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Router service stopping...")
			return ctx.Err()

		case <-discoveryTicker.C:
			if _, discErr := s.discovery.DiscoverIndexes(ctx); discErr != nil {
				s.logger.Error("Index discovery failed", infralogger.Error(discErr))
			}

		case <-pollTicker.C:
			s.pollAndRoute(ctx)
		}
	}
}

// pollAndRoute fetches new articles and routes them
func (s *Service) pollAndRoute(ctx context.Context) {
	indexes := s.discovery.GetIndexes()
	if len(indexes) == 0 {
		s.logger.Info("No indexes discovered, skipping poll")
		return
	}

	// Load custom channels (Layer 2)
	channels, err := s.repo.ListEnabledChannelsWithRules(ctx)
	if err != nil {
		s.logger.Error("Failed to load channels", infralogger.Error(err))
		// Continue with Layer 1 routing only
		channels = []models.Channel{}
	}

	// Loop until we've drained the queue
	for {
		articles, fetchErr := s.fetchArticles(ctx, indexes)
		if fetchErr != nil {
			s.logger.Error("Failed to fetch articles", infralogger.Error(fetchErr))
			return
		}

		if len(articles) == 0 {
			return
		}

		batchSize := len(articles)
		s.logger.Info("Processing articles batch",
			infralogger.Int("batch_size", batchSize),
			infralogger.Int("articles_fetched_total", batchSize),
		)

		var publishedCount int
		for i := range articles {
			publishedCount += len(s.routeArticle(ctx, &articles[i], channels))
		}
		s.logger.Info("Batch complete",
			infralogger.Int("articles_in_batch", batchSize),
			infralogger.Int("articles_published_total", publishedCount),
		)

		// Update cursor
		lastArticle := articles[len(articles)-1]
		s.lastSort = lastArticle.Sort
		if persistErr := s.repo.UpdateCursor(ctx, s.lastSort); persistErr != nil {
			s.logger.Error("Failed to persist cursor", infralogger.Error(persistErr))
		}

		// If we got less than batch size, we're done
		if len(articles) < s.config.BatchSize {
			return
		}
	}
}

// publishRoutes publishes an article to each ChannelRoute and returns names of channels
// where publishing succeeded.
func (s *Service) publishRoutes(ctx context.Context, article *Article, routes []ChannelRoute) []string {
	published := make([]string, 0, len(routes))
	for _, route := range routes {
		if s.publishToChannel(ctx, article, route.Channel, route.ChannelID) {
			published = append(published, route.Channel)
		}
	}
	return published
}

// routeArticle routes a single article through all routing domains and returns the list
// of channel names where publish succeeded.
func (s *Service) routeArticle(ctx context.Context, article *Article, channels []models.Channel) []string {
	const maxChannelsPerArticle = 30

	domains := []RoutingDomain{
		NewTopicDomain(),
		NewDBChannelDomain(channels),
		NewCrimeDomain(),
		NewLocationDomain(),
		NewMiningDomain(),
		NewEntertainmentDomain(),
		NewAnishinaabeeDomain(),
		NewCoforgeDomain(),
	}

	var publishedChannels []string
	for _, domain := range domains {
		routes := domain.Routes(article)
		if len(routes) == 0 {
			continue
		}
		s.logger.Debug("routing decision",
			infralogger.String("domain", domain.Name()),
			infralogger.Int("routes", len(routes)),
		)
		publishedChannels = append(publishedChannels, s.publishRoutes(ctx, article, routes)...)
	}

	if len(publishedChannels) > maxChannelsPerArticle {
		s.logger.Warn("article published to unusually many channels",
			infralogger.String("article_id", article.ID),
			infralogger.Int("channel_count", len(publishedChannels)),
			infralogger.Int("max_channels", maxChannelsPerArticle),
		)
	}

	// Emit pipeline event (one event per article, all channels in metadata)
	s.emitPublishedEvent(ctx, article, publishedChannels)
	return publishedChannels
}

// classifiedContentWildcard matches all classified content indexes in Elasticsearch.
const classifiedContentWildcard = "*_classified_content"

// fetchArticles fetches articles from all classified indexes using search_after.
// Uses a wildcard pattern instead of listing individual indexes to avoid exceeding
// Elasticsearch's HTTP line length limit when many indexes exist.
func (s *Service) fetchArticles(ctx context.Context, _ []string) ([]Article, error) {
	query := s.buildESQuery()

	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	res, err := s.esClient.Search(
		s.esClient.Search.WithContext(ctx),
		s.esClient.Search.WithIndex(classifiedContentWildcard),
		s.esClient.Search.WithBody(bytes.NewReader(queryJSON)),
		s.esClient.Search.WithSize(s.config.BatchSize),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		const httpStatusNotFound = 404
		if res.StatusCode == httpStatusNotFound {
			s.logger.Debug("Indexes not found (this is normal for new sources)")
			return []Article{}, nil
		}
		errorBody, readErr := io.ReadAll(res.Body)
		if readErr != nil {
			return nil, fmt.Errorf("elasticsearch error (status %d): failed to read error body: %w", res.StatusCode, readErr)
		}
		return nil, fmt.Errorf("elasticsearch error (status %d): %s", res.StatusCode, string(errorBody))
	}

	bodyBytes, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read response body: %w", readErr)
	}

	var esResponse struct {
		Hits struct {
			Hits []struct {
				ID     string          `json:"_id"`
				Source json.RawMessage `json:"_source"`
				Sort   []any           `json:"sort"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if decodeErr := json.Unmarshal(bodyBytes, &esResponse); decodeErr != nil {
		const maxErrorBodyLength = 1000
		errorPreview := string(bodyBytes)
		if len(errorPreview) > maxErrorBodyLength {
			errorPreview = errorPreview[:maxErrorBodyLength] + "... (truncated)"
		}
		s.logger.Error("Failed to decode Elasticsearch response",
			infralogger.Int("response_length", len(bodyBytes)),
			infralogger.String("response_preview", errorPreview),
			infralogger.Error(decodeErr),
		)
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

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
		article.Sort = hit.Sort
		article.extractNestedFields()
		articles = append(articles, article)
	}

	return articles, nil
}

// buildESQuery builds an Elasticsearch query for all classified content
func (s *Service) buildESQuery() map[string]any {
	mustClauses := []map[string]any{
		{
			"term": map[string]any{
				"content_type": "article",
			},
		},
	}

	sortClause := []map[string]any{
		{"crawled_at": map[string]any{"order": "asc"}},
		{"_shard_doc": map[string]any{"order": "asc"}}, // ES 9.x: use _shard_doc instead of _id for tiebreaker
	}

	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": mustClauses,
			},
		},
		"sort": sortClause,
	}

	// Add search_after if we have a cursor
	if len(s.lastSort) > 0 {
		query["search_after"] = s.lastSort
	}

	return query
}

// publishToChannel publishes an article to a Redis channel.
// Returns true if the article was successfully published, false otherwise.
func (s *Service) publishToChannel(ctx context.Context, article *Article, channelName string, channelID *uuid.UUID) bool {
	// Check if already published to this channel
	published, checkErr := s.repo.CheckArticlePublished(ctx, article.ID, channelName)
	if checkErr != nil {
		s.logger.Error("Error checking if article is published",
			infralogger.String("article_id", article.ID),
			infralogger.String("channel", channelName),
			infralogger.Error(checkErr),
		)
		return false
	}

	if published {
		return false
	}

	// Build message payload
	payload := map[string]any{
		"publisher": map[string]any{
			"channel_id":   channelID,
			"published_at": time.Now().Format(time.RFC3339),
			"channel":      channelName,
		},
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
		"content_subtype":   article.ContentSubtype,
		"source_reputation": article.SourceReputation,
		"confidence":        article.Confidence,
		"og_title":          article.OGTitle,
		"og_description":    article.OGDescription,
		"og_image":          article.OGImage,
		"og_url":            article.OGURL,
		"word_count":        article.WordCount,
		// Crime classification
		"crime_relevance":      article.CrimeRelevance,
		"crime_sub_label":      article.CrimeSubLabel,
		"crime_types":          article.CrimeTypes,
		"location_specificity": article.LocationSpecificity,
		"homepage_eligible":    article.HomepageEligible,
		"category_pages":       article.CategoryPages,
		"review_required":      article.ReviewRequired,
		// Mining classification
		"mining": article.Mining,
		// Anishinaabe classification
		"anishinaabe": article.Anishinaabe,
		// Coforge classification
		"coforge": article.Coforge,
		// Entertainment classification
		"entertainment_relevance":         article.EntertainmentRelevance,
		"entertainment_categories":        article.EntertainmentCategories,
		"entertainment_homepage_eligible": article.EntertainmentHomepageEligible,
		"entertainment":                   article.Entertainment,
		// Location detection
		"location_city":       article.LocationCity,
		"location_province":   article.LocationProvince,
		"location_country":    article.LocationCountry,
		"location_confidence": article.LocationConfidence,
	}

	messageJSON, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("Failed to marshal message",
			infralogger.String("article_id", article.ID),
			infralogger.Error(err),
		)
		return false
	}

	if publishErr := s.redisClient.Publish(ctx, channelName, messageJSON).Err(); publishErr != nil {
		s.logger.Error("Failed to publish to Redis",
			infralogger.String("article_id", article.ID),
			infralogger.String("channel", channelName),
			infralogger.Error(publishErr),
		)
		return false
	}

	// Record in publish history
	historyReq := &models.PublishHistoryCreateRequest{
		ArticleID:    article.ID,
		ArticleTitle: article.Title,
		ArticleURL:   article.URL,
		ChannelName:  channelName,
		QualityScore: article.QualityScore,
		Topics:       article.Topics,
	}

	if _, historyErr := s.repo.CreatePublishHistory(ctx, historyReq); historyErr != nil {
		s.logger.Error("Error recording publish history",
			infralogger.String("article_id", article.ID),
			infralogger.Error(historyErr),
		)
	}

	s.logger.Info("Published article to channel",
		infralogger.String("article_id", article.ID),
		infralogger.String("title", article.Title),
		infralogger.String("channel", channelName),
	)

	return true
}

// emitPublishedEvent emits a pipeline event after an article is published to channels.
func (s *Service) emitPublishedEvent(ctx context.Context, article *Article, channels []string) {
	if s.pipeline == nil || len(channels) == 0 {
		return
	}

	pipelineErr := s.pipeline.Emit(ctx, pipeline.Event{
		ArticleURL: article.URL,
		SourceName: article.Source,
		Stage:      "published",
		OccurredAt: time.Now(),
		Metadata: map[string]any{
			"channels":      channels,
			"quality_score": article.QualityScore,
			"topics":        article.Topics,
		},
	})
	if pipelineErr != nil {
		s.logger.Warn("Failed to emit pipeline event",
			infralogger.Error(pipelineErr),
			infralogger.String("article_id", article.ID),
			infralogger.String("stage", "published"),
		)
	}
}
