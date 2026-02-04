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
}

// CrimeProperties defines the nested properties for crime classification.
type CrimeProperties struct {
	Type       string               `json:"type,omitempty"`
	Properties CrimeFieldProperties `json:"properties,omitempty"`
}

// CrimeFieldProperties defines individual fields within crime classification.
type CrimeFieldProperties struct {
	Relevance           Field `json:"street_crime_relevance"`
	CrimeTypes          Field `json:"crime_types"`
	LocationSpecificity Field `json:"location_specificity"`
	FinalConfidence     Field `json:"final_confidence"`
	HomepageEligible    Field `json:"homepage_eligible"`
	CategoryPages       Field `json:"category_pages"`
	ReviewRequired      Field `json:"review_required"`
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
	}
}

// createCrimeProperties creates nested properties for crime classification.
func createCrimeProperties() CrimeProperties {
	return CrimeProperties{
		Type: "object",
		Properties: CrimeFieldProperties{
			Relevance:           Field{Type: "keyword"},
			CrimeTypes:          Field{Type: "keyword"},
			LocationSpecificity: Field{Type: "keyword"},
			FinalConfidence:     Field{Type: "float"},
			HomepageEligible:    Field{Type: "boolean"},
			CategoryPages:       Field{Type: "keyword"},
			ReviewRequired:      Field{Type: "boolean"},
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
		Crime: classified.Crime,
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
