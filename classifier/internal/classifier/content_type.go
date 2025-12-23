package classifier

import (
	"context"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

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
	// Strategy 1: Check Open Graph metadata (highest confidence)
	if raw.OGType != "" {
		ogType := strings.ToLower(strings.TrimSpace(raw.OGType))

		// Check for article indicators
		if ogType == "article" || ogType == "news" || strings.Contains(ogType, "article") {
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
			}, nil
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
			Confidence: 0.75,
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
		Confidence: 0.6,
		Method:     "default",
		Reason:     "Content does not meet article criteria",
	}, nil
}

// hasArticleCharacteristics checks if the content has characteristics of an article
// Based on crawler's detection logic: word count threshold and metadata presence
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

	// Additional indicators that boost confidence
	hasDescription := raw.MetaDescription != "" || raw.OGDescription != ""
	hasAuthor := raw.OGTitle != "" // OG title often includes author info for news sites
	hasPublishedDate := raw.PublishedDate != nil

	// At least one additional indicator should be present
	return hasDescription || hasAuthor || hasPublishedDate
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
