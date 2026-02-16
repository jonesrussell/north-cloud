package domain

import "time"

// ClassificationResult represents the result of classifying content
type ClassificationResult struct {
	ContentID string `json:"content_id"`

	// Content type classification
	ContentType    string  `json:"content_type"`    // "article", "page", "video", "image", "job"
	ContentSubtype string  `json:"content_subtype"` // e.g., "press_release", "blog_post", "event"
	TypeConfidence float64 `json:"type_confidence"` // 0.0-1.0
	TypeMethod     string  `json:"type_method"`     // "detected_content_type", "url_exclusion", "og_metadata", "content_pattern", "heuristic"

	// Quality scoring
	QualityScore   int            `json:"quality_score"`   // 0-100
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

	// Crime hybrid classification (optional)
	Crime *CrimeResult `json:"crime,omitempty"`

	// Mining hybrid classification (optional)
	Mining *MiningResult `json:"mining,omitempty"`

	// Coforge hybrid classification (optional)
	Coforge *CoforgeResult `json:"coforge,omitempty"`

	// Entertainment hybrid classification (optional)
	Entertainment *EntertainmentResult `json:"entertainment,omitempty"`

	// Anishinaabe hybrid classification (optional)
	Anishinaabe *AnishinaabeResult `json:"anishinaabe,omitempty"`

	// Location detection (content-based)
	Location *LocationResult `json:"location,omitempty"`
}

// AnishinaabeResult holds Anishinaabe hybrid classification results.
type AnishinaabeResult struct {
	Relevance       string   `json:"relevance"`
	Categories      []string `json:"categories"`
	FinalConfidence float64  `json:"final_confidence"`
	ReviewRequired  bool     `json:"review_required"`
	ModelVersion    string   `json:"model_version,omitempty"`
}

// EntertainmentResult holds Entertainment hybrid classification results.
type EntertainmentResult struct {
	Relevance        string   `json:"relevance"`
	Categories       []string `json:"categories"`
	FinalConfidence  float64  `json:"final_confidence"`
	HomepageEligible bool     `json:"homepage_eligible"`
	ReviewRequired   bool     `json:"review_required"`
	ModelVersion     string   `json:"model_version,omitempty"`
}

// MiningResult holds Mining hybrid classification results.
type MiningResult struct {
	Relevance       string   `json:"relevance"`
	MiningStage     string   `json:"mining_stage"`
	Commodities     []string `json:"commodities"`
	Location        string   `json:"location"`
	FinalConfidence float64  `json:"final_confidence"`
	ReviewRequired  bool     `json:"review_required"`
	ModelVersion    string   `json:"model_version,omitempty"`
	SourceTextUsed  string   `json:"-"` // Internal only, for debugging
}

// CoforgeResult holds Coforge hybrid classification results.
type CoforgeResult struct {
	Relevance           string   `json:"relevance"`
	RelevanceConfidence float64  `json:"relevance_confidence"`
	Audience            string   `json:"audience"`
	AudienceConfidence  float64  `json:"audience_confidence"`
	Topics              []string `json:"topics"`
	Industries          []string `json:"industries"`
	FinalConfidence     float64  `json:"final_confidence"`
	ReviewRequired      bool     `json:"review_required"`
	ModelVersion        string   `json:"model_version,omitempty"`
}

// CrimeResult holds Crime hybrid classification results.
type CrimeResult struct {
	Relevance           string   `json:"street_crime_relevance"`
	SubLabel            string   `json:"sub_label,omitempty"` // "criminal_justice" or "crime_context" for peripheral_crime
	CrimeTypes          []string `json:"crime_types"`
	LocationSpecificity string   `json:"location_specificity"`
	FinalConfidence     float64  `json:"final_confidence"`
	HomepageEligible    bool     `json:"homepage_eligible"`
	CategoryPages       []string `json:"category_pages"`
	ReviewRequired      bool     `json:"review_required"`
}

// ClassifiedContent represents the full enriched document for Elasticsearch
// This combines RawContent + ClassificationResult
type ClassifiedContent struct {
	RawContent

	// Classification results (flattened for ES indexing)
	ContentType      string             `json:"content_type"`
	ContentSubtype   string             `json:"content_subtype,omitempty"`
	QualityScore     int                `json:"quality_score"`
	QualityFactors   map[string]any     `json:"quality_factors"`
	Topics           []string           `json:"topics"`
	TopicScores      map[string]float64 `json:"topic_scores"`
	SourceReputation int                `json:"source_reputation"`
	SourceCategory   string             `json:"source_category"`

	// Classification metadata
	ClassifierVersion    string  `json:"classifier_version"`
	ClassificationMethod string  `json:"classification_method"`
	ModelVersion         string  `json:"model_version,omitempty"`
	Confidence           float64 `json:"confidence"`

	// Crime hybrid classification (optional)
	Crime *CrimeResult `json:"crime,omitempty"`

	// Mining hybrid classification (optional)
	Mining *MiningResult `json:"mining,omitempty"`

	// Coforge hybrid classification (optional)
	Coforge *CoforgeResult `json:"coforge,omitempty"`

	// Entertainment hybrid classification (optional)
	Entertainment *EntertainmentResult `json:"entertainment,omitempty"`

	// Anishinaabe hybrid classification (optional)
	Anishinaabe *AnishinaabeResult `json:"anishinaabe,omitempty"`

	// Location detection (content-based)
	Location *LocationResult `json:"location,omitempty"`

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

// ContentSubtype constants (granularity within article-like content).
// Values correspond to the crawler's DetectedContent* constants,
// passed via meta.detected_content_type in RawContent.
const (
	ContentSubtypePressRelease        = "press_release"
	ContentSubtypeBlogPost            = "blog_post"
	ContentSubtypeEvent               = "event"
	ContentSubtypeAdvisory            = "advisory"
	ContentSubtypeReport              = "report"
	ContentSubtypeBlotter             = "blotter"
	ContentSubtypeCompanyAnnouncement = "company_announcement"
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

// Location specificity constants.
const (
	SpecificityCity     = "city"
	SpecificityProvince = "province"
	SpecificityCountry  = "country"
	SpecificityUnknown  = "unknown"
)

// LocationResult holds the detected location for an article.
type LocationResult struct {
	City        string  `json:"city,omitempty"`
	Province    string  `json:"province,omitempty"`
	Country     string  `json:"country"`
	Specificity string  `json:"specificity"`
	Confidence  float64 `json:"confidence"`
}

// GetSpecificity returns the specificity level based on populated fields.
func (l *LocationResult) GetSpecificity() string {
	if l.City != "" {
		return SpecificityCity
	}
	if l.Province != "" {
		return SpecificityProvince
	}
	if l.Country != "" && l.Country != "unknown" {
		return SpecificityCountry
	}
	return SpecificityUnknown
}
