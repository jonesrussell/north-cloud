package domain

// CrimeAggregation represents crime distribution statistics
type CrimeAggregation struct {
	BySubLabel        map[string]int64 `json:"by_sub_label"`
	ByRelevance       map[string]int64 `json:"by_relevance"`
	ByCrimeType       map[string]int64 `json:"by_crime_type"`
	TotalCrimeRelated int64            `json:"total_crime_related"`
	TotalDocuments    int64            `json:"total_documents"`
}

// MiningAggregation represents mining distribution statistics
type MiningAggregation struct {
	ByRelevance    map[string]int64 `json:"by_relevance"`
	ByMiningStage  map[string]int64 `json:"by_mining_stage"`
	ByCommodity    map[string]int64 `json:"by_commodity"`
	ByLocation     map[string]int64 `json:"by_location"`
	TotalMining    int64            `json:"total_mining"`
	TotalDocuments int64            `json:"total_documents"`
}

// LocationAggregation represents geographic distribution statistics
type LocationAggregation struct {
	ByCountry     map[string]int64 `json:"by_country"`
	ByProvince    map[string]int64 `json:"by_province"`
	ByCity        map[string]int64 `json:"by_city"`
	BySpecificity map[string]int64 `json:"by_specificity"`
}

// OverviewAggregation represents high-level pipeline statistics
type OverviewAggregation struct {
	TotalDocuments      int64          `json:"total_documents"`
	TotalCrimeRelated   int64          `json:"total_crime_related"`
	TopCities           []string       `json:"top_cities"`
	TopCrimeTypes       []string       `json:"top_crime_types"`
	QualityDistribution QualityBuckets `json:"quality_distribution"`
}

// QualityBuckets represents quality score distribution
type QualityBuckets struct {
	High   int64 `json:"high"`   // 70-100
	Medium int64 `json:"medium"` // 40-69
	Low    int64 `json:"low"`    // 0-39
}

// AggregationRequest represents a request for aggregated statistics
type AggregationRequest struct {
	Filters *DocumentFilters `json:"filters,omitempty"`
}

// SourceHealth represents per-source pipeline health metrics from Elasticsearch
type SourceHealth struct {
	Source          string  `json:"source"`
	RawCount        int64   `json:"raw_count"`
	ClassifiedCount int64   `json:"classified_count"`
	Backlog         int64   `json:"backlog"`
	Delta24h        int64   `json:"delta_24h"`
	AvgQuality      float64 `json:"avg_quality"`
}

// SourceHealthResponse represents the response for source health aggregation
type SourceHealthResponse struct {
	Sources []SourceHealth `json:"sources"`
	Total   int            `json:"total"`
}

// ClassificationDriftAggregation represents content_type and crime_relevance counts for drift detection.
// Uses crime.street_crime_relevance (classifier field name).
type ClassificationDriftAggregation struct {
	ByContentType     map[string]int64            `json:"by_content_type"`
	ByCrimeRelevance  map[string]int64            `json:"by_crime_relevance"`
	ContentTypeXCrime map[string]map[string]int64 `json:"content_type_x_crime"`
	TotalDocuments    int64                       `json:"total_documents"`
}

// ClassificationDriftRequest holds optional filters for classification drift (e.g. hours, source).
type ClassificationDriftRequest struct {
	Hours   int      `json:"hours,omitempty"` // default 24
	Sources []string `json:"sources,omitempty"`
}

// ContentTypeMismatchCount is the count of docs with content_type=page AND crime.street_crime_relevance=core_street_crime.
type ContentTypeMismatchCount struct {
	Count int64 `json:"count"`
}

// SuspectedMisclassificationDoc is a single document flagged as possible misclassification (page + crime topic).
type SuspectedMisclassificationDoc struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	CanonicalURL   string  `json:"canonical_url"`
	ContentType    string  `json:"content_type"`
	CrimeRelevance string  `json:"crime_relevance"`
	Confidence     float64 `json:"confidence,omitempty"`
	CrawledAt      string  `json:"crawled_at,omitempty"`
}

// SuspectedMisclassificationResponse is the list of suspected misclassifications and total.
type SuspectedMisclassificationResponse struct {
	Documents []SuspectedMisclassificationDoc `json:"documents"`
	Total     int64                           `json:"total"`
}

// ClassificationDriftTimeseriesBucket is one day's bucket for content type drift over time.
type ClassificationDriftTimeseriesBucket struct {
	Date         string `json:"date"`
	ArticleCount int64  `json:"article_count"`
	PageCount    int64  `json:"page_count"`
	OtherCount   int64  `json:"other_count"`
	Total        int64  `json:"total"`
}

// ClassificationDriftTimeseriesResponse is the 7-day (or N-day) daily breakdown.
type ClassificationDriftTimeseriesResponse struct {
	Buckets []ClassificationDriftTimeseriesBucket `json:"buckets"`
}
