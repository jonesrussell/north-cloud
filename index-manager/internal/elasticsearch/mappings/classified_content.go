package mappings

import "maps"

// getRawContentFields returns the raw content field definitions
func getRawContentFields() map[string]any {
	indexFalse := false
	return map[string]any{
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
			"index": indexFalse, // Store but don't index
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
	}
}

// getCrimeMapping returns the nested crime object mapping
func getCrimeMapping() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sub_label": map[string]any{
				"type": "keyword",
			},
			"primary_crime_type": map[string]any{
				"type": "keyword",
			},
			"relevance": map[string]any{
				"type": "keyword",
			},
			"crime_types": map[string]any{
				"type": "keyword",
			},
			"final_confidence": map[string]any{
				"type": "float",
			},
			"homepage_eligible": map[string]any{
				"type": "boolean",
			},
			"review_required": map[string]any{
				"type": "boolean",
			},
			"model_version": map[string]any{
				"type": "keyword",
			},
		},
	}
}

// getLocationMapping returns the nested location object mapping
func getLocationMapping() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{
				"type": "keyword",
			},
			"province": map[string]any{
				"type": "keyword",
			},
			"country": map[string]any{
				"type": "keyword",
			},
			"specificity": map[string]any{
				"type": "keyword",
			},
			"confidence": map[string]any{
				"type": "float",
			},
		},
	}
}

// getMiningMapping returns the nested mining object mapping
func getMiningMapping() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"relevance": map[string]any{
				"type": "keyword",
			},
			"mining_stage": map[string]any{
				"type": "keyword",
			},
			"commodities": map[string]any{
				"type": "keyword",
			},
			"location": map[string]any{
				"type": "keyword",
			},
			"final_confidence": map[string]any{
				"type": "float",
			},
			"review_required": map[string]any{
				"type": "boolean",
			},
			"model_version": map[string]any{
				"type": "keyword",
			},
		},
	}
}

// getClassificationFields returns the classification result field definitions
func getClassificationFields() map[string]any {
	return map[string]any{
		"content_type": map[string]any{
			"type": "keyword",
		},
		"content_subtype": map[string]any{
			"type": "keyword",
		},
		"quality_score": map[string]any{
			"type": "integer",
		},
		"quality_factors": map[string]any{
			"type": "object",
		},
		"topics": map[string]any{
			"type": "keyword",
		},
		"topic_scores": map[string]any{
			"type": "object",
		},
		"crime":    getCrimeMapping(),
		"location": getLocationMapping(),
		"mining":   getMiningMapping(),
		// Keep is_crime_related for backward compatibility (computed field)
		"is_crime_related": map[string]any{
			"type": "boolean",
		},
		"source_reputation": map[string]any{
			"type": "integer",
		},
		"source_category": map[string]any{
			"type": "keyword",
		},
		"classifier_version": map[string]any{
			"type": "keyword",
		},
		"classification_method": map[string]any{
			"type": "keyword",
		},
		"model_version": map[string]any{
			"type": "keyword",
		},
		"confidence": map[string]any{
			"type": "float",
		},
	}
}

// GetClassifiedContentMapping returns the Elasticsearch mapping for classified content indexes
func GetClassifiedContentMapping() map[string]any {
	properties := make(map[string]any)

	// Add raw content fields
	maps.Copy(properties, getRawContentFields())

	// Add classification fields
	maps.Copy(properties, getClassificationFields())

	return map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 1,
		},
		"mappings": map[string]any{
			"properties": properties,
		},
	}
}
