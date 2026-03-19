package mcp

func getSearchTools() []Tool {
	return []Tool{
		{
			Name:  "search_content",
			Scope: ScopeShared,
			Description: "Full-text search across all classified content with filtering and facets. " +
				"Use when: User wants to find content by keyword, topic, or quality. Returns up to 20 results per page.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Search query string",
					},
					"topics": map[string]any{
						"type":        "array",
						"description": "Filter by topics",
						"items": map[string]any{
							"type": "string",
						},
					},
					"content_type": map[string]any{
						"type":        "string",
						"description": "Filter by content type",
					},
					"min_quality_score": map[string]any{
						"type":        "integer",
						"description": "Minimum quality score",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "Page number (default: 1)",
					},
					"page_size": map[string]any{
						"type":        "integer",
						"description": "Results per page (default: 20, max: 100)",
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

func getClassifierTools() []Tool {
	return []Tool{
		{
			Name:  "classify_content",
			Scope: ScopeShared,
			Description: "Classify a single content item to determine content type, quality score, topics, and crime detection. " +
				"Use when: You want to preview how the classifier would categorize specific content. " +
				"Requires title, raw_text, url.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title": map[string]any{
						"type":        "string",
						"description": "Content title",
					},
					"raw_text": map[string]any{
						"type":        "string",
						"description": "Content text body",
					},
					"url": map[string]any{
						"type":        "string",
						"description": "Content URL",
					},
					"metadata": map[string]any{
						"type":        "object",
						"description": "Additional metadata (JSON object)",
					},
				},
				"required": []string{"title", "raw_text", "url"},
			},
		},
	}
}

func getObservabilityTools() []Tool {
	return []Tool{
		{
			Name:  "get_grafana_alerts",
			Scope: ScopeShared,
			Description: "Get active alerts from Grafana. Use when: User wants to check system health, " +
				"see firing alerts, or investigate incidents. Returns all active (firing) alerts with " +
				"their labels, annotations, and status.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"include_silenced": map[string]any{
						"type":        "boolean",
						"description": "Include silenced alerts in the response (default: false)",
					},
				},
			},
		},
	}
}

func getIndexManagerTools() []Tool {
	return []Tool{
		{
			Name:  "delete_index",
			Scope: ScopeProd,
			Description: "Delete an Elasticsearch index by name permanently. " +
				"Use when: An index is stale, corrupted, or no longer needed. " +
				"This is irreversible and deletes all documents. Requires index_name.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"index_name": map[string]any{
						"type":        "string",
						"description": "The name of the Elasticsearch index to delete (e.g., 'example_com_raw_content')",
					},
				},
				"required": []string{"index_name"},
			},
		},
		{
			Name:  "list_indexes",
			Scope: ScopeShared,
			Description: "List all Elasticsearch indexes with pagination. " +
				"Use when: You need to see which indexes exist in Elasticsearch. " +
				"Returns: paginated list of index names (default 20, max 100).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of indexes to return (default: 20, max: 100)",
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of indexes to skip for pagination (default: 0)",
					},
				},
			},
		},
	}
}
