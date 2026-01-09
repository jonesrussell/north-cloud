package mappings

// GetPageMapping returns the Elasticsearch mapping for legacy page indexes
func GetPageMapping() map[string]any {
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
				"title": map[string]any{
					"type": "text",
				},
				"content": map[string]any{
					"type": "text",
				},
				"description": map[string]any{
					"type": "text",
				},
				"keywords": map[string]any{
					"type": "keyword",
				},
				"og_title": map[string]any{
					"type": "text",
				},
				"og_description": map[string]any{
					"type": "text",
				},
				"og_image": map[string]any{
					"type": "keyword",
				},
				"og_url": map[string]any{
					"type": "keyword",
				},
				"canonical_url": map[string]any{
					"type": "keyword",
				},
				"created_at": map[string]any{
					"type": "date",
				},
				"updated_at": map[string]any{
					"type": "date",
				},
				"status": map[string]any{
					"type": "keyword",
				},
			},
		},
	}
}
