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
	"github.com/redis/go-redis/v9"
)

// Config holds router service configuration
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
}

// NewService creates a new router service
func NewService(
	repo *database.Repository,
	disc *discovery.Service,
	esClient *elasticsearch.Client,
	redisClient *redis.Client,
	cfg Config,
	logger infralogger.Logger,
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
		s.logger.Debug("No indexes discovered, skipping poll")
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

		s.logger.Debug("Processing articles",
			infralogger.Int("count", len(articles)),
		)

		for i := range articles {
			s.routeArticle(ctx, &articles[i], channels)
		}

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

// routeArticle routes a single article to Layer 1 and Layer 2 channels
func (s *Service) routeArticle(ctx context.Context, article *Article, channels []models.Channel) {
	// Layer 1: Automatic topic channels
	for _, topic := range article.Topics {
		channel := fmt.Sprintf("articles:%s", topic)
		s.publishToChannel(ctx, article, channel, nil)
	}

	// Layer 2: Custom channels
	for i := range channels {
		ch := &channels[i]
		if ch.Rules.Matches(article.QualityScore, article.ContentType, article.Topics) {
			s.publishToChannel(ctx, article, ch.RedisChannel, &ch.ID)
		}
	}
}

// Article represents an article from Elasticsearch
type Article struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Body          string    `json:"body"`
	RawText       string    `json:"raw_text"`
	RawHTML       string    `json:"raw_html"`
	URL           string    `json:"canonical_url"`
	Source        string    `json:"source"`
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

	// Sort values for search_after pagination
	Sort []any `json:"-"`
}

// fetchArticles fetches articles from all classified indexes using search_after
func (s *Service) fetchArticles(ctx context.Context, indexes []string) ([]Article, error) {
	query := s.buildESQuery()

	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	res, err := s.esClient.Search(
		s.esClient.Search.WithContext(ctx),
		s.esClient.Search.WithIndex(indexes...),
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
		{"_id": map[string]any{"order": "asc"}},
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

// publishToChannel publishes an article to a Redis channel
func (s *Service) publishToChannel(ctx context.Context, article *Article, channelName string, channelID *uuid.UUID) {
	// Check if already published to this channel
	published, checkErr := s.repo.CheckArticlePublished(ctx, article.ID, channelName)
	if checkErr != nil {
		s.logger.Error("Error checking if article is published",
			infralogger.String("article_id", article.ID),
			infralogger.String("channel", channelName),
			infralogger.Error(checkErr),
		)
		return
	}

	if published {
		return
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

	messageJSON, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("Failed to marshal message",
			infralogger.String("article_id", article.ID),
			infralogger.Error(err),
		)
		return
	}

	if publishErr := s.redisClient.Publish(ctx, channelName, messageJSON).Err(); publishErr != nil {
		s.logger.Error("Failed to publish to Redis",
			infralogger.String("article_id", article.ID),
			infralogger.String("channel", channelName),
			infralogger.Error(publishErr),
		)
		return
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

	s.logger.Debug("Published article to channel",
		infralogger.String("article_id", article.ID),
		infralogger.String("channel", channelName),
	)
}

// GenerateLayer1Channels returns topic-based channel names for an article
func GenerateLayer1Channels(article *Article) []string {
	channels := make([]string, len(article.Topics))
	for i, topic := range article.Topics {
		channels[i] = fmt.Sprintf("articles:%s", topic)
	}
	return channels
}
