package domain

// RFPDocument represents a classified RFP document for Elasticsearch indexing.
// Field names mirror the classifier's classified_content schema so the search
// service multi-match query (title, raw_text, body, …) picks up these docs.
type RFPDocument struct {
	Title        string   `json:"title"`
	URL          string   `json:"url"`
	SourceName   string   `json:"source_name"`
	ContentType  string   `json:"content_type"`
	QualityScore int      `json:"quality_score"`
	Snippet      string   `json:"snippet"`
	RawText      string   `json:"raw_text"`
	Topics       []string `json:"topics"`
	CrawledAt    string   `json:"crawled_at"`
	RFP          RFP      `json:"rfp"`
}

// RFP holds the structured fields extracted from a procurement notice.
type RFP struct {
	ExtractionMethod string   `json:"extraction_method"`
	Title            string   `json:"title,omitempty"`
	ReferenceNumber  string   `json:"reference_number"`
	OrganizationName string   `json:"organization_name,omitempty"`
	Description      string   `json:"description,omitempty"`
	PublishedDate    string   `json:"published_date,omitempty"`
	ClosingDate      string   `json:"closing_date,omitempty"`
	AmendmentDate    string   `json:"amendment_date,omitempty"`
	AmendmentNumber  string   `json:"amendment_number,omitempty"`
	BudgetCurrency   string   `json:"budget_currency,omitempty"`
	ProcurementType  string   `json:"procurement_type,omitempty"`
	Categories       []string `json:"categories,omitempty"`
	Province         string   `json:"province,omitempty"`
	City             string   `json:"city,omitempty"`
	Country          string   `json:"country,omitempty"`
	SourceURL        string   `json:"source_url,omitempty"`
	ContactName      string   `json:"contact_name,omitempty"`
	ContactEmail     string   `json:"contact_email,omitempty"`
	GSIN             string   `json:"gsin,omitempty"`
	UNSPSC           string   `json:"unspsc,omitempty"`
	TenderStatus     string   `json:"tender_status,omitempty"`
}
