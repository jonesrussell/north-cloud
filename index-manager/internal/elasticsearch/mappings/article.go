package mappings

// GetArticleMapping returns the Elasticsearch mapping for legacy article indexes
func GetArticleMapping() map[string]interface{} {
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
				"title": map[string]interface{}{
					"type": "text",
				},
				"body": map[string]interface{}{
					"type": "text",
				},
				"author": map[string]interface{}{
					"type": "keyword",
				},
				"byline_name": map[string]interface{}{
					"type": "keyword",
				},
				"published_date": map[string]interface{}{
					"type": "date",
				},
				"source": map[string]interface{}{
					"type": "keyword",
				},
				"tags": map[string]interface{}{
					"type": "keyword",
				},
				"keywords": map[string]interface{}{
					"type": "keyword",
				},
				"intro": map[string]interface{}{
					"type": "text",
				},
				"description": map[string]interface{}{
					"type": "text",
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
				"word_count": map[string]interface{}{
					"type": "integer",
				},
				"category": map[string]interface{}{
					"type": "keyword",
				},
				"section": map[string]interface{}{
					"type": "keyword",
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}
