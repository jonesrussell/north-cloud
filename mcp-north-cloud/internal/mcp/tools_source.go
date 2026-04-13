package mcp

//nolint:funlen // Tool definitions are data structures, not complex logic
func getSourceManagerTools() []Tool {
	return []Tool{
		{
			Name:  "add_source",
			Scope: ScopeShared,
			Description: "Add a new content source for crawling. Use when: Only need to register a source " +
				"without crawling. For full setup, prefer onboard_source.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Name of the source",
					},
					"url": map[string]any{
						"type":        "string",
						"description": "Base URL of the source",
					},
					"type": map[string]any{
						"type":        "string",
						"description": "Type of source (e.g., 'news', 'blog')",
					},
					"selectors": map[string]any{
						"type": "object",
						"description": "CSS selectors for content extraction. Minimal: {title: 'h1', body: 'article'}. " +
							"Optional: date (e.g. 'time[datetime]'), author (e.g. '.byline').",
					},
					"active": map[string]any{
						"type":        "boolean",
						"description": "Whether the source is active",
					},
					"feed_url": map[string]any{
						"type":        "string",
						"description": "RSS/Atom feed URL for the source (optional, enables feed-based crawling)",
					},
				},
				"required": []string{"name", "url", "type", "selectors"},
			},
		},
		{
			Name:  "list_sources",
			Scope: ScopeShared,
			Description: "List all configured content sources with pagination. " +
				"Use when: You need to see what sources are registered, find a source_id, or check source status. " +
				"Returns: paginated list with id, name, url, type, active status (default 20, max 100).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of sources to return (default: 20, max: 100)",
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of sources to skip for pagination (default: 0)",
					},
				},
			},
		},
		{
			Name:  "update_source",
			Scope: ScopeShared,
			Description: "Update an existing source configuration. " +
				"Use when: You need to change a source's name, URL, selectors, active status, or feed URL. " +
				"Only source_id is required; omitted fields keep their current values.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"source_id": map[string]any{
						"type":        "string",
						"description": "ID of the source to update",
					},
					"name": map[string]any{
						"type":        "string",
						"description": "New name for the source",
					},
					"url": map[string]any{
						"type":        "string",
						"description": "New URL for the source",
					},
					"selectors": map[string]any{
						"type":        "object",
						"description": "New selectors (JSON object)",
					},
					"active": map[string]any{
						"type":        "boolean",
						"description": "New active status",
					},
					"feed_url": map[string]any{
						"type":        "string",
						"description": "RSS/Atom feed URL for the source (enables feed-based crawling)",
					},
					"feed_poll_interval_minutes": map[string]any{
						"type":        "integer",
						"description": "How often to poll the feed in minutes (0 = never poll). Typical: 60",
					},
					"ingestion_mode": map[string]any{
						"type":        "string",
						"description": "How content is ingested: 'feed' (RSS polling), 'crawl' (link discovery), or '' (default)",
					},
				},
				"required": []string{"source_id"},
			},
		},
		{
			Name:  "enable_feed",
			Scope: ScopeShared,
			Description: "Re-enable a source's feed after it was auto-disabled by the crawler. " +
				"Use when: A source's feed was disabled due to network errors or bad URLs, and you've fixed the issue. " +
				"Clears feed_disabled_at and feed_disable_reason so the crawler resumes polling.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"source_id": map[string]any{
						"type":        "string",
						"description": "ID of the source whose feed to re-enable",
					},
				},
				"required": []string{"source_id"},
			},
		},
		{
			Name:  "delete_source",
			Scope: ScopeProd,
			Description: "Delete a content source permanently. " +
				"Use when: A source is no longer needed and should be removed. Requires source_id. This is irreversible.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"source_id": map[string]any{
						"type":        "string",
						"description": "ID of the source to delete",
					},
				},
				"required": []string{"source_id"},
			},
		},
		{
			Name:  "test_source",
			Scope: ScopeShared,
			Description: "Test crawl a source without saving the results. Use when: Validating selectors " +
				"before adding a source. Call before add_source or onboard_source if selectors are uncertain.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{
						"type":        "string",
						"description": "URL to test crawl",
					},
					"selectors": map[string]any{
						"type": "object",
						"description": "CSS selectors to test. Minimal: {title: 'h1', body: 'article'}. " +
							"Optional: date (e.g. 'time[datetime]'), author (e.g. '.byline').",
					},
				},
				"required": []string{"url", "selectors"},
			},
		},
	}
}
