package classifier

import (
	"context"
	"net/url"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

const (
	// Content type confidence constants
	articleConfidence          = 0.75
	pageConfidence             = 0.6
	urlExclusionConfidence     = 0.9
	ogTypeValidationConfidence = 0.7
	// String literal for article type matching
	articleTypeString = "article"
)

// nonArticleURLPatterns contains URL path patterns that indicate non-article content
// Note: matchesURLPattern() handles both trailing and non-trailing slashes
var nonArticleURLPatterns = []string{
	// Account/Auth pages
	"/account", "/login", "/signin", "/signup", "/register",

	// Classifieds
	"/classifieds", "/classified", "/ads", "/advertisements",

	// Directory and submissions
	"/directory", "/submissions",

	// Browsing/navigation pages
	"/category", "/categories", "/browse", "/listings",
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
			Confidence: urlExclusionConfidence,
			Method:     "url_exclusion",
			Reason:     "URL pattern indicates non-article content",
		}, nil
	}

	// Strategy 1: Check Open Graph metadata (with validation)
	if raw.OGType != "" {
		result := c.classifyFromOGType(raw)
		if result != nil {
			return result, nil
		}
		// If result is nil, fall through to heuristic check
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

// classifyFromOGType classifies content based on Open Graph type metadata
// Returns nil if OGType should be ignored (e.g., "website") or doesn't match known types
func (c *ContentTypeClassifier) classifyFromOGType(raw *domain.RawContent) *ContentTypeResult {
	ogType := strings.ToLower(strings.TrimSpace(raw.OGType))

	// If og_type is empty, fall through immediately
	// (After crawler fix, this means page had no explicit og:type tag)
	if ogType == "" {
		return nil
	}

	// Check for article indicators
	// Trust og_type as authoritative - if it says "article", it's an article
	if ogType == articleTypeString || ogType == "news" || strings.Contains(ogType, articleTypeString) {
		c.logger.Debug("Content type detected via OG metadata",
			"content_id", raw.ID,
			"og_type", ogType,
			"result", domain.ContentTypeArticle,
		)
		return &ContentTypeResult{
			Type:       domain.ContentTypeArticle,
			Confidence: 1.0,
			Method:     "og_metadata",
			Reason:     "Open Graph type indicates article content",
		}
	}

	// Don't trust "website" OGType - it's the default and not meaningful
	if ogType == "website" {
		c.logger.Debug("OGType is 'website' (default), ignoring and using heuristics",
			"content_id", raw.ID,
		)
		return nil // Return nil to fall through to heuristic check
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
		}
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
		}
	}

	return nil // Unknown OGType, fall through to heuristic check
}

// matchesURLPattern checks if URL path matches pattern (handles trailing slashes intelligently)
func matchesURLPattern(path, pattern string) bool {
	// Exact match
	if path == pattern {
		return true
	}

	// Pattern with trailing slash matches path prefix
	// e.g., pattern="/classifieds/" matches "/classifieds/job-listings"
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(path, pattern)
	}

	// Pattern without trailing slash matches:
	// 1. Exact path (already checked above)
	// 2. Path prefix with slash appended
	// e.g., pattern="/classifieds" matches "/classifieds/job-listings"
	return strings.HasPrefix(path, pattern+"/")
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

	// Check path patterns - use new helper function
	for _, pattern := range nonArticleURLPatterns {
		if matchesURLPattern(path, pattern) {
			c.logger.Debug("URL matched non-article pattern",
				"url", urlStr,
				"path", path,
				"pattern", pattern,
			)
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
	return hasDescription
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
