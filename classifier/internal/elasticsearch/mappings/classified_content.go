package mappings

// ClassifiedContentMapping represents the Elasticsearch mapping for classified content
type ClassifiedContentMapping struct {
	Settings ClassifiedContentSettings `json:"settings"`
	Mappings ClassifiedContentMappings `json:"mappings"`
}

// ClassifiedContentSettings defines index-level settings
type ClassifiedContentSettings struct {
	BaseSettings
}

// ClassifiedContentMappings defines the field mappings for classified content
type ClassifiedContentMappings struct {
	Properties ClassifiedContentProperties `json:"properties"`
}

// ClassifiedContentProperties defines the properties for each field in the classified content mapping
// This includes all raw content fields PLUS classification results
type ClassifiedContentProperties struct {
	// ===== Raw Content Fields =====
	// Core identifiers
	ID         Field `json:"id"`
	URL        Field `json:"url"`
	SourceName Field `json:"source_name"`

	// Raw content
	Title   Field `json:"title"`
	RawHTML Field `json:"raw_html"`
	RawText Field `json:"raw_text"`

	// Open Graph metadata
	OGType        Field `json:"og_type"`
	OGTitle       Field `json:"og_title"`
	OGDescription Field `json:"og_description"`
	OGImage       Field `json:"og_image"`
	OGURL         Field `json:"og_url"`

	// Basic metadata
	MetaDescription Field `json:"meta_description"`
	MetaKeywords    Field `json:"meta_keywords"`
	CanonicalURL    Field `json:"canonical_url"`

	// Timestamps
	CrawledAt     Field `json:"crawled_at"`
	PublishedDate Field `json:"published_date"`

	// Processing status
	ClassificationStatus Field `json:"classification_status"`
	ClassifiedAt         Field `json:"classified_at"`

	// Quick metrics
	WordCount Field `json:"word_count"`

	// ===== Classification Results =====
	// Content type
	ContentType    Field `json:"content_type"`
	ContentSubtype Field `json:"content_subtype"`

	// Quality scoring
	QualityScore   Field `json:"quality_score"`
	QualityFactors Field `json:"quality_factors"` // object type

	// Topic classification
	Topics      Field `json:"topics"`       // keyword array
	TopicScores Field `json:"topic_scores"` // object type

	// Source reputation
	SourceReputation Field `json:"source_reputation"`
	SourceCategory   Field `json:"source_category"`

	// Classification metadata
	ClassifierVersion    Field `json:"classifier_version"`
	ClassificationMethod Field `json:"classification_method"`
	ModelVersion         Field `json:"model_version"`
	Confidence           Field `json:"confidence"`

	// Crime classification (hybrid rule + ML)
	Crime CrimeProperties `json:"crime,omitempty"`

	// Mining classification (hybrid rule + ML)
	Mining MiningProperties `json:"mining,omitempty"`

	// Entertainment classification (hybrid rule + ML)
	Entertainment EntertainmentProperties `json:"entertainment,omitempty"`

	// Location detection (content-based)
	Location LocationProperties `json:"location,omitempty"`

	// RFP structured extraction
	RFP RFPProperties `json:"rfp,omitempty"`
}

// EntertainmentProperties defines the nested properties for entertainment classification.
type EntertainmentProperties struct {
	Type       string                       `json:"type,omitempty"`
	Properties EntertainmentFieldProperties `json:"properties,omitempty"`
}

// EntertainmentFieldProperties defines individual fields within entertainment classification.
type EntertainmentFieldProperties struct {
	Relevance        Field `json:"relevance"`
	Categories       Field `json:"categories"`
	FinalConfidence  Field `json:"final_confidence"`
	HomepageEligible Field `json:"homepage_eligible"`
	ReviewRequired   Field `json:"review_required"`
	ModelVersion     Field `json:"model_version"`
}

// CrimeProperties defines the nested properties for crime classification.
type CrimeProperties struct {
	Type       string               `json:"type,omitempty"`
	Properties CrimeFieldProperties `json:"properties,omitempty"`
}

// CrimeFieldProperties defines individual fields within crime classification.
type CrimeFieldProperties struct {
	Relevance           Field `json:"street_crime_relevance"`
	SubLabel            Field `json:"sub_label"`
	CrimeTypes          Field `json:"crime_types"`
	LocationSpecificity Field `json:"location_specificity"`
	FinalConfidence     Field `json:"final_confidence"`
	HomepageEligible    Field `json:"homepage_eligible"`
	CategoryPages       Field `json:"category_pages"`
	ReviewRequired      Field `json:"review_required"`
}

// MiningProperties defines the nested properties for mining classification.
type MiningProperties struct {
	Type       string                `json:"type,omitempty"`
	Properties MiningFieldProperties `json:"properties,omitempty"`
}

// MiningFieldProperties defines individual fields within mining classification.
type MiningFieldProperties struct {
	Relevance       Field `json:"relevance"`
	MiningStage     Field `json:"mining_stage"`
	Commodities     Field `json:"commodities"`
	Location        Field `json:"location"`
	FinalConfidence Field `json:"final_confidence"`
	ReviewRequired  Field `json:"review_required"`
	ModelVersion    Field `json:"model_version"`
}

// LocationProperties defines the nested properties for location detection.
type LocationProperties struct {
	Type       string                  `json:"type,omitempty"`
	Properties LocationFieldProperties `json:"properties,omitempty"`
}

// LocationFieldProperties defines individual fields within location detection.
type LocationFieldProperties struct {
	City        Field `json:"city"`
	Province    Field `json:"province"`
	Country     Field `json:"country"`
	Specificity Field `json:"specificity"`
	Confidence  Field `json:"confidence"`
}

// RFPProperties defines the nested properties for RFP extraction.
type RFPProperties struct {
	Type       string             `json:"type,omitempty"`
	Properties RFPFieldProperties `json:"properties,omitempty"`
}

// RFPFieldProperties defines individual fields within RFP extraction.
type RFPFieldProperties struct {
	ExtractionMethod Field `json:"extraction_method"`
	Title            Field `json:"title"`
	ReferenceNumber  Field `json:"reference_number"`
	OrganizationName Field `json:"organization_name"`
	Description      Field `json:"description"`
	PublishedDate    Field `json:"published_date"`
	ClosingDate      Field `json:"closing_date"`
	AmendmentDate    Field `json:"amendment_date"`
	BudgetMin        Field `json:"budget_min"`
	BudgetMax        Field `json:"budget_max"`
	BudgetCurrency   Field `json:"budget_currency"`
	ProcurementType  Field `json:"procurement_type"`
	NAICSCodes       Field `json:"naics_codes"`
	Categories       Field `json:"categories"`
	Province         Field `json:"province"`
	City             Field `json:"city"`
	Country          Field `json:"country"`
	Eligibility      Field `json:"eligibility"`
	SourceURL        Field `json:"source_url"`
	ContactName      Field `json:"contact_name"`
	ContactEmail     Field `json:"contact_email"`
}

// createRawContentProperties creates properties for raw content fields
func createRawContentProperties() ClassifiedContentProperties {
	indexFalse := false
	dateFormat := "strict_date_optional_time||epoch_millis"

	return ClassifiedContentProperties{
		// ===== Raw Content Fields =====
		ID:                   Field{Type: "keyword"},
		URL:                  Field{Type: "keyword"},
		SourceName:           Field{Type: "keyword"},
		Title:                Field{Type: "text", Analyzer: "standard"},
		RawHTML:              Field{Type: "text", Index: &indexFalse}, // Store but don't index
		RawText:              Field{Type: "text", Analyzer: "standard"},
		OGType:               Field{Type: "keyword"},
		OGTitle:              Field{Type: "text", Analyzer: "standard"},
		OGDescription:        Field{Type: "text", Analyzer: "standard"},
		OGImage:              Field{Type: "keyword"},
		OGURL:                Field{Type: "keyword"},
		MetaDescription:      Field{Type: "text", Analyzer: "standard"},
		MetaKeywords:         Field{Type: "keyword"},
		CanonicalURL:         Field{Type: "keyword"},
		CrawledAt:            Field{Type: "date", Format: dateFormat},
		PublishedDate:        Field{Type: "date", Format: dateFormat},
		ClassificationStatus: Field{Type: "keyword"},
		ClassifiedAt:         Field{Type: "date", Format: dateFormat},
		WordCount:            Field{Type: "integer"},
	}
}

// createClassificationProperties creates properties for classification result fields
func createClassificationProperties() ClassifiedContentProperties {
	return ClassifiedContentProperties{
		// ===== Classification Results =====
		ContentType:          Field{Type: "keyword"},
		ContentSubtype:       Field{Type: "keyword"},
		QualityScore:         Field{Type: "integer"},
		QualityFactors:       Field{Type: "object"},  // Nested object with dynamic keys
		Topics:               Field{Type: "keyword"}, // Array of keywords
		TopicScores:          Field{Type: "object"},  // Map of topic -> score
		SourceReputation:     Field{Type: "integer"},
		SourceCategory:       Field{Type: "keyword"},
		ClassifierVersion:    Field{Type: "keyword"},
		ClassificationMethod: Field{Type: "keyword"},
		ModelVersion:         Field{Type: "keyword"},
		Confidence:           Field{Type: "float"},
		Crime:                createCrimeProperties(),
		Mining:               createMiningProperties(),
		Entertainment:        createEntertainmentProperties(),
		Location:             createLocationProperties(),
		RFP:                  createRFPProperties(),
	}
}

// createEntertainmentProperties creates nested properties for entertainment classification.
func createEntertainmentProperties() EntertainmentProperties {
	return EntertainmentProperties{
		Type: "object",
		Properties: EntertainmentFieldProperties{
			Relevance:        Field{Type: "keyword"},
			Categories:       Field{Type: "keyword"},
			FinalConfidence:  Field{Type: "float"},
			HomepageEligible: Field{Type: "boolean"},
			ReviewRequired:   Field{Type: "boolean"},
			ModelVersion:     Field{Type: "keyword"},
		},
	}
}

// createCrimeProperties creates nested properties for crime classification.
func createCrimeProperties() CrimeProperties {
	return CrimeProperties{
		Type: "object",
		Properties: CrimeFieldProperties{
			Relevance:           Field{Type: "keyword"},
			SubLabel:            Field{Type: "keyword"},
			CrimeTypes:          Field{Type: "keyword"},
			LocationSpecificity: Field{Type: "keyword"},
			FinalConfidence:     Field{Type: "float"},
			HomepageEligible:    Field{Type: "boolean"},
			CategoryPages:       Field{Type: "keyword"},
			ReviewRequired:      Field{Type: "boolean"},
		},
	}
}

// createLocationProperties creates nested properties for location detection.
func createLocationProperties() LocationProperties {
	return LocationProperties{
		Type: "object",
		Properties: LocationFieldProperties{
			City:        Field{Type: "keyword"},
			Province:    Field{Type: "keyword"},
			Country:     Field{Type: "keyword"},
			Specificity: Field{Type: "keyword"},
			Confidence:  Field{Type: "float"},
		},
	}
}

// createMiningProperties creates nested properties for mining classification.
func createMiningProperties() MiningProperties {
	return MiningProperties{
		Type: "object",
		Properties: MiningFieldProperties{
			Relevance:       Field{Type: "keyword"},
			MiningStage:     Field{Type: "keyword"},
			Commodities:     Field{Type: "keyword"},
			Location:        Field{Type: "keyword"},
			FinalConfidence: Field{Type: "float"},
			ReviewRequired:  Field{Type: "boolean"},
			ModelVersion:    Field{Type: "keyword"},
		},
	}
}

// createRFPProperties creates nested properties for RFP extraction.
// Date fields use keyword (not date) because heuristic extraction produces raw strings
// that may not conform to strict_date_optional_time. When Schema.org extraction is added,
// consider upgrading to date type with a normalization step.
func createRFPProperties() RFPProperties {
	return RFPProperties{
		Type: "object",
		Properties: RFPFieldProperties{
			ExtractionMethod: Field{Type: "keyword"},
			Title:            Field{Type: "text", Analyzer: "standard"},
			ReferenceNumber:  Field{Type: "keyword"},
			OrganizationName: Field{Type: "keyword"},
			Description:      Field{Type: "text", Analyzer: "standard"},
			PublishedDate:    Field{Type: "keyword"},
			ClosingDate:      Field{Type: "keyword"},
			AmendmentDate:    Field{Type: "keyword"},
			BudgetMin:        Field{Type: "float"},
			BudgetMax:        Field{Type: "float"},
			BudgetCurrency:   Field{Type: "keyword"},
			ProcurementType:  Field{Type: "keyword"},
			NAICSCodes:       Field{Type: "keyword"},
			Categories:       Field{Type: "keyword"},
			Province:         Field{Type: "keyword"},
			City:             Field{Type: "keyword"},
			Country:          Field{Type: "keyword"},
			Eligibility:      Field{Type: "text", Analyzer: "standard"},
			SourceURL:        Field{Type: "keyword"},
			ContactName:      Field{Type: "keyword"},
			ContactEmail:     Field{Type: "keyword"},
		},
	}
}

// mergeProperties merges two ClassifiedContentProperties structs
func mergeProperties(raw, classified ClassifiedContentProperties) ClassifiedContentProperties {
	return ClassifiedContentProperties{
		// Raw content fields
		ID: raw.ID, URL: raw.URL, SourceName: raw.SourceName,
		Title: raw.Title, RawHTML: raw.RawHTML, RawText: raw.RawText,
		OGType: raw.OGType, OGTitle: raw.OGTitle, OGDescription: raw.OGDescription,
		OGImage: raw.OGImage, OGURL: raw.OGURL,
		MetaDescription: raw.MetaDescription, MetaKeywords: raw.MetaKeywords,
		CanonicalURL: raw.CanonicalURL,
		CrawledAt:    raw.CrawledAt, PublishedDate: raw.PublishedDate,
		ClassificationStatus: raw.ClassificationStatus, ClassifiedAt: raw.ClassifiedAt,
		WordCount: raw.WordCount,
		// Classification fields
		ContentType: classified.ContentType, ContentSubtype: classified.ContentSubtype,
		QualityScore: classified.QualityScore, QualityFactors: classified.QualityFactors,
		Topics: classified.Topics, TopicScores: classified.TopicScores,
		SourceReputation: classified.SourceReputation, SourceCategory: classified.SourceCategory,
		ClassifierVersion:    classified.ClassifierVersion,
		ClassificationMethod: classified.ClassificationMethod,
		ModelVersion:         classified.ModelVersion, Confidence: classified.Confidence,
		Crime:         classified.Crime,
		Mining:        classified.Mining,
		Entertainment: classified.Entertainment,
		Location:      classified.Location,
		RFP:           classified.RFP,
	}
}

// NewClassifiedContentMapping creates a new classified content mapping with default settings
func NewClassifiedContentMapping() *ClassifiedContentMapping {
	rawProps := createRawContentProperties()
	classifiedProps := createClassificationProperties()
	properties := mergeProperties(rawProps, classifiedProps)

	return &ClassifiedContentMapping{
		Settings: ClassifiedContentSettings{
			BaseSettings: DefaultSettings(),
		},
		Mappings: ClassifiedContentMappings{
			Properties: properties,
		},
	}
}

// GetJSON returns the classified content mapping as a JSON string
func (m *ClassifiedContentMapping) GetJSON() (string, error) {
	return ToJSON(m)
}

// Validate validates the classified content mapping configuration
func (m *ClassifiedContentMapping) Validate() error {
	return ValidateSettings(m.Settings.BaseSettings)
}
