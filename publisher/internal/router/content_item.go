package router

import "time"

// CoforgeData holds Coforge classification fields from Elasticsearch.
type CoforgeData struct {
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

// MiningData holds mining classification fields from Elasticsearch.
type MiningData struct {
	Relevance       string   `json:"relevance"`
	MiningStage     string   `json:"mining_stage"`
	Commodities     []string `json:"commodities"`
	Location        string   `json:"location"`
	FinalConfidence float64  `json:"final_confidence"`
	ReviewRequired  bool     `json:"review_required"`
	ModelVersion    string   `json:"model_version,omitempty"`
}

// CrimeData matches the classifier's nested crime object in Elasticsearch.
type CrimeData struct {
	Relevance      string   `json:"street_crime_relevance"`
	SubLabel       string   `json:"sub_label,omitempty"`
	CrimeTypes     []string `json:"crime_types"`
	Specificity    string   `json:"location_specificity"`
	Confidence     float64  `json:"final_confidence"`
	Homepage       bool     `json:"homepage_eligible"`
	Categories     []string `json:"category_pages"`
	ReviewRequired bool     `json:"review_required"`
}

// LocationData matches the classifier's nested location object in Elasticsearch.
type LocationData struct {
	City        string  `json:"city,omitempty"`
	Province    string  `json:"province,omitempty"`
	Country     string  `json:"country"`
	Specificity string  `json:"specificity"`
	Confidence  float64 `json:"confidence"`
}

// IndigenousData holds Indigenous classification fields from Elasticsearch.
type IndigenousData struct {
	Relevance       string   `json:"relevance"`
	Categories      []string `json:"categories"`
	Region          string   `json:"region,omitempty"`
	FinalConfidence float64  `json:"final_confidence"`
	ReviewRequired  bool     `json:"review_required"`
	ModelVersion    string   `json:"model_version,omitempty"`
}

// EntertainmentData holds entertainment classification fields from Elasticsearch.
type EntertainmentData struct {
	Relevance        string   `json:"relevance"`
	Categories       []string `json:"categories"`
	FinalConfidence  float64  `json:"final_confidence"`
	HomepageEligible bool     `json:"homepage_eligible"`
	ReviewRequired   bool     `json:"review_required"`
	ModelVersion     string   `json:"model_version,omitempty"`
}

// RecipeData holds the publisher view of structured recipe extraction from Elasticsearch.
// It is a subset of the classifier's RecipeResult; ES may index additional fields that are
// ignored on unmarshal.
type RecipeData struct {
	ExtractionMethod string   `json:"extraction_method"`
	Name             string   `json:"name,omitempty"`
	Ingredients      []string `json:"ingredients,omitempty"`
	Category         string   `json:"category,omitempty"`
	Cuisine          string   `json:"cuisine,omitempty"`
}

// JobData holds the publisher view of structured job extraction from Elasticsearch.
// It is a subset of the classifier's JobResult; ES may index additional fields that are
// ignored on unmarshal.
type JobData struct {
	ExtractionMethod string `json:"extraction_method"`
	Title            string `json:"title,omitempty"`
	Company          string `json:"company,omitempty"`
	Location         string `json:"location,omitempty"`
	EmploymentType   string `json:"employment_type,omitempty"`
	Industry         string `json:"industry,omitempty"`
}

// RFPData holds the publisher view of structured RFP extraction from Elasticsearch.
// It is a subset of the classifier's RFPResult; ES may index additional fields that are
// ignored on unmarshal.
type RFPData struct {
	ExtractionMethod string   `json:"extraction_method"`
	Title            string   `json:"title,omitempty"`
	ReferenceNumber  string   `json:"reference_number,omitempty"`
	OrganizationName string   `json:"organization_name,omitempty"`
	ClosingDate      string   `json:"closing_date,omitempty"`
	BudgetMin        *float64 `json:"budget_min,omitempty"`
	BudgetMax        *float64 `json:"budget_max,omitempty"`
	BudgetCurrency   string   `json:"budget_currency,omitempty"`
	ProcurementType  string   `json:"procurement_type,omitempty"`
	Categories       []string `json:"categories,omitempty"`
	Province         string   `json:"province,omitempty"`
	City             string   `json:"city,omitempty"`
	Country          string   `json:"country,omitempty"`
}

// ContentItem represents a content item from Elasticsearch classified_content index.
type ContentItem struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Body          string    `json:"body"`
	RawText       string    `json:"raw_text"`
	RawHTML       string    `json:"raw_html"`
	URL           string    `json:"canonical_url"`
	Source        string    `json:"source"`
	PublishedDate time.Time `json:"published_date"`

	// Classification metadata
	QualityScore     int      `json:"quality_score"`
	Topics           []string `json:"topics"`
	ContentType      string   `json:"content_type"`
	ContentSubtype   string   `json:"content_subtype,omitempty"`
	SourceReputation int      `json:"source_reputation"`
	Confidence       float64  `json:"confidence"`

	// Crime classification (hybrid rule + ML) — flat fields
	CrimeRelevance      string   `json:"crime_relevance"`
	CrimeSubLabel       string   `json:"crime_sub_label,omitempty"`
	CrimeTypes          []string `json:"crime_types"`
	LocationSpecificity string   `json:"location_specificity"`
	HomepageEligible    bool     `json:"homepage_eligible"`
	CategoryPages       []string `json:"category_pages"`
	ReviewRequired      bool     `json:"review_required"`

	// Location detection (content-based) — flat fields
	LocationCity       string  `json:"location_city,omitempty"`
	LocationProvince   string  `json:"location_province,omitempty"`
	LocationCountry    string  `json:"location_country"`
	LocationConfidence float64 `json:"location_confidence"`

	// Nested classifier objects from Elasticsearch
	Crime         *CrimeData         `json:"crime,omitempty"`
	Location      *LocationData      `json:"location,omitempty"`
	Mining        *MiningData        `json:"mining,omitempty"`
	Indigenous    *IndigenousData    `json:"indigenous,omitempty"`
	Entertainment *EntertainmentData `json:"entertainment,omitempty"`
	Coforge       *CoforgeData       `json:"coforge,omitempty"`

	// Recipe, Job, and RFP structured extraction
	Recipe *RecipeData `json:"recipe,omitempty"`
	Job    *JobData    `json:"job,omitempty"`
	RFP    *RFPData    `json:"rfp,omitempty"`

	// Entertainment flat fields (populated from nested Entertainment object)
	EntertainmentRelevance        string   `json:"entertainment_relevance"`
	EntertainmentCategories       []string `json:"entertainment_categories"`
	EntertainmentHomepageEligible bool     `json:"entertainment_homepage_eligible"`

	// Open Graph metadata
	OGTitle       string `json:"og_title"`
	OGDescription string `json:"og_description"`
	OGImage       string `json:"og_image"`
	OGURL         string `json:"og_url"`

	// Additional fields
	WordCount int `json:"word_count"`

	// Sort values for search_after pagination
	Sort []any `json:"-"`
}

// extractNestedFields copies values from nested Elasticsearch objects into the
// flat ContentItem fields used by domain routing functions.
// Call after unmarshaling from Elasticsearch.
func (c *ContentItem) extractNestedFields() {
	if c.Crime != nil {
		c.CrimeRelevance = c.Crime.Relevance
		c.CrimeSubLabel = c.Crime.SubLabel
		c.CrimeTypes = c.Crime.CrimeTypes
		c.LocationSpecificity = c.Crime.Specificity
		c.HomepageEligible = c.Crime.Homepage
		c.CategoryPages = c.Crime.Categories
		c.ReviewRequired = c.Crime.ReviewRequired
	}

	if c.Location != nil {
		c.LocationCity = c.Location.City
		c.LocationProvince = c.Location.Province
		c.LocationCountry = c.Location.Country
		c.LocationConfidence = c.Location.Confidence
		if c.Location.Specificity != "" {
			c.LocationSpecificity = c.Location.Specificity
		}
	}

	if c.Entertainment != nil {
		c.EntertainmentRelevance = c.Entertainment.Relevance
		c.EntertainmentCategories = c.Entertainment.Categories
		c.EntertainmentHomepageEligible = c.Entertainment.HomepageEligible
	}
}
