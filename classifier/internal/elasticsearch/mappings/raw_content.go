package mappings

// RawContentMapping represents the Elasticsearch mapping for raw content
type RawContentMapping struct {
	Settings RawContentSettings `json:"settings"`
	Mappings RawContentMappings `json:"mappings"`
}

// RawContentSettings defines index-level settings
type RawContentSettings struct {
	BaseSettings
}

// RawContentMappings defines the field mappings for raw content
type RawContentMappings struct {
	Properties RawContentProperties `json:"properties"`
}

// RawContentProperties defines the properties for each field in the raw content mapping
type RawContentProperties struct {
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
}

// NewRawContentMapping creates a new raw content mapping with default settings
func NewRawContentMapping() *RawContentMapping {
	// For raw_html, we want to store but not index it (too large, not searchable)
	indexFalse := false

	return &RawContentMapping{
		Settings: RawContentSettings{
			BaseSettings: DefaultSettings(),
		},
		Mappings: RawContentMappings{
			Properties: RawContentProperties{
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
					Index: &indexFalse, // Store but don't index (large field)
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
			},
		},
	}
}

// GetJSON returns the raw content mapping as a JSON string
func (m *RawContentMapping) GetJSON() (string, error) {
	return ToJSON(m)
}

// Validate validates the raw content mapping configuration
func (m *RawContentMapping) Validate() error {
	return ValidateSettings(m.Settings.BaseSettings)
}
