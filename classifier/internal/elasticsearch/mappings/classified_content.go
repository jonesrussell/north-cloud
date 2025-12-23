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
	Topics         Field `json:"topics"`       // keyword array
	TopicScores    Field `json:"topic_scores"` // object type
	IsCrimeRelated Field `json:"is_crime_related"`

	// Source reputation
	SourceReputation Field `json:"source_reputation"`
	SourceCategory   Field `json:"source_category"`

	// Classification metadata
	ClassifierVersion    Field `json:"classifier_version"`
	ClassificationMethod Field `json:"classification_method"`
	ModelVersion         Field `json:"model_version"`
	Confidence           Field `json:"confidence"`
}

// NewClassifiedContentMapping creates a new classified content mapping with default settings
func NewClassifiedContentMapping() *ClassifiedContentMapping {
	// For raw_html, we want to store but not index it (too large, not searchable)
	indexFalse := false

	return &ClassifiedContentMapping{
		Settings: ClassifiedContentSettings{
			BaseSettings: DefaultSettings(),
		},
		Mappings: ClassifiedContentMappings{
			Properties: ClassifiedContentProperties{
				// ===== Raw Content Fields =====
				ID: Field{
					Type: "keyword",
				},
				URL: Field{
					Type: "keyword",
				},
				SourceName: Field{
					Type: "keyword",
				},
				Title: Field{
					Type:     "text",
					Analyzer: "standard",
				},
				RawHTML: Field{
					Type:  "text",
					Index: &indexFalse, // Store but don't index
				},
				RawText: Field{
					Type:     "text",
					Analyzer: "standard",
				},
				OGType: Field{
					Type: "keyword",
				},
				OGTitle: Field{
					Type:     "text",
					Analyzer: "standard",
				},
				OGDescription: Field{
					Type:     "text",
					Analyzer: "standard",
				},
				OGImage: Field{
					Type: "keyword",
				},
				OGURL: Field{
					Type: "keyword",
				},
				MetaDescription: Field{
					Type:     "text",
					Analyzer: "standard",
				},
				MetaKeywords: Field{
					Type: "keyword",
				},
				CanonicalURL: Field{
					Type: "keyword",
				},
				CrawledAt: Field{
					Type:   "date",
					Format: "strict_date_optional_time||epoch_millis",
				},
				PublishedDate: Field{
					Type:   "date",
					Format: "strict_date_optional_time||epoch_millis",
				},
				ClassificationStatus: Field{
					Type: "keyword",
				},
				ClassifiedAt: Field{
					Type:   "date",
					Format: "strict_date_optional_time||epoch_millis",
				},
				WordCount: Field{
					Type: "integer",
				},

				// ===== Classification Results =====
				ContentType: Field{
					Type: "keyword",
				},
				ContentSubtype: Field{
					Type: "keyword",
				},
				QualityScore: Field{
					Type: "integer",
				},
				QualityFactors: Field{
					Type: "object", // Nested object with dynamic keys
				},
				Topics: Field{
					Type: "keyword", // Array of keywords
				},
				TopicScores: Field{
					Type: "object", // Map of topic -> score
				},
				IsCrimeRelated: Field{
					Type: "boolean",
				},
				SourceReputation: Field{
					Type: "integer",
				},
				SourceCategory: Field{
					Type: "keyword",
				},
				ClassifierVersion: Field{
					Type: "keyword",
				},
				ClassificationMethod: Field{
					Type: "keyword",
				},
				ModelVersion: Field{
					Type: "keyword",
				},
				Confidence: Field{
					Type: "float",
				},
			},
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
