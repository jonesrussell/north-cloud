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

// AnishinaabeData holds Anishinaabe classification fields from Elasticsearch.
type AnishinaabeData struct {
	Relevance       string   `json:"relevance"`
	Categories      []string `json:"categories"`
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

// Article represents an article from Elasticsearch classified_content index.
type Article struct {
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
	Anishinaabe   *AnishinaabeData   `json:"anishinaabe,omitempty"`
	Entertainment *EntertainmentData `json:"entertainment,omitempty"`
	Coforge       *CoforgeData       `json:"coforge,omitempty"`

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
// flat Article fields used by domain routing functions.
// Call after unmarshaling from Elasticsearch.
func (a *Article) extractNestedFields() {
	if a.Crime != nil {
		a.CrimeRelevance = a.Crime.Relevance
		a.CrimeSubLabel = a.Crime.SubLabel
		a.CrimeTypes = a.Crime.CrimeTypes
		a.LocationSpecificity = a.Crime.Specificity
		a.HomepageEligible = a.Crime.Homepage
		a.CategoryPages = a.Crime.Categories
		a.ReviewRequired = a.Crime.ReviewRequired
	}

	if a.Location != nil {
		a.LocationCity = a.Location.City
		a.LocationProvince = a.Location.Province
		a.LocationCountry = a.Location.Country
		a.LocationConfidence = a.Location.Confidence
		if a.Location.Specificity != "" {
			a.LocationSpecificity = a.Location.Specificity
		}
	}

	if a.Entertainment != nil {
		a.EntertainmentRelevance = a.Entertainment.Relevance
		a.EntertainmentCategories = a.Entertainment.Categories
		a.EntertainmentHomepageEligible = a.Entertainment.HomepageEligible
	}
}
