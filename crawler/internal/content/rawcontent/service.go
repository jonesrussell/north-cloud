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
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	storagepkg "github.com/jonesrussell/north-cloud/crawler/internal/storage"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
)

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
	logger     logger.Interface
	storage    types.Interface
	sources    sources.Interface
	rawIndexer *storagepkg.RawContentIndexer
}

// NewRawContentService creates a new raw content service.
func NewRawContentService(
	log logger.Interface,
	storage types.Interface,
	sourcesManager sources.Interface,
) *RawContentService {
	rawIndexer := storagepkg.NewRawContentIndexer(storage, log)
	return &RawContentService{
		logger:     log,
		storage:    storage,
		sources:    sourcesManager,
		rawIndexer: rawIndexer,
	}
}

// Process implements the Interface for HTML element processing.
// Extracts raw content from any HTML page and indexes it to raw_content.
func (s *RawContentService) Process(e *colly.HTMLElement) error {
	if e == nil {
		return errors.New("HTML element is nil")
	}

	sourceURL := e.Request.URL.String()

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

	// Ensure raw_content index exists
	ctx := context.Background()
	if err := s.rawIndexer.EnsureRawContentIndex(ctx, sourceName); err != nil {
		s.logger.Warn("Failed to ensure raw_content index, continuing anyway",
			"error", err,
			"source_name", sourceName)
	}

	// Convert RawContentData to RawContent for indexing
	rawContent := s.convertToRawContent(rawData, sourceName)

	// Index to raw_content (no validation - classifier will handle that)
	err := s.rawIndexer.IndexRawContent(ctx, rawContent)
	if err != nil {
		s.logger.Error("Failed to index raw content",
			"error", err,
			"url", sourceURL,
			"source_name", sourceName)
		return fmt.Errorf("failed to index raw content: %w", err)
	}

	s.logger.Debug("Indexed raw content for classification",
		"url", sourceURL,
		"source_name", sourceName,
		"title", rawData.Title,
		"word_count", rawContent.WordCount)

	return nil
}

// getSourceConfig gets the source configuration and returns source name and selectors.
func (s *RawContentService) getSourceConfig(sourceURL string) (string, SourceSelectors) {
	var sourceName string
	selectors := SourceSelectors{}

	if s.sources == nil {
		// No sources manager, use URL as source name
		sourceName = extractSourceNameFromURL(sourceURL)
		s.logger.Debug("No sources manager available, using URL-based source name",
			"source_name", sourceName,
			"url", sourceURL)
		return sourceName, selectors
	}

	// Try to find source by URL (matching domain)
	sourceConfig := s.findSourceByURL(sourceURL)
	if sourceConfig == nil {
		// Source not found, use URL as source name
		sourceName = extractSourceNameFromURL(sourceURL)
		s.logger.Debug("Source not found for URL, using URL-based source name",
			"url", sourceURL,
			"source_name", sourceName)
		return sourceName, selectors
	}

	// Use hostname from the URL being crawled, not the source's Name field
	// This ensures index names are based on URLs (e.g., "www.sudbury.com") rather than human-readable names
	sourceName = extractSourceNameFromURL(sourceURL)
	s.logger.Debug("Source found by URL, using URL-based source name for indexing",
		"url", sourceURL,
		"source_name", sourceName,
		"source_config_name", sourceConfig.Name)

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
func (s *RawContentService) convertToRawContent(rawData *RawContentData, sourceName string) *storagepkg.RawContent {
	// Calculate word count
	wordCount := calculateWordCount(rawData.RawText)

	// Use OG type as-is from HTML extraction
	// Don't default to "article" - let classifier decide based on content characteristics
	ogType := rawData.OGType
	// Keep empty if not present in HTML

	return &storagepkg.RawContent{
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
		ClassificationStatus: "pending",
		CrawledAt:            time.Now(),
		WordCount:            wordCount,
	}
}

// calculateWordCount calculates word count from text
func calculateWordCount(text string) int {
	if text == "" {
		return 0
	}
	words := strings.Fields(text)
	return len(words)
}
