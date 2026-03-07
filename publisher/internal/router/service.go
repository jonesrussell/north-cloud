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
	"github.com/jonesrussell/north-cloud/publisher/internal/telemetry"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/pipeline"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	PollInterval      time.Duration
	DiscoveryInterval time.Duration
	BatchSize         int
}

// Service handles routing content items to Redis channels using two-layer routing
type Service struct {
	repo        *database.Repository
	discovery   *discovery.Service
	esClient    *elasticsearch.Client
	redisClient *redis.Client
	logger      infralogger.Logger
	config      Config
	lastSort    []any
	pipeline    *pipeline.Client
	telemetry   *telemetry.Provider
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
	tp *telemetry.Provider,
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
		telemetry:   tp,
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

// pollAndRoute fetches new content items and routes them
func (s *Service) pollAndRoute(ctx context.Context) {
	pollStart := time.Now()

	indexes := s.discovery.GetIndexes()
	if len(indexes) == 0 {
		s.logger.Info("No indexes discovered, skipping poll")
		return
	}

	// Load custom channels (Layer 2)
	channels, err := s.repo.ListEnabledChannelsWithRules(ctx)
	if err != nil {
		s.logger.Error("Failed to load channels — aborting poll cycle to prevent dedup pollution",
			infralogger.Error(err),
		)
		return
	}

	// Loop until we've drained the queue
	var totalItems int
	for {
		items, fetchErr := s.fetchContentItems(ctx, indexes)
		if fetchErr != nil {
			s.logger.Error("Failed to fetch content items", infralogger.Error(fetchErr))
			return
		}

		if len(items) == 0 {
			break
		}

		batchSize := len(items)
		totalItems += batchSize
		s.logger.Info("Processing content items batch",
			infralogger.Int("batch_size", batchSize),
			infralogger.Int("items_fetched_total", batchSize),
		)

		var publishedCount int
		for i := range items {
			publishedTo := s.routeContentItem(ctx, &items[i], channels)
			publishedCount += len(publishedTo)
			if s.telemetry != nil {
				s.telemetry.RecordChannelsPerDoc(len(publishedTo))
			}
		}
		s.logger.Info("Batch complete",
			infralogger.Int("items_in_batch", batchSize),
			infralogger.Int("items_published_total", publishedCount),
		)

		// Update cursor and record cursor lag
		lastItem := items[len(items)-1]
		s.lastSort = lastItem.Sort
		if persistErr := s.repo.UpdateCursor(ctx, s.lastSort); persistErr != nil {
			s.logger.Error("Failed to persist cursor", infralogger.Error(persistErr))
		}
		if s.telemetry != nil {
			s.recordCursorLag(lastItem)
		}

		// If we got less than batch size, we're done
		if len(items) < s.config.BatchSize {
			break
		}
	}

	if s.telemetry != nil && totalItems > 0 {
		s.telemetry.RecordBatch(totalItems, time.Since(pollStart))
	}
}

// publishRoutes publishes a content item to each ChannelRoute and returns names of channels
// where publishing succeeded.
func (s *Service) publishRoutes(ctx context.Context, item *ContentItem, routes []ChannelRoute) []string {
	published := make([]string, 0, len(routes))
	for _, route := range routes {
		if s.publishToChannel(ctx, item, route.Channel, route.ChannelID) {
			published = append(published, route.Channel)
		}
	}
	return published
}

// routeContentItem routes a single content item through all routing domains and returns the list
// of channel names where publish succeeded.
func (s *Service) routeContentItem(ctx context.Context, item *ContentItem, channels []models.Channel) []string {
	const maxChannelsPerItem = 30

	domains := []RoutingDomain{
		NewTopicDomain(),
		NewDBChannelDomain(channels),
		NewCrimeDomain(),
		NewLocationDomain(),
		NewMiningDomain(),
		NewEntertainmentDomain(),
		NewIndigenousDomain(),
		NewCoforgeDomain(),
		NewRecipeDomain(),
		NewJobDomain(),
		NewRFPDomain(),
	}

	var publishedChannels []string
	for _, domain := range domains {
		routes := domain.Routes(item)
		if len(routes) == 0 {
			continue
		}
		s.logger.Debug("routing decision",
			infralogger.String("domain", domain.Name()),
			infralogger.Int("routes", len(routes)),
		)
		publishedChannels = append(publishedChannels, s.publishRoutes(ctx, item, routes)...)
	}

	if len(publishedChannels) > maxChannelsPerItem {
		s.logger.Warn("content item published to unusually many channels",
			infralogger.String("content_id", item.ID),
			infralogger.Int("channel_count", len(publishedChannels)),
			infralogger.Int("max_channels", maxChannelsPerItem),
		)
	}

	// Emit pipeline event (one event per content item, all channels in metadata)
	s.emitPublishedEvent(ctx, item, publishedChannels)
	return publishedChannels
}

// classifiedContentWildcard matches all classified content indexes in Elasticsearch.
const classifiedContentWildcard = "*_classified_content"

// fetchContentItems fetches content items from all classified indexes using search_after.
// Uses a wildcard pattern instead of listing individual indexes to avoid exceeding
// Elasticsearch's HTTP line length limit when many indexes exist.
func (s *Service) fetchContentItems(ctx context.Context, _ []string) ([]ContentItem, error) {
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
			return []ContentItem{}, nil
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

	items := make([]ContentItem, 0, len(esResponse.Hits.Hits))
	for _, hit := range esResponse.Hits.Hits {
		var item ContentItem
		if unmarshalErr := json.Unmarshal(hit.Source, &item); unmarshalErr != nil {
			s.logger.Error("Error unmarshaling content item",
				infralogger.String("content_id", hit.ID),
				infralogger.Error(unmarshalErr),
			)
			continue
		}
		item.ID = hit.ID
		item.Sort = hit.Sort
		item.extractNestedFields()
		items = append(items, item)
	}

	return items, nil
}

// buildESQuery builds an Elasticsearch query for all classified content
func (s *Service) buildESQuery() map[string]any {
	mustClauses := []map[string]any{
		{
			"terms": map[string]any{
				"content_type": []string{"article", "recipe", "job", "rfp"},
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

// publishToChannel publishes a content item to a Redis channel.
// Returns true if the item was successfully published, false otherwise.
func (s *Service) publishToChannel(ctx context.Context, item *ContentItem, channelName string, channelID *uuid.UUID) bool {
	// Check if already published to this channel
	published, checkErr := s.repo.CheckContentPublished(ctx, item.ID, channelName)
	if checkErr != nil {
		s.logger.Error("Error checking if content is published",
			infralogger.String("content_id", item.ID),
			infralogger.String("channel", channelName),
			infralogger.Error(checkErr),
		)
		return false
	}

	if published {
		if s.telemetry != nil {
			s.telemetry.RecordDedupHit()
		}
		return false
	}

	messageJSON, err := json.Marshal(buildPublishPayload(item, channelName, channelID))
	if err != nil {
		s.logger.Error("Failed to marshal message",
			infralogger.String("content_id", item.ID),
			infralogger.Error(err),
		)
		return false
	}

	if publishErr := s.redisClient.Publish(ctx, channelName, messageJSON).Err(); publishErr != nil {
		s.logger.Error("Failed to publish to Redis",
			infralogger.String("content_id", item.ID),
			infralogger.String("channel", channelName),
			infralogger.Error(publishErr),
		)
		return false
	}

	// Record in publish history
	if _, historyErr := s.repo.CreatePublishHistory(ctx, buildHistoryReq(channelID, item, channelName)); historyErr != nil {
		s.logger.Error("Error recording publish history — skipping to prevent duplicate publish",
			infralogger.String("content_id", item.ID),
			infralogger.String("channel", channelName),
			infralogger.Error(historyErr),
		)
		return false
	}

	s.logger.Info("Published content item to channel",
		infralogger.String("content_id", item.ID),
		infralogger.String("title", item.Title),
		infralogger.String("channel", channelName),
	)

	if s.telemetry != nil {
		s.telemetry.RecordPublish(channelName)
	}

	return true
}

// buildPublishPayload constructs the Redis message payload for a content item.
func buildPublishPayload(item *ContentItem, channelName string, channelID *uuid.UUID) map[string]any {
	return map[string]any{
		"publisher": map[string]any{
			"channel_id":   channelID,
			"published_at": time.Now().Format(time.RFC3339),
			"channel":      channelName,
		},
		"id":                item.ID,
		"title":             item.Title,
		"body":              item.Body,
		"raw_text":          item.RawText,
		"raw_html":          item.RawHTML,
		"canonical_url":     item.URL,
		"source":            item.Source,
		"published_date":    item.PublishedDate.Format(time.RFC3339),
		"quality_score":     item.QualityScore,
		"topics":            item.Topics,
		"content_type":      item.ContentType,
		"content_subtype":   item.ContentSubtype,
		"source_reputation": item.SourceReputation,
		"confidence":        item.Confidence,
		"og_title":          item.OGTitle,
		"og_description":    item.OGDescription,
		"og_image":          item.OGImage,
		"og_url":            item.OGURL,
		"word_count":        item.WordCount,
		// Crime classification
		"crime_relevance":      item.CrimeRelevance,
		"crime_sub_label":      item.CrimeSubLabel,
		"crime_types":          item.CrimeTypes,
		"location_specificity": item.LocationSpecificity,
		"homepage_eligible":    item.HomepageEligible,
		"category_pages":       item.CategoryPages,
		"review_required":      item.ReviewRequired,
		// Mining classification
		"mining": item.Mining,
		// Indigenous classification
		"indigenous": item.Indigenous,
		// Coforge classification
		"coforge": item.Coforge,
		// Entertainment classification
		"entertainment_relevance":         item.EntertainmentRelevance,
		"entertainment_categories":        item.EntertainmentCategories,
		"entertainment_homepage_eligible": item.EntertainmentHomepageEligible,
		"entertainment":                   item.Entertainment,
		// Recipe extraction
		"recipe": item.Recipe,
		// Job extraction
		"job": item.Job,
		// RFP extraction
		"rfp": item.RFP,
		// Location detection
		"location_city":       item.LocationCity,
		"location_province":   item.LocationProvince,
		"location_country":    item.LocationCountry,
		"location_confidence": item.LocationConfidence,
	}
}

// buildHistoryReq constructs a PublishHistoryCreateRequest from the content item and routing info.
func buildHistoryReq(channelID *uuid.UUID, item *ContentItem, channelName string) *models.PublishHistoryCreateRequest {
	return &models.PublishHistoryCreateRequest{
		ChannelID:    channelID,
		ContentID:    item.ID,
		ContentTitle: item.Title,
		ContentURL:   item.URL,
		ChannelName:  channelName,
		QualityScore: item.QualityScore,
		Topics:       item.Topics,
	}
}

// recordCursorLag extracts the crawled_at timestamp from the sort key and records the cursor lag.
func (s *Service) recordCursorLag(item ContentItem) {
	if len(item.Sort) == 0 {
		return
	}
	// Sort key is [crawled_at_millis, _shard_doc]. First element is epoch millis (float64 from JSON).
	if millis, ok := item.Sort[0].(float64); ok {
		const msPerSecond = 1000
		ts := time.Unix(int64(millis)/msPerSecond, (int64(millis)%msPerSecond)*int64(time.Millisecond))
		s.telemetry.RecordCursorLag(ts)
	}
}

// emitPublishedEvent emits a pipeline event after a content item is published to channels.
func (s *Service) emitPublishedEvent(ctx context.Context, item *ContentItem, channels []string) {
	if s.pipeline == nil || len(channels) == 0 {
		return
	}

	pipelineErr := s.pipeline.Emit(ctx, pipeline.Event{
		ContentURL: item.URL,
		SourceName: item.Source,
		Stage:      "published",
		OccurredAt: time.Now(),
		Metadata: map[string]any{
			"channels":      channels,
			"quality_score": item.QualityScore,
			"topics":        item.Topics,
		},
	})
	if pipelineErr != nil {
		s.logger.Warn("Failed to emit pipeline event",
			infralogger.Error(pipelineErr),
			infralogger.String("content_id", item.ID),
			infralogger.String("stage", "published"),
		)
	}
}
