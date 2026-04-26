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

	// Indigenous hybrid classification (optional)
	Indigenous *IndigenousResult `json:"indigenous,omitempty"`

	// Location detection (content-based)
	Location *LocationResult `json:"location,omitempty"`

	// Recipe structured extraction (optional)
	Recipe *RecipeResult `json:"recipe,omitempty"`

	// Job structured extraction (optional)
	Job *JobResult `json:"job,omitempty"`

	// RFP structured extraction (optional)
	RFP *RFPResult `json:"rfp,omitempty"`

	// Need signal detection (optional)
	NeedSignal *NeedSignalResult `json:"need_signal,omitempty"`

	// ICP segment alignment (optional)
	ICP *ICPResult `json:"icp,omitempty"`
}

// IndigenousResult holds Indigenous hybrid classification results.
type IndigenousResult struct {
	Relevance       string   `json:"relevance"`
	Categories      []string `json:"categories"`
	Region          string   `json:"region,omitempty"`
	FinalConfidence float64  `json:"final_confidence"`
	ReviewRequired  bool     `json:"review_required"`
	ModelVersion    string   `json:"model_version,omitempty"`

	// Decision context (observability)
	DecisionPath     string  `json:"decision_path,omitempty"`
	MLConfidenceRaw  float64 `json:"ml_confidence_raw,omitempty"`
	RuleTriggered    string  `json:"rule_triggered,omitempty"`
	ProcessingTimeMs int64   `json:"processing_time_ms,omitempty"`
}

// EntertainmentResult holds Entertainment hybrid classification results.
type EntertainmentResult struct {
	Relevance        string   `json:"relevance"`
	Categories       []string `json:"categories"`
	FinalConfidence  float64  `json:"final_confidence"`
	HomepageEligible bool     `json:"homepage_eligible"`
	ReviewRequired   bool     `json:"review_required"`
	ModelVersion     string   `json:"model_version,omitempty"`

	// Decision context (observability)
	DecisionPath     string  `json:"decision_path,omitempty"`
	MLConfidenceRaw  float64 `json:"ml_confidence_raw,omitempty"`
	RuleTriggered    string  `json:"rule_triggered,omitempty"`
	ProcessingTimeMs int64   `json:"processing_time_ms,omitempty"`
}

// DrillResult holds a single extracted drill result from a mining article.
type DrillResult struct {
	HoleID     string  `json:"hole_id"`
	Commodity  string  `json:"commodity"`
	InterceptM float64 `json:"intercept_m"`
	Grade      float64 `json:"grade"`
	Unit       string  `json:"unit"`
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

	// Decision context (observability)
	DecisionPath     string  `json:"decision_path,omitempty"`
	MLConfidenceRaw  float64 `json:"ml_confidence_raw,omitempty"`
	RuleTriggered    string  `json:"rule_triggered,omitempty"`
	ProcessingTimeMs int64   `json:"processing_time_ms,omitempty"`

	// Drill results extraction
	DrillResults     []DrillResult `json:"drill_results,omitempty"`
	ExtractionMethod string        `json:"extraction_method,omitempty"` // "regex", "llm", "hybrid", ""
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

	// Decision context (observability)
	DecisionPath     string  `json:"decision_path,omitempty"`
	MLConfidenceRaw  float64 `json:"ml_confidence_raw,omitempty"`
	RuleTriggered    string  `json:"rule_triggered,omitempty"`
	ProcessingTimeMs int64   `json:"processing_time_ms,omitempty"`
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

	// Decision context (observability)
	DecisionPath     string  `json:"decision_path,omitempty"`
	MLConfidenceRaw  float64 `json:"ml_confidence_raw,omitempty"`
	RuleTriggered    string  `json:"rule_triggered,omitempty"`
	ProcessingTimeMs int64   `json:"processing_time_ms,omitempty"`
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

	// Quality gate flag — true when article indexed despite low quality_score
	LowQuality bool `json:"low_quality,omitempty"`

	// Crime hybrid classification (optional)
	Crime *CrimeResult `json:"crime,omitempty"`

	// Mining hybrid classification (optional)
	Mining *MiningResult `json:"mining,omitempty"`

	// Coforge hybrid classification (optional)
	Coforge *CoforgeResult `json:"coforge,omitempty"`

	// Entertainment hybrid classification (optional)
	Entertainment *EntertainmentResult `json:"entertainment,omitempty"`

	// Indigenous hybrid classification (optional)
	Indigenous *IndigenousResult `json:"indigenous,omitempty"`

	// Location detection (content-based)
	Location *LocationResult `json:"location,omitempty"`

	// Recipe structured extraction (optional)
	Recipe *RecipeResult `json:"recipe,omitempty"`

	// Job structured extraction (optional)
	Job *JobResult `json:"job,omitempty"`

	// RFP structured extraction (optional)
	RFP *RFPResult `json:"rfp,omitempty"`

	// Need signal detection (optional)
	NeedSignal *NeedSignalResult `json:"need_signal,omitempty"`

	// ICP segment alignment (optional)
	ICP *ICPResult `json:"icp,omitempty"`

	// Publisher compatibility aliases
	// These duplicate RawContent fields for backward compatibility with publisher
	Body   string `json:"body"`   // Alias for RawText (publisher expects "body")
	Source string `json:"source"` // Alias for URL (publisher expects "source")
}

// ContentType constants
const (
	ContentTypeArticle    = "article"
	ContentTypePage       = "page"
	ContentTypeVideo      = "video"
	ContentTypeImage      = "image"
	ContentTypeJob        = "job"
	ContentTypeRecipe     = "recipe"
	ContentTypeEvent      = "event"
	ContentTypeObituary   = "obituary"
	ContentTypeRFP        = "rfp"
	ContentTypeNeedSignal = "need_signal"
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
	ContentSubtypeEventReport         = "event_report"
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

// RecipeResult holds structured recipe extraction results.
// Non-nil RecipeResult values always have ExtractionMethod set by the extractors ("schema_org" or "heuristic").
type RecipeResult struct {
	ExtractionMethod string   `json:"extraction_method"` // "schema_org" or "heuristic"
	Name             string   `json:"name,omitempty"`
	Ingredients      []string `json:"ingredients,omitempty"`
	Instructions     string   `json:"instructions,omitempty"`
	PrepTimeMinutes  *int     `json:"prep_time_minutes,omitempty"`
	CookTimeMinutes  *int     `json:"cook_time_minutes,omitempty"`
	TotalTimeMinutes *int     `json:"total_time_minutes,omitempty"`
	Servings         string   `json:"servings,omitempty"`
	Category         string   `json:"category,omitempty"`
	Cuisine          string   `json:"cuisine,omitempty"`
	Calories         string   `json:"calories,omitempty"`
	ImageURL         string   `json:"image_url,omitempty"`
	Rating           *float64 `json:"rating,omitempty"`
	RatingCount      *int     `json:"rating_count,omitempty"`
}

// JobResult holds structured job posting extraction results.
// Non-nil JobResult values always have ExtractionMethod set by the extractors ("schema_org" or "heuristic").
type JobResult struct {
	ExtractionMethod string   `json:"extraction_method"` // "schema_org" or "heuristic"
	Title            string   `json:"title,omitempty"`
	Company          string   `json:"company,omitempty"`
	Location         string   `json:"location,omitempty"`
	SalaryMin        *float64 `json:"salary_min,omitempty"`
	SalaryMax        *float64 `json:"salary_max,omitempty"`
	SalaryCurrency   string   `json:"salary_currency,omitempty"`
	EmploymentType   string   `json:"employment_type,omitempty"` // full_time, part_time, contract, temporary, internship
	PostedDate       string   `json:"posted_date,omitempty"`
	ExpiresDate      string   `json:"expires_date,omitempty"`
	Description      string   `json:"description,omitempty"`
	Industry         string   `json:"industry,omitempty"`
	Qualifications   string   `json:"qualifications,omitempty"`
	Benefits         string   `json:"benefits,omitempty"`
}

// RFPResult holds structured RFP/procurement extraction results.
// Non-nil values always have ExtractionMethod set ("heuristic").
// TODO: add "schema_org" extraction when structured procurement data becomes common.
type RFPResult struct {
	ExtractionMethod string `json:"extraction_method"`
	// DocumentType classifies the procurement document kind.
	// Values: "" (normal solicitation/bid), "notice" (Notice to Industry, Proactive Disclosure),
	// "rfi" (Request for Information — for info only, no bid expected).
	DocumentType     string   `json:"document_type,omitempty"`
	Title            string   `json:"title,omitempty"`
	ReferenceNumber  string   `json:"reference_number,omitempty"`
	OrganizationName string   `json:"organization_name,omitempty"`
	Description      string   `json:"description,omitempty"`
	PublishedDate    string   `json:"published_date,omitempty"`
	ClosingDate      string   `json:"closing_date,omitempty"`
	AmendmentDate    string   `json:"amendment_date,omitempty"`
	BudgetMin        *float64 `json:"budget_min,omitempty"`
	BudgetMax        *float64 `json:"budget_max,omitempty"`
	BudgetCurrency   string   `json:"budget_currency,omitempty"`
	ProcurementType  string   `json:"procurement_type,omitempty"`
	NAICSCodes       []string `json:"naics_codes,omitempty"`
	Categories       []string `json:"categories,omitempty"`
	Province         string   `json:"province,omitempty"`
	City             string   `json:"city,omitempty"`
	Country          string   `json:"country,omitempty"`
	Eligibility      string   `json:"eligibility,omitempty"`
	SourceURL        string   `json:"source_url,omitempty"`
	ContactName      string   `json:"contact_name,omitempty"`
	ContactEmail     string   `json:"contact_email,omitempty"`
}

// NeedSignalResult holds detection results for proactive outreach signals.
// Non-nil values indicate the content suggests an organization may need web services.
type NeedSignalResult struct {
	SignalType                 string   `json:"signal_type"`
	OrganizationName           string   `json:"organization_name,omitempty"`
	OrganizationNameNormalized string   `json:"organization_name_normalized,omitempty"`
	Sector                     string   `json:"sector,omitempty"`
	Province                   string   `json:"province,omitempty"`
	City                       string   `json:"city,omitempty"`
	ContactEmail               string   `json:"contact_email,omitempty"`
	ContactName                string   `json:"contact_name,omitempty"`
	SourceURL                  string   `json:"source_url,omitempty"`
	Keywords                   []string `json:"keywords,omitempty"`
	Confidence                 float64  `json:"confidence"`
}

type ICPResult struct {
	Segments     []ICPSegmentResult `json:"segments"`
	ModelVersion string             `json:"model_version"`
}

type ICPSegmentResult struct {
	Segment         string   `json:"segment"`
	Score           float64  `json:"score"`
	MatchedKeywords []string `json:"matched_keywords"`
}
