package classifier

import (
	"context"
	"net/url"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

const (
	// Content type confidence constants
	articleConfidence = 0.75
	pageConfidence    = 0.6
	// String literal for article type matching
	articleTypeString = "article"
)

// nonArticleURLPatterns contains URL path patterns that indicate non-article content
var nonArticleURLPatterns = []string{
	"/account/", "/login", "/signin", "/signup", "/register",
	"/classifieds/", "/classified/", "/ads/", "/advertisements/",
	"/category/", "/categories/", "/browse/", "/listings/",
	"/search", "/results",
}

// ContentTypeClassifier determines the type of content (article, page, video, image, job)
type ContentTypeClassifier struct {
	logger Logger
}

// ContentTypeResult represents the result of content type classification
type ContentTypeResult struct {
	Type       string  // "article", "page", "video", "image", "job"
	Confidence float64 // 0.0-1.0
	Method     string  // "og_metadata", "heuristic", "default"
	Reason     string  // Human-readable explanation
}

// NewContentTypeClassifier creates a new content type classifier
func NewContentTypeClassifier(logger Logger) *ContentTypeClassifier {
	return &ContentTypeClassifier{
		logger: logger,
	}
}

// Classify determines the content type of the given raw content
// This is ported from crawler's html_processor.go DetectContentType logic
func (c *ContentTypeClassifier) Classify(ctx context.Context, raw *domain.RawContent) (*ContentTypeResult, error) {
	// Strategy 0: Check URL exclusions first (non-article patterns)
	if c.isNonArticleURL(raw.URL) {
		c.logger.Debug("Content type excluded via URL pattern",
			"content_id", raw.ID,
			"url", raw.URL,
			"result", domain.ContentTypePage,
		)
		return &ContentTypeResult{
			Type:       domain.ContentTypePage,
			Confidence: 0.9,
			Method:     "url_exclusion",
			Reason:     "URL pattern indicates non-article content",
		}, nil
	}

	// Strategy 1: Check Open Graph metadata (with validation)
	if raw.OGType != "" {
		ogType := strings.ToLower(strings.TrimSpace(raw.OGType))

		// Check for article indicators
		if ogType == articleTypeString || ogType == "news" || strings.Contains(ogType, articleTypeString) {
			// Validate OGType with additional indicators - require published date for high confidence
			if raw.PublishedDate != nil {
				c.logger.Debug("Content type detected via OG metadata with published date",
					"content_id", raw.ID,
					"og_type", ogType,
					"result", domain.ContentTypeArticle,
				)
				return &ContentTypeResult{
					Type:       domain.ContentTypeArticle,
					Confidence: 1.0,
					Method:     "og_metadata",
					Reason:     "Open Graph type indicates article content with published date",
				}, nil
			}
			// OGType says article but no published date - be cautious, classify as page
			c.logger.Debug("OGType indicates article but missing published date, classifying as page",
				"content_id", raw.ID,
				"og_type", ogType,
				"result", domain.ContentTypePage,
			)
			return &ContentTypeResult{
				Type:       domain.ContentTypePage,
				Confidence: 0.7,
				Method:     "og_metadata_validation",
				Reason:     "OGType indicates article but missing published date (validation failed)",
			}, nil
		}

		// Don't trust "website" OGType - it's the default and not meaningful
		if ogType == "website" {
			c.logger.Debug("OGType is 'website' (default), ignoring and using heuristics",
				"content_id", raw.ID,
			)
			// Fall through to heuristic check
		}

		// Check for video
		if ogType == "video" || ogType == "video.other" || strings.Contains(ogType, "video") {
			c.logger.Debug("Content type detected via OG metadata",
				"content_id", raw.ID,
				"og_type", ogType,
				"result", domain.ContentTypeVideo,
			)
			return &ContentTypeResult{
				Type:       domain.ContentTypeVideo,
				Confidence: 1.0,
				Method:     "og_metadata",
				Reason:     "Open Graph type indicates video content",
			}, nil
		}

		// Check for image
		if ogType == "image" || strings.Contains(ogType, "image") {
			c.logger.Debug("Content type detected via OG metadata",
				"content_id", raw.ID,
				"og_type", ogType,
				"result", domain.ContentTypeImage,
			)
			return &ContentTypeResult{
				Type:       domain.ContentTypeImage,
				Confidence: 1.0,
				Method:     "og_metadata",
				Reason:     "Open Graph type indicates image content",
			}, nil
		}
	}

	// Strategy 2: Heuristic-based detection
	// Check if content has characteristics of an article
	if c.hasArticleCharacteristics(raw) {
		c.logger.Debug("Content type detected via heuristics",
			"content_id", raw.ID,
			"word_count", raw.WordCount,
			"has_title", raw.Title != "",
			"has_meta_description", raw.MetaDescription != "",
			"result", domain.ContentTypeArticle,
		)
		return &ContentTypeResult{
			Type:       domain.ContentTypeArticle,
			Confidence: articleConfidence,
			Method:     "heuristic",
			Reason:     "Content has article characteristics (sufficient length, metadata)",
		}, nil
	}

	// Default: page
	c.logger.Debug("Content type defaulted to page",
		"content_id", raw.ID,
		"word_count", raw.WordCount,
		"result", domain.ContentTypePage,
	)
	return &ContentTypeResult{
		Type:       domain.ContentTypePage,
		Confidence: pageConfidence,
		Method:     "default",
		Reason:     "Content does not meet article criteria",
	}, nil
}

// isNonArticleURL checks if the URL matches patterns that indicate non-article content
func (c *ContentTypeClassifier) isNonArticleURL(urlStr string) bool {
	if urlStr == "" {
		return false
	}

	// Parse URL to get path
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		// If parsing fails, check raw string for patterns
		lowerURL := strings.ToLower(urlStr)
		for _, pattern := range nonArticleURLPatterns {
			if strings.Contains(lowerURL, pattern) {
				return true
			}
		}
		// Check for query parameters that indicate redirect/auth pages
		if strings.Contains(lowerURL, "returnurl=") || strings.Contains(lowerURL, "redirect=") {
			return true
		}
		return false
	}

	path := strings.ToLower(parsedURL.Path)

	// Check path patterns
	for _, pattern := range nonArticleURLPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Check for query parameters that indicate redirect/auth pages
	query := strings.ToLower(parsedURL.RawQuery)
	if strings.Contains(query, "returnurl=") || strings.Contains(query, "redirect=") {
		return true
	}

	// Homepage is typically not an article
	if path == "/" || path == "" {
		return true
	}

	return false
}

// hasArticleCharacteristics checks if the content has characteristics of an article
// Strengthened to require published date for heuristic-based classification
func (c *ContentTypeClassifier) hasArticleCharacteristics(raw *domain.RawContent) bool {
	// Minimum word count for articles (from crawler: MinArticleBodyLength = 200)
	const minArticleWordCount = 200

	// Must have minimum word count
	if raw.WordCount < minArticleWordCount {
		return false
	}

	// Should have a title
	if raw.Title == "" {
		return false
	}

	// Require published date - strongest indicator of article content
	if raw.PublishedDate == nil {
		return false
	}

	// Require description (meta or OG) - not just OGTitle which is too common
	hasDescription := raw.MetaDescription != "" || raw.OGDescription != ""
	if !hasDescription {
		return false
	}

	return true
}

// ClassifyBatch classifies multiple content items efficiently
func (c *ContentTypeClassifier) ClassifyBatch(ctx context.Context, rawItems []*domain.RawContent) ([]*ContentTypeResult, error) {
	results := make([]*ContentTypeResult, len(rawItems))

	for i, raw := range rawItems {
		result, err := c.Classify(ctx, raw)
		if err != nil {
			return nil, err
		}
		results[i] = result
	}

	return results, nil
}

// GetStats returns statistics about content type classifications
func (c *ContentTypeClassifier) GetStats() map[string]int {
	// TODO: Implement stats tracking
	// This would track counts of each content type classified
	return map[string]int{
		"article": 0,
		"page":    0,
		"video":   0,
		"image":   0,
		"job":     0,
	}
}
