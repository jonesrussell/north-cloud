package mcp

// getAuthTools returns authentication tools for JWT token generation.
func getAuthTools() []Tool {
	return []Tool{
		{
			Name:  "get_auth_token",
			Scope: ScopeProd,
			Description: "Generate a JWT auth token for manual API testing. Use when: You need to make " +
				"authenticated API calls to North Cloud services from CLI. Returns a 24-hour JWT token. " +
				"The MCP server already handles auth internally, so this is only needed for manual curl commands.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

// getWorkflowTools returns high-level workflow tools that orchestrate multiple services
func getWorkflowTools() []Tool {
	return []Tool{
		{
			Name:  "onboard_source",
			Scope: ScopeShared,
			Description: "Set up a content pipeline: creates a source and starts crawling. " +
				"Use when: User wants to add a new website/source and start crawling. " +
				"Returns: source_id, job_id. Prefer over add_source + schedule_crawl for new sources.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Name of the source (e.g., 'Example News')",
					},
					"url": map[string]any{
						"type":        "string",
						"description": "Base URL of the source to crawl",
					},
					"source_type": map[string]any{
						"type":        "string",
						"description": "Type of source (e.g., 'news', 'blog')",
					},
					"selectors": map[string]any{
						"type": "object",
						"description": "CSS selectors for content extraction. Minimal: {title: 'h1', body: 'article'}. " +
							"Optional: date (e.g. 'time[datetime]'), author (e.g. '.byline').",
					},
					"crawl_interval_minutes": map[string]any{
						"type":        "integer",
						"description": "Crawl interval in minutes (optional, omit for one-time crawl)",
					},
					"crawl_interval_type": map[string]any{
						"type":        "string",
						"description": "Interval type: 'minutes', 'hours', or 'days' (required if crawl_interval_minutes set)",
						"enum":        []string{"minutes", "hours", "days"},
					},
				},
				"required": []string{"name", "url", "source_type", "selectors"},
			},
		},
	}
}
