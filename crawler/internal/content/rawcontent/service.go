// Package rawcontent provides a service for extracting and indexing raw content
// from any HTML page without type assumptions or validation.
package rawcontent

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	storagepkg "github.com/jonesrussell/north-cloud/crawler/internal/storage"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/pipeline"
)

// minPostExtractionWordCount is the minimum word count for extracted content to be indexed.
const minPostExtractionWordCount = 50

// DetectedContentTypeCtxKey is the colly request context key for detected content type
// (set by the crawler when IsStructuredContentPage returns true).
const DetectedContentTypeCtxKey = "detected_content_type"

// Interface defines the interface for processing raw content.
type Interface interface {
	// Process handles the processing of raw content from any HTML page.
	Process(e *colly.HTMLElement) error
}

// Ensure RawContentService implements Interface
var _ Interface = (*RawContentService)(nil)

// RawContentService extracts and indexes raw content from any HTML page.
// It does not perform type detection or validation - that's the classifier's job.
type RawContentService struct {
	logger     infralogger.Logger
	storage    types.Interface
	sources    sources.Interface
	rawIndexer *storagepkg.RawContentIndexer
	pipeline   *pipeline.Client
}

// NewRawContentService creates a new raw content service.
func NewRawContentService(
	log infralogger.Logger,
	storage types.Interface,
	sourcesManager sources.Interface,
	pipelineClient *pipeline.Client,
) *RawContentService {
	rawIndexer := storagepkg.NewRawContentIndexer(storage, log)
	return &RawContentService{
		logger:     log,
		storage:    storage,
		sources:    sourcesManager,
		rawIndexer: rawIndexer,
		pipeline:   pipelineClient,
	}
}

// Process implements the Interface for HTML element processing.
// Extracts raw content from any HTML page and indexes it to raw_content.
func (s *RawContentService) Process(e *colly.HTMLElement) error {
	if e == nil {
		return errors.New("HTML element is nil")
	}

	sourceURL := e.Request.URL.String()

	// Read detected content type from crawler context (set when IsStructuredContentPage returns true)
	var detectedContentType string
	if v := e.Request.Ctx.GetAny(DetectedContentTypeCtxKey); v != nil {
		if str, ok := v.(string); ok {
			detectedContentType = str
		} else {
			s.logger.Warn("detected_content_type context value is not a string",
				infralogger.String("url", sourceURL),
				infralogger.Any("value", v),
			)
		}
	}

	// Get source configuration to determine source name and selectors
	sourceName, selectors := s.getSourceConfig(sourceURL)

	// Extract raw content using generic extractor
	rawData := ExtractRawContent(
		e,
		sourceURL,
		selectors.Title,
		selectors.Body,
		selectors.Container,
		selectors.Exclude,
	)

	// Validate extracted content before indexing
	if rawData.Title == "" && rawData.RawText == "" {
		s.logger.Debug("Skipping page with no extractable content",
			infralogger.String("url", sourceURL))
		return nil
	}

	wordCount := len(strings.Fields(rawData.RawText))
	if wordCount < minPostExtractionWordCount {
		s.logger.Debug("Skipping page with insufficient content",
			infralogger.String("url", sourceURL),
			infralogger.Int("word_count", wordCount),
			infralogger.Int("min_word_count", minPostExtractionWordCount))
		return nil
	}

	// Ensure raw_content index exists
	ctx := context.Background()
	if err := s.rawIndexer.EnsureRawContentIndex(ctx, sourceName); err != nil {
		s.logger.Warn("Failed to ensure raw_content index, continuing anyway",
			infralogger.Error(err),
			infralogger.String("source_name", sourceName))
	}

	// Convert RawContentData to RawContent for indexing
	rawContent := s.convertToRawContent(rawData, sourceName, detectedContentType)

	// Index to raw_content (no validation - classifier will handle that)
	err := s.rawIndexer.IndexRawContent(ctx, rawContent)
	if err != nil {
		s.logger.Error("Failed to index raw content",
			infralogger.Error(err),
			infralogger.String("url", sourceURL),
			infralogger.String("source_name", sourceName))
		return fmt.Errorf("failed to index raw content: %w", err)
	}

	// Emit pipeline event (fire-and-forget)
	s.emitIndexedEvent(ctx, sourceURL, sourceName, rawData, rawContent)

	s.logger.Debug("Indexed raw content for classification",
		infralogger.String("url", sourceURL),
		infralogger.String("source_name", sourceName),
		infralogger.String("title", rawData.Title),
		infralogger.Int("word_count", rawContent.WordCount),
	)

	return nil
}

// emitIndexedEvent emits a pipeline event after successful raw content indexing.
func (s *RawContentService) emitIndexedEvent(
	ctx context.Context,
	sourceURL, sourceName string,
	rawData *RawContentData,
	rawContent *storagepkg.RawContent,
) {
	if s.pipeline == nil {
		return
	}

	indexName := sourceName + "_raw_content"
	pipelineErr := s.pipeline.Emit(ctx, pipeline.Event{
		ArticleURL: sourceURL,
		SourceName: sourceName,
		Stage:      "indexed",
		OccurredAt: time.Now(),
		Metadata: map[string]any{
			"title":       rawData.Title,
			"word_count":  rawContent.WordCount,
			"index_name":  indexName,
			"document_id": rawContent.ID,
		},
	})
	if pipelineErr != nil {
		s.logger.Warn("Failed to emit pipeline event",
			infralogger.Error(pipelineErr),
			infralogger.String("url", sourceURL),
			infralogger.String("stage", "indexed"),
		)
	}
}

// getSourceConfig gets the source configuration and returns source name and selectors.
func (s *RawContentService) getSourceConfig(sourceURL string) (string, SourceSelectors) {
	var sourceName string
	selectors := SourceSelectors{}

	if s.sources == nil {
		// No sources manager, use URL as source name
		sourceName = extractSourceNameFromURL(sourceURL)
		s.logger.Debug("No sources manager available, using URL-based source name",
			infralogger.String("source_name", sourceName),
			infralogger.String("url", sourceURL))
		return sourceName, selectors
	}

	// Try to find source by URL (matching domain)
	sourceConfig := s.findSourceByURL(sourceURL)
	if sourceConfig == nil {
		// Source not found, use URL as source name
		sourceName = extractSourceNameFromURL(sourceURL)
		s.logger.Debug("Source not found for URL, using URL-based source name",
			infralogger.String("url", sourceURL),
			infralogger.String("source_name", sourceName))
		return sourceName, selectors
	}

	// Use hostname from the URL being crawled, not the source's Name field
	// This ensures index names are based on URLs (e.g., "www.sudbury.com") rather than human-readable names
	sourceName = extractSourceNameFromURL(sourceURL)
	s.logger.Debug("Source found by URL, using URL-based source name for indexing",
		infralogger.String("url", sourceURL),
		infralogger.String("source_name", sourceName),
		infralogger.String("source_config_name", sourceConfig.Name))

	// Extract selectors from source config (if available)
	// We'll use article selectors as a guide, but won't enforce article-specific logic
	if sourceConfig.Selectors.Article.Title != "" {
		selectors.Title = sourceConfig.Selectors.Article.Title
	}
	if sourceConfig.Selectors.Article.Body != "" {
		selectors.Body = sourceConfig.Selectors.Article.Body
	}
	if sourceConfig.Selectors.Article.Container != "" {
		selectors.Container = sourceConfig.Selectors.Article.Container
	}
	if len(sourceConfig.Selectors.Article.Exclude) > 0 {
		selectors.Exclude = sourceConfig.Selectors.Article.Exclude
	}

	return sourceName, selectors
}

// SourceSelectors represents generic selectors for content extraction
type SourceSelectors struct {
	Title     string
	Body      string
	Container string
	Exclude   []string
}

// extractSourceNameFromURL extracts a source name from a URL
func extractSourceNameFromURL(urlStr string) string {
	// Simple extraction: use domain name
	// Example: https://example.com/article -> example_com
	urlStr = strings.TrimPrefix(urlStr, "http://")
	urlStr = strings.TrimPrefix(urlStr, "https://")

	parts := strings.Split(urlStr, "/")
	if len(parts) > 0 {
		domainName := parts[0]
		domainName = strings.ReplaceAll(domainName, ".", "_")
		domainName = strings.ToLower(domainName)
		return domainName
	}

	return "unknown_source"
}

// findSourceByURL attempts to find a source configuration by matching the URL domain.
func (s *RawContentService) findSourceByURL(pageURL string) *sources.Config {
	if s.sources == nil {
		return nil
	}

	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return nil
	}

	// Get all sources and try to match by domain
	sourceConfigs, err := s.sources.GetSources()
	if err != nil {
		return nil
	}

	for i := range sourceConfigs {
		source := &sourceConfigs[i]
		// Check if domain matches any allowed domain
		for _, allowedDomain := range source.AllowedDomains {
			if allowedDomain == hostname || allowedDomain == "*."+hostname {
				return source
			}
		}
		// Also check source URL
		if sourceParsedURL, parseErr := url.Parse(source.URL); parseErr == nil {
			if sourceParsedURL.Hostname() == hostname {
				return source
			}
		}
	}

	return nil
}

// convertToRawContent converts RawContentData to storage.RawContent for indexing
func (s *RawContentService) convertToRawContent(rawData *RawContentData, sourceName, detectedContentType string) *storagepkg.RawContent {
	// Calculate word count
	wordCount := calculateWordCount(rawData.RawText)

	// Use OG type as-is from HTML extraction
	// Don't default to "article" - let classifier decide based on content characteristics
	ogType := rawData.OGType
	// Keep empty if not present in HTML

	// Build meta object with additional metadata
	meta := make(map[string]any)
	if rawData.ArticleOpinion {
		meta["article_opinion"] = rawData.ArticleOpinion
	}
	if rawData.ArticleContentTier != "" {
		meta["article_content_tier"] = rawData.ArticleContentTier
	}
	if rawData.TwitterCard != "" {
		meta["twitter_card"] = rawData.TwitterCard
	}
	if rawData.TwitterSite != "" {
		meta["twitter_site"] = rawData.TwitterSite
	}
	if rawData.OGImageWidth > 0 {
		meta["og_image_width"] = rawData.OGImageWidth
	}
	if rawData.OGImageHeight > 0 {
		meta["og_image_height"] = rawData.OGImageHeight
	}
	if rawData.OGSiteName != "" {
		meta["og_site_name"] = rawData.OGSiteName
	}
	if detectedContentType != "" {
		meta["detected_content_type"] = detectedContentType
	}

	rawContent := &storagepkg.RawContent{
		ID:                   rawData.ID,
		URL:                  rawData.URL,
		SourceName:           sourceName,
		Title:                rawData.Title,
		RawText:              rawData.RawText,
		RawHTML:              rawData.RawHTML,
		MetaDescription:      rawData.MetaDescription,
		MetaKeywords:         rawData.MetaKeywords,
		OGType:               ogType,
		OGTitle:              rawData.OGTitle,
		OGDescription:        rawData.OGDescription,
		OGImage:              rawData.OGImage,
		Author:               rawData.Author,
		PublishedDate:        rawData.PublishedDate,
		CanonicalURL:         rawData.CanonicalURL,
		ArticleSection:       rawData.ArticleSection,
		JSONLDData:           rawData.JSONLDData,
		ClassificationStatus: "pending",
		CrawledAt:            time.Now(),
		WordCount:            wordCount,
	}
	// Defensive normalization so jsonld_raw never contains object/array for
	// publisher, author, image, mainEntityOfPage (avoids ES mapping conflicts).
	NormalizeJSONLDRawForIndex(rawContent.JSONLDData)

	// Add CreatedAt and UpdatedAt to meta if they exist
	if !rawData.CreatedAt.IsZero() {
		meta["created_at"] = rawData.CreatedAt
	}
	if !rawData.UpdatedAt.IsZero() {
		meta["updated_at"] = rawData.UpdatedAt
	}

	// Only add meta object if it has content
	if len(meta) > 0 {
		rawContent.Meta = meta
	}

	return rawContent
}

// calculateWordCount calculates word count from text
func calculateWordCount(text string) int {
	if text == "" {
		return 0
	}
	words := strings.Fields(text)
	return len(words)
}
