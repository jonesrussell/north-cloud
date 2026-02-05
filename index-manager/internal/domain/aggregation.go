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
