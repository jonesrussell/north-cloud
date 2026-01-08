package domain

import "time"

// ClassificationResult represents the result of classifying content
type ClassificationResult struct {
	ContentID string `json:"content_id"`

	// Content type classification
	ContentType    string  `json:"content_type"`    // "article", "page", "video", "image", "job"
	ContentSubtype string  `json:"content_subtype"` // e.g., "news_article", "blog_post"
	TypeConfidence float64 `json:"type_confidence"` // 0.0-1.0
	TypeMethod     string  `json:"type_method"`     // "og_metadata", "selector_based", "heuristic", "ml_model"

	// Quality scoring
	QualityScore   int                    `json:"quality_score"`   // 0-100
	QualityFactors map[string]any `json:"quality_factors"` // Breakdown of quality score

	// Topic classification
	Topics      []string           `json:"topics"`       // e.g., ["crime", "local_news"]
	TopicScores map[string]float64 `json:"topic_scores"` // e.g., {"crime": 0.95}

	// Source reputation
	SourceReputation int    `json:"source_reputation"` // 0-100
	SourceCategory   string `json:"source_category"`   // "news", "blog", "government", "unknown"

	// Classification metadata
	ClassifierVersion    string    `json:"classifier_version"`      // e.g., "1.0.0"
	ClassificationMethod string    `json:"classification_method"`   // "rule_based", "ml_model", "hybrid"
	ModelVersion         string    `json:"model_version,omitempty"` // For ML models
	Confidence           float64   `json:"confidence"`              // Overall confidence (0.0-1.0)
	ProcessingTimeMs     int64     `json:"processing_time_ms"`      // Processing duration
	ClassifiedAt         time.Time `json:"classified_at"`
}

// ClassifiedContent represents the full enriched document for Elasticsearch
// This combines RawContent + ClassificationResult
type ClassifiedContent struct {
	RawContent

	// Classification results (flattened for ES indexing)
	ContentType      string                 `json:"content_type"`
	ContentSubtype   string                 `json:"content_subtype,omitempty"`
	QualityScore     int                    `json:"quality_score"`
	QualityFactors   map[string]any `json:"quality_factors"`
	Topics           []string               `json:"topics"`
	TopicScores      map[string]float64     `json:"topic_scores"`
	SourceReputation int                    `json:"source_reputation"`
	SourceCategory   string                 `json:"source_category"`

	// Classification metadata
	ClassifierVersion    string  `json:"classifier_version"`
	ClassificationMethod string  `json:"classification_method"`
	ModelVersion         string  `json:"model_version,omitempty"`
	Confidence           float64 `json:"confidence"`

	// Publisher compatibility aliases
	// These duplicate RawContent fields for backward compatibility with publisher
	Body   string `json:"body"`   // Alias for RawText (publisher expects "body")
	Source string `json:"source"` // Alias for URL (publisher expects "source")
}

// ContentType constants
const (
	ContentTypeArticle = "article"
	ContentTypePage    = "page"
	ContentTypeVideo   = "video"
	ContentTypeImage   = "image"
	ContentTypeJob     = "job"
)

// SourceCategory constants
const (
	SourceCategoryNews       = "news"
	SourceCategoryBlog       = "blog"
	SourceCategoryGovernment = "government"
	SourceCategoryUnknown    = "unknown"
)

// ClassificationMethod constants
const (
	MethodRuleBased = "rule_based"
	MethodMLModel   = "ml_model"
	MethodHybrid    = "hybrid"
)
