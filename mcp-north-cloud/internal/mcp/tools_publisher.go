package mcp

//nolint:funlen // Tool definitions are data structures, not complex logic
func getPublisherTools() []Tool {
	return []Tool{
		{
			Name:  "create_channel",
			Scope: ScopeProd,
			Description: "Create a new publishing channel with embedded routing rules. Channels define " +
				"a Redis pub/sub topic and filtering rules (topics, quality, content types). " +
				"Returns: full channel object with rules.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Human-readable channel name (e.g., 'Crime Feed', 'Mining News')",
					},
					"slug": map[string]any{
						"type":        "string",
						"description": "URL-friendly identifier (e.g., 'crime_feed', 'mining_news')",
					},
					"redis_channel": map[string]any{
						"type":        "string",
						"description": "Redis pub/sub channel name (e.g., 'articles:crime', 'articles:mining')",
					},
					"description": map[string]any{
						"type":        "string",
						"description": "Human-readable description of what this channel publishes",
					},
					"rules": map[string]any{
						"type":        "object",
						"description": "Filtering rules for content routing",
						"properties": map[string]any{
							"include_topics": map[string]any{
								"type":        "array",
								"description": "Topics to include (content must match at least one)",
								"items":       map[string]any{"type": "string"},
							},
							"exclude_topics": map[string]any{
								"type":        "array",
								"description": "Topics to exclude",
								"items":       map[string]any{"type": "string"},
							},
							"min_quality_score": map[string]any{
								"type":        "integer",
								"description": "Minimum quality score (0-100)",
							},
							"content_types": map[string]any{
								"type":        "array",
								"description": "Content types to include (e.g., ['article', 'recipe'])",
								"items":       map[string]any{"type": "string"},
							},
						},
					},
					"enabled": map[string]any{
						"type":        "boolean",
						"description": "Whether the channel is active (default: true)",
					},
				},
				"required": []string{"name", "slug", "redis_channel"},
			},
		},
		{
			Name:  "list_channels",
			Scope: ScopeShared,
			Description: "List all publishing channels with their routing rules. Returns: channel IDs, " +
				"names, slugs, Redis channels, rules, and enabled status.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"active_only": map[string]any{
						"type":        "boolean",
						"description": "If true, return only active channels (default: false)",
					},
				},
			},
		},
		{
			Name:  "delete_channel",
			Scope: ScopeProd,
			Description: "Delete a publishing channel permanently. " +
				"Use when: A channel is no longer needed. Requires channel_id. This is irreversible.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"channel_id": map[string]any{
						"type":        "string",
						"description": "ID of the channel to delete",
					},
				},
				"required": []string{"channel_id"},
			},
		},
		{
			Name:  "preview_channel",
			Scope: ScopeShared,
			Description: "Preview a channel's configuration, routing rules, and matching content summary. " +
				"Use when: You want to verify a channel's rules or see what content it would match before making changes. " +
				"Requires channel_id.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"channel_id": map[string]any{
						"type":        "string",
						"description": "ID of the channel to preview",
					},
				},
				"required": []string{"channel_id"},
			},
		},
		{
			Name:  "get_publish_history",
			Scope: ScopeShared,
			Description: "Get publishing history with optional channel filter and pagination. " +
				"Use when: Reviewing what content was published, when, and to which channels. " +
				"Returns: paginated history records (default 50, max 100).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"channel_name": map[string]any{
						"type":        "string",
						"description": "Filter by channel name",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Number of records to return (default: 50)",
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of records to skip (default: 0)",
					},
				},
			},
		},
		{
			Name:  "get_publisher_stats",
			Scope: ScopeShared,
			Description: "Get publisher statistics including total published content by channel. " +
				"Use when: You need an overview of publishing volume and channel activity. No arguments required.",
			InputSchema: map[string]any{
				"type": "object",
			},
		},
	}
}
