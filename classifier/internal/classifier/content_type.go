package classifier

import (
	"context"
	"net/url"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	// Content type confidence constants
	articleConfidence      = 0.75
	pageConfidence         = 0.6
	urlExclusionConfidence = 0.9
	listingPageConfidence  = 0.85
	// String literal for article type matching
	articleTypeString = "article"
	// Listing page detection thresholds
	minReadMoreCountForListing = 3
	minDateCountForListing     = 5
	minSummaryCountForListing  = 3
)

// alwaysExcludedPrefixes contains URL path prefixes that always indicate non-article content.
// These match as prefixes: /classifieds/job-listings is excluded because /classifieds is.
var alwaysExcludedPrefixes = []string{
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

// sectionIndexPaths contains section paths that are only excluded when they are the
// exact path (index/listing pages). Articles within these sections (e.g., /news/article-slug)
// should pass through to other classification strategies.
var sectionIndexPaths = []string{
	"/news", "/articles", "/stories", "/posts", "/blog",
	"/ontario-news", "/local-news", "/breaking-news",
}

// paginationQueryParams contains query parameter names that indicate pagination
var paginationQueryParams = []string{
	"page", "p", "pagenum", "paged", "page_num", "page_number",
	"offset", "start", "from",
}

// ContentTypeClassifier determines the type of content (article, page, video, image, job)
type ContentTypeClassifier struct {
	logger infralogger.Logger
}

// ContentTypeResult represents the result of content type classification
type ContentTypeResult struct {
	Type       string  // "article", "page", "video", "image", "job"
	Confidence float64 // 0.0-1.0
	Method     string  // "og_metadata", "heuristic", "default"
	Reason     string  // Human-readable explanation
}

// NewContentTypeClassifier creates a new content type classifier
func NewContentTypeClassifier(logger infralogger.Logger) *ContentTypeClassifier {
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
			infralogger.String("content_id", raw.ID),
			infralogger.String("url", raw.URL),
			infralogger.String("result", domain.ContentTypePage),
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

	// Strategy 2: Check for listing page content patterns (before article heuristics)
	// Listing pages often have multiple article links or "Read more" patterns
	if c.isListingPageContent(raw) {
		c.logger.Debug("Content type detected as listing page via content patterns",
			infralogger.String("content_id", raw.ID),
			infralogger.String("url", raw.URL),
			infralogger.String("result", domain.ContentTypePage),
		)
		return &ContentTypeResult{
			Type:       domain.ContentTypePage,
			Confidence: listingPageConfidence,
			Method:     "content_pattern",
			Reason:     "Content has listing page characteristics (multiple article links)",
		}, nil
	}

	// Strategy 3: Heuristic-based detection
	// Check if content has characteristics of an article
	if c.hasArticleCharacteristics(raw) {
		c.logger.Debug("Content type detected via heuristics",
			infralogger.String("content_id", raw.ID),
			infralogger.Int("word_count", raw.WordCount),
			infralogger.Bool("has_title", raw.Title != ""),
			infralogger.Bool("has_meta_description", raw.MetaDescription != ""),
			infralogger.String("result", domain.ContentTypeArticle),
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
		infralogger.String("content_id", raw.ID),
		infralogger.Int("word_count", raw.WordCount),
		infralogger.String("result", domain.ContentTypePage),
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
			infralogger.String("content_id", raw.ID),
			infralogger.String("og_type", ogType),
			infralogger.String("result", domain.ContentTypeArticle),
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
			infralogger.String("content_id", raw.ID),
		)
		return nil // Return nil to fall through to heuristic check
	}

	// Check for video
	if ogType == "video" || ogType == "video.other" || strings.Contains(ogType, "video") {
		c.logger.Debug("Content type detected via OG metadata",
			infralogger.String("content_id", raw.ID),
			infralogger.String("og_type", ogType),
			infralogger.String("result", domain.ContentTypeVideo),
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
			infralogger.String("content_id", raw.ID),
			infralogger.String("og_type", ogType),
			infralogger.String("result", domain.ContentTypeImage),
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

// hasPaginationQuery checks if the query string contains pagination parameters
func (c *ContentTypeClassifier) hasPaginationQuery(query string) bool {
	if query == "" {
		return false
	}

	// Parse query parameters
	values, err := url.ParseQuery(query)
	if err != nil {
		// If parsing fails, do simple string matching
		lowerQuery := strings.ToLower(query)
		for _, param := range paginationQueryParams {
			if strings.Contains(lowerQuery, param+"=") {
				return true
			}
		}
		return false
	}

	// Check if any pagination parameter exists and has a value
	for _, param := range paginationQueryParams {
		if values.Has(param) {
			// Only consider it pagination if the value is numeric (not empty or non-numeric)
			value := strings.TrimSpace(values.Get(param))
			if value != "" && c.isNumeric(value) {
				return true
			}
		}
	}

	return false
}

// isNumeric checks if a string represents a numeric value (integer)
func (c *ContentTypeClassifier) isNumeric(s string) bool {
	if s == "" {
		return false
	}
	// Check if string contains only digits (allowing negative sign)
	for i, r := range s {
		if i == 0 && r == '-' {
			continue // Allow negative sign at start
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != "" && (s[0] != '-' || len(s) > 1)
}

// matchesURLPattern checks if URL path matches pattern as a prefix (handles trailing slashes).
// Use for always-excluded paths where any subpath should also be excluded.
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

// isExactSectionPath checks if URL path exactly matches a section path.
// Matches "/news" and "/news/" but NOT "/news/article-slug".
func isExactSectionPath(path, section string) bool {
	return path == section || path == section+"/"
}

// isNonArticleURL checks if the URL matches patterns that indicate non-article content
func (c *ContentTypeClassifier) isNonArticleURL(urlStr string) bool {
	if urlStr == "" {
		return false
	}

	// Parse URL to get path
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return c.isNonArticleURLFallback(urlStr)
	}

	path := strings.ToLower(parsedURL.Path)

	// Check always-excluded prefixes (auth, classifieds, directory, etc.)
	if c.matchesAlwaysExcluded(urlStr, path) {
		return true
	}

	// Check section index paths (exact match only -- /news excluded, /news/article-slug is not)
	if c.matchesSectionIndex(urlStr, path) {
		return true
	}

	// Check for query parameters that indicate redirect/auth pages
	query := strings.ToLower(parsedURL.RawQuery)
	if strings.Contains(query, "returnurl=") || strings.Contains(query, "redirect=") {
		return true
	}

	// Check for pagination query parameters (indicates listing/index pages)
	if c.hasPaginationQuery(query) {
		c.logger.Debug("URL matched pagination query parameter",
			infralogger.String("url", urlStr),
			infralogger.String("query", query),
		)
		return true
	}

	// Homepage is typically not an article
	if path == "/" || path == "" {
		return true
	}

	return false
}

// isNonArticleURLFallback handles URL pattern matching when URL parsing fails.
func (c *ContentTypeClassifier) isNonArticleURLFallback(urlStr string) bool {
	lowerURL := strings.ToLower(urlStr)
	for _, pattern := range alwaysExcludedPrefixes {
		if strings.Contains(lowerURL, pattern) {
			return true
		}
	}
	for _, section := range sectionIndexPaths {
		if strings.Contains(lowerURL, section) {
			return true
		}
	}
	if strings.Contains(lowerURL, "returnurl=") || strings.Contains(lowerURL, "redirect=") {
		return true
	}
	return false
}

// matchesAlwaysExcluded checks if a path matches any always-excluded prefix pattern.
func (c *ContentTypeClassifier) matchesAlwaysExcluded(urlStr, path string) bool {
	for _, pattern := range alwaysExcludedPrefixes {
		if matchesURLPattern(path, pattern) {
			c.logger.Debug("URL matched always-excluded pattern",
				infralogger.String("url", urlStr),
				infralogger.String("path", path),
				infralogger.String("pattern", pattern),
			)
			return true
		}
	}
	return false
}

// matchesSectionIndex checks if a path exactly matches a section index path.
func (c *ContentTypeClassifier) matchesSectionIndex(urlStr, path string) bool {
	for _, section := range sectionIndexPaths {
		if isExactSectionPath(path, section) {
			c.logger.Debug("URL matched section index path",
				infralogger.String("url", urlStr),
				infralogger.String("path", path),
				infralogger.String("section", section),
			)
			return true
		}
	}
	return false
}

// isListingPageContent checks if content has characteristics of a listing/index page
// Listing pages typically have multiple article links, "Read more" patterns, or article summaries
func (c *ContentTypeClassifier) isListingPageContent(raw *domain.RawContent) bool {
	// Check raw text for listing page indicators
	lowerText := strings.ToLower(raw.RawText)

	// Count occurrences of "Read more" or similar patterns (strong indicator of listing pages)
	readMorePatterns := []string{"read more", "read more >", "read more>>", "continue reading", "full story"}
	readMoreCount := 0
	for _, pattern := range readMorePatterns {
		readMoreCount += strings.Count(lowerText, pattern)
	}

	// If we find 3+ "Read more" links, it's likely a listing page
	if readMoreCount >= minReadMoreCountForListing {
		return true
	}

	// Check for multiple article date patterns (listing pages often show multiple dates)
	// Pattern: "Dec 26, 2025" or "January 5, 2026" appearing multiple times
	datePatterns := []string{
		"jan ", "feb ", "mar ", "apr ", "may ", "jun ",
		"jul ", "aug ", "sep ", "oct ", "nov ", "dec ",
	}
	dateCount := 0
	for _, pattern := range datePatterns {
		dateCount += strings.Count(lowerText, pattern)
	}

	// If we find 5+ date mentions, it's likely a listing page with multiple articles
	if dateCount >= minDateCountForListing {
		return true
	}

	// Check for article summary patterns (listing pages have multiple article previews)
	// Pattern: Multiple instances of article datelines (e.g., "TORONTO —", "OTTAWA —")
	summaryIndicators := []string{
		"toronto —", "ottawa —", "ontario —", // News article datelines
		"vancouver —", "montreal —", "calgary —", "edmonton —",
	}
	summaryCount := 0
	for _, indicator := range summaryIndicators {
		summaryCount += strings.Count(lowerText, indicator)
	}

	// If we have 3+ datelines, it's likely a listing page with multiple article summaries
	if summaryCount >= minSummaryCountForListing {
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
