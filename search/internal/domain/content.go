package domain

import "time"

// SearchCrimeInfo contains crime classification data from the crime nested object
type SearchCrimeInfo struct {
	Relevance string `json:"relevance,omitempty"`
}

// ClassifiedContent represents a document from Elasticsearch classified_content indexes
type ClassifiedContent struct {
	ID               string           `json:"id"`
	URL              string           `json:"url"`
	SourceName       string           `json:"source_name"`
	Title            string           `json:"title"`
	RawText          string           `json:"raw_text"`
	RawHTML          string           `json:"raw_html,omitempty"`
	OGTitle          string           `json:"og_title,omitempty"`
	OGDescription    string           `json:"og_description,omitempty"`
	MetaDescription  string           `json:"meta_description,omitempty"`
	CrawledAt        *time.Time       `json:"crawled_at,omitempty"`
	PublishedDate    *time.Time       `json:"published_date,omitempty"`
	ContentType      string           `json:"content_type"`
	QualityScore     int              `json:"quality_score"`
	Topics           []string         `json:"topics,omitempty"`
	Crime            *SearchCrimeInfo `json:"crime,omitempty"`
	SourceReputation int              `json:"source_reputation,omitempty"`
	Confidence       float64          `json:"confidence,omitempty"`
	WordCount        int              `json:"word_count,omitempty"`

	// Alias fields for compatibility
	Body   string `json:"body,omitempty"`   // Alias for raw_text
	Source string `json:"source,omitempty"` // Alias for url
}

// GetCrimeRelevance returns the crime relevance string from the nested crime object
func (c *ClassifiedContent) GetCrimeRelevance() string {
	if c.Crime != nil {
		return c.Crime.Relevance
	}
	return ""
}

// ToSearchHit converts ClassifiedContent to SearchHit
func (c *ClassifiedContent) ToSearchHit(score float64, highlight map[string][]string) *SearchHit {
	// Generate snippet from raw_text if no highlight available
	snippet := ""
	if len(highlight) == 0 && len(c.RawText) > 150 {
		snippet = c.RawText[:150] + "..."
	}

	return &SearchHit{
		ID:             c.ID,
		Title:          c.Title,
		URL:            c.URL,
		SourceName:     c.SourceName,
		PublishedDate:  c.PublishedDate,
		CrawledAt:      c.CrawledAt,
		QualityScore:   c.QualityScore,
		ContentType:    c.ContentType,
		Topics:         c.Topics,
		CrimeRelevance: c.GetCrimeRelevance(),
		Score:          score,
		Highlight:      highlight,
		Snippet:        snippet,
	}
}
