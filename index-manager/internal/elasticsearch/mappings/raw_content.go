package mappings

// GetRawContentMapping returns the Elasticsearch mapping for raw content indexes
func GetRawContentMapping() map[string]any {
	indexFalse := false

	return map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 1,
		},
		"mappings": map[string]any{
			"properties": map[string]any{
				"id": map[string]any{
					"type": "keyword",
				},
				"url": map[string]any{
					"type": "keyword",
				},
				"source_name": map[string]any{
					"type": "keyword",
				},
				"title": map[string]any{
					"type":     "text",
					"analyzer": "standard",
				},
				"raw_html": map[string]any{
					"type":  "text",
					"index": indexFalse, // Store but don't index (large field)
				},
				"raw_text": map[string]any{
					"type":     "text",
					"analyzer": "standard",
				},
				"og_type": map[string]any{
					"type": "keyword",
				},
				"og_title": map[string]any{
					"type":     "text",
					"analyzer": "standard",
				},
				"og_description": map[string]any{
					"type":     "text",
					"analyzer": "standard",
				},
				"og_image": map[string]any{
					"type": "keyword",
				},
				"og_url": map[string]any{
					"type": "keyword",
				},
				"meta_description": map[string]any{
					"type":     "text",
					"analyzer": "standard",
				},
				"meta_keywords": map[string]any{
					"type": "keyword",
				},
				"canonical_url": map[string]any{
					"type": "keyword",
				},
				"author": map[string]any{
					"type": "text",
				},
				"crawled_at": map[string]any{
					"type":   "date",
					"format": "strict_date_optional_time||epoch_millis",
				},
				"published_date": map[string]any{
					"type":   "date",
					"format": "strict_date_optional_time||epoch_millis",
				},
				"classification_status": map[string]any{
					"type": "keyword",
				},
				"classified_at": map[string]any{
					"type":   "date",
					"format": "strict_date_optional_time||epoch_millis",
				},
				"word_count": map[string]any{
					"type": "integer",
				},
			},
		},
	}
}
