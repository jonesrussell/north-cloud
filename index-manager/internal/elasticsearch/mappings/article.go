package mappings

// GetArticleMapping returns the Elasticsearch mapping for legacy article indexes
func GetArticleMapping() map[string]any {
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
				"title": map[string]any{
					"type": "text",
				},
				"body": map[string]any{
					"type": "text",
				},
				"author": map[string]any{
					"type": "keyword",
				},
				"byline_name": map[string]any{
					"type": "keyword",
				},
				"published_date": map[string]any{
					"type": "date",
				},
				"source": map[string]any{
					"type": "keyword",
				},
				"tags": map[string]any{
					"type": "keyword",
				},
				"keywords": map[string]any{
					"type": "keyword",
				},
				"intro": map[string]any{
					"type": "text",
				},
				"description": map[string]any{
					"type": "text",
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
				"word_count": map[string]any{
					"type": "integer",
				},
				"category": map[string]any{
					"type": "keyword",
				},
				"section": map[string]any{
					"type": "keyword",
				},
				"created_at": map[string]any{
					"type": "date",
				},
				"updated_at": map[string]any{
					"type": "date",
				},
			},
		},
	}
}
