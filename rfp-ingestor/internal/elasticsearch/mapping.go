package elasticsearch

// RFPIndexMapping returns the Elasticsearch index mapping for rfp_classified_content.
// The index name matches the classifier's classified_content schema so the search API
// wildcard query *_classified_content picks up these documents.
func RFPIndexMapping() map[string]any {
	return map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]any{
			"properties": map[string]any{
				"title":         map[string]any{"type": "text", "analyzer": "standard"},
				"url":           map[string]any{"type": "keyword"},
				"source_name":   map[string]any{"type": "keyword"},
				"content_type":  map[string]any{"type": "keyword"},
				"quality_score": map[string]any{"type": "integer"},
				"snippet":       map[string]any{"type": "text", "analyzer": "standard"},
				"topics":        map[string]any{"type": "keyword"},
				"crawled_at":    map[string]any{"type": "date"},
				"rfp": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"extraction_method": map[string]any{"type": "keyword"},
						"title":             map[string]any{"type": "text", "analyzer": "standard"},
						"reference_number":  map[string]any{"type": "keyword"},
						"organization_name": map[string]any{"type": "keyword"},
						"description":       map[string]any{"type": "text", "analyzer": "standard"},
						"published_date":    map[string]any{"type": "keyword"},
						"closing_date":      map[string]any{"type": "keyword"},
						"amendment_date":    map[string]any{"type": "keyword"},
						"amendment_number":  map[string]any{"type": "keyword"},
						"budget_currency":   map[string]any{"type": "keyword"},
						"procurement_type":  map[string]any{"type": "keyword"},
						"categories":        map[string]any{"type": "keyword"},
						"province":          map[string]any{"type": "keyword"},
						"city":              map[string]any{"type": "keyword"},
						"country":           map[string]any{"type": "keyword"},
						"source_url":        map[string]any{"type": "keyword"},
						"contact_name":      map[string]any{"type": "keyword"},
						"contact_email":     map[string]any{"type": "keyword"},
						"gsin":              map[string]any{"type": "keyword"},
						"unspsc":            map[string]any{"type": "keyword"},
						"tender_status":     map[string]any{"type": "keyword"},
					},
				},
			},
		},
	}
}
