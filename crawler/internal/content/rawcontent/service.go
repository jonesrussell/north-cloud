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
	"github.com/jonesrussell/gocrawl/internal/domain"
	"github.com/jonesrussell/gocrawl/internal/logger"
	"github.com/jonesrussell/gocrawl/internal/sources"
	storagepkg "github.com/jonesrussell/gocrawl/internal/storage"
	"github.com/jonesrussell/gocrawl/internal/storage/types"
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

	// Convert to domain.Article format for indexing (temporary compatibility)
	// The rawIndexer expects domain.Article but we'll populate it with raw content
	article := s.convertRawDataToArticle(rawData)

	// Index to raw_content (no validation - classifier will handle that)
	err := s.rawIndexer.IndexArticle(ctx, article, sourceName)
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
		"word_count", article.WordCount)

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

// convertRawDataToArticle converts RawContentData to domain.Article for indexing compatibility
func (s *RawContentService) convertRawDataToArticle(rawData *RawContentData) *domain.Article {
	// Calculate word count
	wordCount := calculateWordCount(rawData.RawText)

	// Convert keywords from string to slice
	var keywords []string
	if rawData.MetaKeywords != "" {
		keywords = strings.Split(rawData.MetaKeywords, ",")
		for i := range keywords {
			keywords[i] = strings.TrimSpace(keywords[i])
		}
	}

	article := &domain.Article{
		ID:            rawData.ID,
		Title:         rawData.Title,
		Body:          rawData.RawText,
		Intro:         rawData.MetaDescription,
		Author:        rawData.Author,
		PublishedDate: time.Time{},
		Source:        rawData.URL,
		Tags:          []string{},
		Keywords:      keywords,
		Description:   rawData.MetaDescription,
		OgTitle:       rawData.OGTitle,
		OgDescription: rawData.OGDescription,
		OgImage:       rawData.OGImage,
		OgURL:         rawData.OGURL,
		CanonicalURL:  rawData.CanonicalURL,
		WordCount:     wordCount,
		CreatedAt:     rawData.CreatedAt,
		UpdatedAt:     rawData.UpdatedAt,
	}

	// Set published date if available
	if rawData.PublishedDate != nil {
		article.PublishedDate = *rawData.PublishedDate
	}

	return article
}

// calculateWordCount calculates word count from text
func calculateWordCount(text string) int {
	if text == "" {
		return 0
	}
	words := strings.Fields(text)
	return len(words)
}
