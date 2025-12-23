// Package articles provides functionality for processing and managing article content.
package articles

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/gocolly/colly/v2"
	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
)

// Interface defines the interface for processing articles.
// It handles the extraction and processing of article content from web pages.
type Interface interface {
	// Process handles the processing of article content.
	// It takes a colly.HTMLElement and processes the article found within it.
	Process(e *colly.HTMLElement) error

	// ProcessArticle processes an article and returns any errors
	ProcessArticle(ctx context.Context, article *domain.Article) error
}

// Ensure ContentService implements Interface
var _ Interface = (*ContentService)(nil)

// ContentService implements both Interface and ServiceInterface for article processing.
type ContentService struct {
	logger     logger.Interface
	storage    types.Interface
	indexName  string
	sources    sources.Interface
	validator  *ArticleValidator
	rawIndexer RawContentIndexer // NEW: For raw content indexing
}

// RawContentIndexer defines the interface for indexing raw content
type RawContentIndexer interface {
	IndexArticle(ctx context.Context, article *domain.Article, sourceName string) error
	EnsureRawContentIndex(ctx context.Context, sourceName string) error
}

// NewContentService creates a new article service.
func NewContentService(log logger.Interface, storage types.Interface, indexName string) *ContentService {
	return &ContentService{
		logger:    log,
		storage:   storage,
		indexName: indexName,
		validator: NewArticleValidator(log),
	}
}

// NewContentServiceWithSources creates a new article service with sources access.
func NewContentServiceWithSources(
	log logger.Interface,
	storage types.Interface,
	indexName string,
	sourcesManager sources.Interface,
) *ContentService {
	return &ContentService{
		logger:    log,
		storage:   storage,
		indexName: indexName,
		sources:   sourcesManager,
		validator: NewArticleValidator(log),
	}
}

// NewContentServiceWithRawIndexer creates a new article service with raw content indexing.
func NewContentServiceWithRawIndexer(
	log logger.Interface,
	storage types.Interface,
	indexName string,
	sourcesManager sources.Interface,
	rawIndexer RawContentIndexer,
) *ContentService {
	return &ContentService{
		logger:     log,
		storage:    storage,
		indexName:  indexName,
		sources:    sourcesManager,
		validator:  NewArticleValidator(log),
		rawIndexer: rawIndexer,
	}
}

// Process implements the Interface for HTML element processing.
func (s *ContentService) Process(e *colly.HTMLElement) error {
	if e == nil {
		return errors.New("HTML element is nil")
	}

	sourceURL := e.Request.URL.String()

	// Get source configuration and determine index name
	// Use local variable to avoid data race when Process() is called concurrently
	indexName, selectors := s.getSourceConfigAndIndex(sourceURL)

	// Extract article data using Colly methods
	articleData := extractArticle(e, selectors, sourceURL)

	// Clean category field
	categories := CleanCategory(articleData.Category)
	categoryStr := ""
	if len(categories) > 0 {
		categoryStr = categories[0] // Use first category as primary
	}

	// Calculate word count
	wordCount := CalculateWordCount(articleData.Body)

	// Convert to domain.Article
	article := &domain.Article{
		ID:            articleData.ID,
		Title:         articleData.Title,
		Body:          articleData.Body,
		Intro:         articleData.Intro,
		Author:        articleData.Author,
		BylineName:    articleData.BylineName,
		PublishedDate: articleData.PublishedDate,
		Source:        articleData.Source,
		Tags:          articleData.Tags,
		Keywords:      articleData.Keywords,
		Description:   articleData.Description,
		Section:       articleData.Section,
		Category:      categoryStr,
		OgTitle:       articleData.OgTitle,
		OgDescription: articleData.OgDescription,
		OgImage:       articleData.OgImage,
		OgURL:         articleData.OgURL,
		CanonicalURL:  articleData.CanonicalURL,
		WordCount:     wordCount,
		CreatedAt:     articleData.CreatedAt,
		UpdatedAt:     articleData.UpdatedAt,
	}

	// Validate article before indexing
	validationResult := s.validator.ValidateArticle(article)
	if !validationResult.IsValid {
		s.logger.Warn("Article validation failed, skipping index",
			"url", article.Source,
			"articleID", article.ID,
			"reason", validationResult.Reason,
			"title", article.Title)
		return fmt.Errorf("article validation failed: %s", validationResult.Reason)
	}

	// Process the article using the service interface with the determined index name
	s.logger.Debug("Processing article with index",
		"index_name", indexName,
		"article_id", article.ID,
		"url", article.Source)
	return s.ProcessArticleWithIndex(context.Background(), article, indexName)
}

// getSourceConfigAndIndex gets the source configuration and determines the index name.
func (s *ContentService) getSourceConfigAndIndex(sourceURL string) (string, configtypes.ArticleSelectors) {
	indexName := s.indexName
	var selectors configtypes.ArticleSelectors

	if s.sources == nil {
		s.logger.Debug("No sources manager available, using default index",
			"default_index", indexName,
			"url", sourceURL)
		return indexName, selectors.Default()
	}

	sourceConfig := s.findSourceByURL(sourceURL)
	if sourceConfig == nil {
		s.logger.Debug("Source not found for URL, using default selectors and index",
			"url", sourceURL,
			"default_index", indexName)
		return indexName, selectors.Default()
	}

	s.logger.Debug("Source found by URL, using source-specific index",
		"url", sourceURL,
		"source_name", sourceConfig.Name,
		"article_index", sourceConfig.ArticleIndex,
		"default_index", indexName)

	selectors = s.convertSourceSelectors(sourceConfig)
	selectors = s.mergeSelectorsWithDefaults(selectors)

	if sourceConfig.ArticleIndex != "" {
		indexName = sourceConfig.ArticleIndex
		s.logger.Debug("Using source-specific article index",
			"index_name", indexName,
			"source_name", sourceConfig.Name,
			"url", sourceURL)
	} else {
		s.logger.Debug("Source found but ArticleIndex is empty, using default index",
			"default_index", indexName,
			"source_name", sourceConfig.Name,
			"url", sourceURL)
	}

	return indexName, selectors
}

// convertSourceSelectors converts source selectors to configtypes.ArticleSelectors.
func (s *ContentService) convertSourceSelectors(sourceConfig *sources.Config) configtypes.ArticleSelectors {
	return configtypes.ArticleSelectors{
		Container:     sourceConfig.Selectors.Article.Container,
		Title:         sourceConfig.Selectors.Article.Title,
		Body:          sourceConfig.Selectors.Article.Body,
		Intro:         sourceConfig.Selectors.Article.Intro,
		Link:          sourceConfig.Selectors.Article.Link,
		Image:         sourceConfig.Selectors.Article.Image,
		Byline:        sourceConfig.Selectors.Article.Byline,
		PublishedTime: sourceConfig.Selectors.Article.PublishedTime,
		TimeAgo:       sourceConfig.Selectors.Article.TimeAgo,
		JSONLD:        sourceConfig.Selectors.Article.JSONLD,
		Section:       sourceConfig.Selectors.Article.Section,
		Keywords:      sourceConfig.Selectors.Article.Keywords,
		Description:   sourceConfig.Selectors.Article.Description,
		OGTitle:       sourceConfig.Selectors.Article.OGTitle,
		OGDescription: sourceConfig.Selectors.Article.OGDescription,
		OGImage:       sourceConfig.Selectors.Article.OGImage,
		OGType:        sourceConfig.Selectors.Article.OGType,
		OGSiteName:    sourceConfig.Selectors.Article.OGSiteName,
		OgURL:         sourceConfig.Selectors.Article.OgURL,
		Canonical:     sourceConfig.Selectors.Article.Canonical,
		WordCount:     sourceConfig.Selectors.Article.WordCount,
		PublishDate:   sourceConfig.Selectors.Article.PublishDate,
		Category:      sourceConfig.Selectors.Article.Category,
		Tags:          sourceConfig.Selectors.Article.Tags,
		Author:        sourceConfig.Selectors.Article.Author,
		BylineName:    sourceConfig.Selectors.Article.BylineName,
		ArticleID:     sourceConfig.Selectors.Article.ArticleID,
		Exclude:       sourceConfig.Selectors.Article.Exclude,
	}
}

// mergeSelectorsWithDefaults merges selectors with default values for empty fields.
func (s *ContentService) mergeSelectorsWithDefaults(
	selectors configtypes.ArticleSelectors,
) configtypes.ArticleSelectors {
	defaults := selectors.Default()
	if selectors.Container == "" {
		selectors.Container = defaults.Container
	}
	if selectors.Body == "" {
		selectors.Body = defaults.Body
	}
	if selectors.Title == "" {
		selectors.Title = defaults.Title
	}
	if selectors.Intro == "" {
		selectors.Intro = defaults.Intro
	}
	if selectors.Byline == "" {
		selectors.Byline = defaults.Byline
	}
	if selectors.PublishedTime == "" {
		selectors.PublishedTime = defaults.PublishedTime
	}
	if selectors.JSONLD == "" {
		selectors.JSONLD = defaults.JSONLD
	}
	if selectors.Description == "" {
		selectors.Description = defaults.Description
	}
	if selectors.Keywords == "" {
		selectors.Keywords = defaults.Keywords
	}
	if selectors.OGTitle == "" {
		selectors.OGTitle = defaults.OGTitle
	}
	if selectors.OGDescription == "" {
		selectors.OGDescription = defaults.OGDescription
	}
	if selectors.OGImage == "" {
		selectors.OGImage = defaults.OGImage
	}
	if selectors.Canonical == "" {
		selectors.Canonical = defaults.Canonical
	}
	return selectors
}

// findSourceByURL attempts to find a source configuration by matching the URL domain.
func (s *ContentService) findSourceByURL(pageURL string) *sources.Config {
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

// ProcessArticle implements the ServiceInterface for article processing.
func (s *ContentService) ProcessArticle(ctx context.Context, article *domain.Article) error {
	return s.ProcessArticleWithIndex(ctx, article, s.indexName)
}

// ProcessArticleWithIndex processes an article and indexes it to the specified index.
// This method uses a local indexName parameter to avoid data races when called concurrently.
func (s *ContentService) ProcessArticleWithIndex(ctx context.Context, article *domain.Article, indexName string) error {
	if article == nil {
		return errors.New("article is nil")
	}

	if article.ID == "" {
		return errors.New("article ID is required")
	}

	if article.Source == "" {
		return errors.New("article source URL is required")
	}

	// Ensure word count is calculated if not set
	if article.WordCount == 0 {
		article.WordCount = CalculateWordCount(article.Body)
	}

	// Clean category if needed
	if article.Category != "" {
		categories := CleanCategory(article.Category)
		if len(categories) > 0 {
			article.Category = categories[0]
		} else {
			article.Category = ""
		}
	}

	// Validate article before indexing (if validator is set)
	if s.validator != nil {
		validationResult := s.validator.ValidateArticle(article)
		if !validationResult.IsValid {
			s.logger.Warn("Article validation failed, skipping index",
				"url", article.Source,
				"articleID", article.ID,
				"reason", validationResult.Reason,
				"title", article.Title)
			return fmt.Errorf("article validation failed: %s", validationResult.Reason)
		}
	}

	// Prepare article for indexing: clean empty fields, normalize arrays, prevent duplication
	article.PrepareForIndexing()

	// Index as raw content for classification (REPLACES article indexing)
	if s.rawIndexer != nil {
		sourceName := s.extractSourceName(article.Source)

		if err := s.rawIndexer.IndexArticle(ctx, article, sourceName); err != nil {
			s.logger.Error("Failed to index raw content",
				"error", err,
				"articleID", article.ID,
				"url", article.Source,
				"source_name", sourceName)
			return fmt.Errorf("failed to index raw content: %w", err)
		}

		s.logger.Info("Raw content indexed for classification",
			"articleID", article.ID,
			"url", article.Source,
			"source_name", sourceName,
			"classification_status", "pending",
			"title", article.Title,
			"wordCount", article.WordCount,
			"publishedDate", article.PublishedDate.Format(time.RFC3339),
			"category", article.Category)
	} else {
		s.logger.Warn("RawContentIndexer not configured, skipping indexing",
			"articleID", article.ID,
			"url", article.Source)
		return errors.New("raw content indexer not configured")
	}

	return nil
}

// extractSourceName extracts the source name (hostname) from a URL for index naming
// Example: "https://example.com/article" → "example.com"
func (s *ContentService) extractSourceName(sourceURL string) string {
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		s.logger.Warn("Failed to parse source URL",
			"url", sourceURL,
			"error", err)
		return "unknown"
	}
	hostname := parsed.Hostname()
	if hostname == "" {
		return "unknown"
	}
	return hostname
}
