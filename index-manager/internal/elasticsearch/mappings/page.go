package mappings

// GetPageMapping returns the Elasticsearch mapping for legacy page indexes
func GetPageMapping() map[string]interface{} {
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
				"title": map[string]interface{}{
					"type": "text",
				},
				"content": map[string]interface{}{
					"type": "text",
				},
				"description": map[string]interface{}{
					"type": "text",
				},
				"keywords": map[string]interface{}{
					"type": "keyword",
				},
				"og_title": map[string]interface{}{
					"type": "text",
				},
				"og_description": map[string]interface{}{
					"type": "text",
				},
				"og_image": map[string]interface{}{
					"type": "keyword",
				},
				"og_url": map[string]interface{}{
					"type": "keyword",
				},
				"canonical_url": map[string]interface{}{
					"type": "keyword",
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
				"status": map[string]interface{}{
					"type": "keyword",
				},
			},
		},
	}
}
