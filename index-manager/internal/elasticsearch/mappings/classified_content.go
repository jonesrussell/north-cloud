package mappings

// getRawContentFields returns the raw content field definitions
func getRawContentFields() map[string]interface{} {
	indexFalse := false
	return map[string]interface{}{
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
			"index": indexFalse, // Store but don't index
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
	}
}

// getClassificationFields returns the classification result field definitions
func getClassificationFields() map[string]interface{} {
	return map[string]interface{}{
		"content_type": map[string]interface{}{
			"type": "keyword",
		},
		"content_subtype": map[string]interface{}{
			"type": "keyword",
		},
		"quality_score": map[string]interface{}{
			"type": "integer",
		},
		"quality_factors": map[string]interface{}{
			"type": "object",
		},
		"topics": map[string]interface{}{
			"type": "keyword",
		},
		"topic_scores": map[string]interface{}{
			"type": "object",
		},
		"is_crime_related": map[string]interface{}{
			"type": "boolean",
		},
		"source_reputation": map[string]interface{}{
			"type": "integer",
		},
		"source_category": map[string]interface{}{
			"type": "keyword",
		},
		"classifier_version": map[string]interface{}{
			"type": "keyword",
		},
		"classification_method": map[string]interface{}{
			"type": "keyword",
		},
		"model_version": map[string]interface{}{
			"type": "keyword",
		},
		"confidence": map[string]interface{}{
			"type": "float",
		},
	}
}

// GetClassifiedContentMapping returns the Elasticsearch mapping for classified content indexes
func GetClassifiedContentMapping() map[string]interface{} {
	properties := make(map[string]interface{})

	// Add raw content fields
	for k, v := range getRawContentFields() {
		properties[k] = v
	}

	// Add classification fields
	for k, v := range getClassificationFields() {
		properties[k] = v
	}

	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 1,
		},
		"mappings": map[string]interface{}{
			"properties": properties,
		},
	}
}
