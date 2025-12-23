package mappings

// GetRawContentMapping returns the Elasticsearch mapping for raw content indexes
func GetRawContentMapping() map[string]interface{} {
	indexFalse := false

	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 1,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type": "keyword",
				},
				"url": map[string]interface{}{
					"type": "keyword",
				},
				"source_name": map[string]interface{}{
					"type": "keyword",
				},
				"title": map[string]interface{}{
					"type":     "text",
					"analyzer": "standard",
				},
				"raw_html": map[string]interface{}{
					"type":  "text",
					"index": indexFalse, // Store but don't index (large field)
				},
				"raw_text": map[string]interface{}{
					"type":     "text",
					"analyzer": "standard",
				},
				"og_type": map[string]interface{}{
					"type": "keyword",
				},
				"og_title": map[string]interface{}{
					"type":     "text",
					"analyzer": "standard",
				},
				"og_description": map[string]interface{}{
					"type":     "text",
					"analyzer": "standard",
				},
				"og_image": map[string]interface{}{
					"type": "keyword",
				},
				"og_url": map[string]interface{}{
					"type": "keyword",
				},
				"meta_description": map[string]interface{}{
					"type":     "text",
					"analyzer": "standard",
				},
				"meta_keywords": map[string]interface{}{
					"type": "keyword",
				},
				"canonical_url": map[string]interface{}{
					"type": "keyword",
				},
				"author": map[string]interface{}{
					"type": "text",
				},
				"crawled_at": map[string]interface{}{
					"type":   "date",
					"format": "strict_date_optional_time||epoch_millis",
				},
				"published_date": map[string]interface{}{
					"type":   "date",
					"format": "strict_date_optional_time||epoch_millis",
				},
				"classification_status": map[string]interface{}{
					"type": "keyword",
				},
				"classified_at": map[string]interface{}{
					"type":   "date",
					"format": "strict_date_optional_time||epoch_millis",
				},
				"word_count": map[string]interface{}{
					"type": "integer",
				},
			},
		},
	}
}
