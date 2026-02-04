package mcp

// getAllTools returns all available MCP tools grouped by service
func getAllTools() []Tool {
	const (
		toolGroupCount         = 8
		estimatedToolsPerGroup = 8
	)
	tools := make([]Tool, 0, toolGroupCount*estimatedToolsPerGroup)
	tools = append(tools, getWorkflowTools()...)
	tools = append(tools, getCrawlerTools()...)
	tools = append(tools, getSourceManagerTools()...)
	tools = append(tools, getPublisherTools()...)
	tools = append(tools, getSearchTools()...)
	tools = append(tools, getClassifierTools()...)
	tools = append(tools, getIndexManagerTools()...)
	tools = append(tools, getAuthTools()...)
	tools = append(tools, getDevelopmentTools()...)
	return tools
}

// getWorkflowTools returns high-level workflow tools that orchestrate multiple services
func getWorkflowTools() []Tool {
	return []Tool{
		{
			Name: "onboard_source",
			Description: "Set up a complete content pipeline in one step: creates a source, starts crawling, " +
				"and optionally configures a publishing route. Use when: User wants to add a new website/source and start crawling. " +
				"Returns: source_id, job_id, optional route_id. Prefer over add_source + schedule_crawl for new sources.",
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
					"channel_id": map[string]any{
						"type":        "string",
						"description": "Channel ID to publish to (optional, omit to skip route creation)",
					},
					"min_quality_score": map[string]any{
						"type":        "integer",
						"description": "Minimum quality score for publishing (0-100, default: 50, only used if channel_id provided)",
					},
					"topics": map[string]any{
						"type":        "array",
						"description": "Topics to filter by when publishing (optional)",
						"items": map[string]any{
							"type": "string",
						},
					},
				},
				"required": []string{"name", "url", "source_type", "selectors"},
			},
		},
	}
}

//nolint:funlen // Tool definitions are data structures, not complex logic
func getCrawlerTools() []Tool {
	return []Tool{
		{
			Name:        "start_crawl",
			Description: "Start a crawl job immediately. Creates a new job that runs once without scheduling.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"source_id": map[string]any{
						"type":        "string",
						"description": "The ID of the source to crawl (from source-manager)",
					},
					"url": map[string]any{
						"type":        "string",
						"description": "The URL to crawl",
					},
				},
				"required": []string{"source_id", "url"},
			},
		},
		{
			Name:        "schedule_crawl",
			Description: "Schedule a recurring crawl job with interval-based scheduling.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"source_id": map[string]any{
						"type":        "string",
						"description": "The ID of the source to crawl",
					},
					"url": map[string]any{
						"type":        "string",
						"description": "The URL to crawl",
					},
					"interval_minutes": map[string]any{
						"type":        "integer",
						"description": "Interval in minutes/hours/days (e.g., 30 for every 30 minutes)",
					},
					"interval_type": map[string]any{
						"type":        "string",
						"description": "Type of interval: 'minutes', 'hours', or 'days'",
						"enum":        []string{"minutes", "hours", "days"},
					},
				},
				"required": []string{"source_id", "url", "interval_minutes", "interval_type"},
			},
		},
		{
			Name:        "list_crawl_jobs",
			Description: "List all crawl jobs with optional status filter and pagination.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status": map[string]any{
						"type":        "string",
						"description": "Filter by status (pending, scheduled, running, completed, failed, paused, cancelled)",
						"enum":        []string{"pending", "scheduled", "running", "completed", "failed", "paused", "cancelled"},
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of jobs to return (default: 20, max: 100)",
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of jobs to skip for pagination (default: 0)",
					},
				},
			},
		},
		{
			Name:        "control_crawl_job",
			Description: "Control a crawl job's state: pause, resume, or cancel. Use when: User wants to pause, resume, or cancel a crawl job.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"job_id": map[string]any{
						"type":        "string",
						"description": "The ID of the job to control",
					},
					"action": map[string]any{
						"type":        "string",
						"description": "Action to perform: 'pause' (stop scheduled job), 'resume' (restart paused job), or 'cancel' (permanently stop job)",
						"enum":        []string{"pause", "resume", "cancel"},
					},
				},
				"required": []string{"job_id", "action"},
			},
		},
		{
			Name:        "get_crawl_stats",
			Description: "Get statistics for a crawl job including success rate and execution history.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"job_id": map[string]any{
						"type":        "string",
						"description": "The ID of the job to get stats for",
					},
				},
				"required": []string{"job_id"},
			},
		},
	}
}

//nolint:funlen // Tool definitions are data structures, not complex logic
func getSourceManagerTools() []Tool {
	return []Tool{
		{
			Name: "add_source",
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
				},
				"required": []string{"name", "url", "type", "selectors"},
			},
		},
		{
			Name:        "list_sources",
			Description: "List all configured content sources with pagination.",
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
			Name:        "update_source",
			Description: "Update an existing source configuration.",
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
				},
				"required": []string{"source_id"},
			},
		},
		{
			Name:        "delete_source",
			Description: "Delete a content source.",
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
			Name: "test_source",
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

//nolint:funlen // Tool definitions are data structures, not complex logic
func getPublisherTools() []Tool {
	return []Tool{
		{
			Name:        "create_route",
			Description: "Create a new publishing route that connects a source to a channel with quality and topic filters.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"source_id": map[string]any{
						"type":        "string",
						"description": "ID of the publisher source",
					},
					"channel_id": map[string]any{
						"type":        "string",
						"description": "ID of the channel to publish to",
					},
					"min_quality_score": map[string]any{
						"type":        "integer",
						"description": "Minimum quality score (0-100) for articles to publish",
					},
					"topics": map[string]any{
						"type":        "array",
						"description": "Topics to filter by (e.g., ['crime', 'news'])",
						"items": map[string]any{
							"type": "string",
						},
					},
					"active": map[string]any{
						"type":        "boolean",
						"description": "Whether the route is active",
					},
				},
				"required": []string{"source_id", "channel_id", "min_quality_score"},
			},
		},
		{
			Name:        "list_routes",
			Description: "List all publishing routes with optional filters and pagination.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"source_id": map[string]any{
						"type":        "string",
						"description": "Filter by source ID",
					},
					"channel_id": map[string]any{
						"type":        "string",
						"description": "Filter by channel ID",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of routes to return (default: 20, max: 100)",
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of routes to skip for pagination (default: 0)",
					},
				},
			},
		},
		{
			Name: "create_channel",
			Description: "Create a new publishing channel. Use when: User wants to set up a new Redis pub/sub " +
				"topic for article routing. Returns: channel_id, name, and status. Channel names typically " +
				"follow 'articles:{topic}' pattern (e.g., 'articles:crime', 'articles:news').",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Channel name, typically 'articles:{topic}' (e.g., 'articles:crime', 'articles:local')",
					},
					"description": map[string]any{
						"type":        "string",
						"description": "Human-readable description of what this channel publishes",
					},
					"enabled": map[string]any{
						"type":        "boolean",
						"description": "Whether the channel is active (default: true)",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name: "list_channels",
			Description: "List all publishing channels. Use when: User wants to see available channels for " +
				"routing or needs a channel_id for create_route/onboard_source. Returns: channel IDs, " +
				"names, descriptions, and active status.",
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
			Name:        "delete_route",
			Description: "Delete a publishing route.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"route_id": map[string]any{
						"type":        "string",
						"description": "ID of the route to delete",
					},
				},
				"required": []string{"route_id"},
			},
		},
		{
			Name:        "preview_route",
			Description: "Preview articles that would be published by a route without actually publishing them.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"route_id": map[string]any{
						"type":        "string",
						"description": "ID of the route to preview",
					},
				},
				"required": []string{"route_id"},
			},
		},
		{
			Name:        "get_publish_history",
			Description: "Get publishing history with pagination.",
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
			Name:        "get_publisher_stats",
			Description: "Get publisher statistics including total published and articles by channel.",
			InputSchema: map[string]any{
				"type": "object",
			},
		},
	}
}

func getSearchTools() []Tool {
	return []Tool{
		{
			Name: "search_articles",
			Description: "Full-text search across all classified content with filtering and facets. " +
				"Use when: User wants to find articles by keyword, topic, or quality. Returns up to 20 results per page.",
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
			Name:        "classify_article",
			Description: "Classify a single article to determine content type, quality score, topics, and crime detection.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title": map[string]any{
						"type":        "string",
						"description": "Article title",
					},
					"raw_text": map[string]any{
						"type":        "string",
						"description": "Article text content",
					},
					"url": map[string]any{
						"type":        "string",
						"description": "Article URL",
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

func getIndexManagerTools() []Tool {
	return []Tool{
		{
			Name:        "delete_index",
			Description: "Deletes an Elasticsearch index by name. This operation is irreversible and will permanently delete the index and all its documents.",
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
			Name:        "list_indexes",
			Description: "List all Elasticsearch indexes with pagination.",
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

func getAuthTools() []Tool {
	return []Tool{
		{
			Name: "get_auth_token",
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

func getDevelopmentTools() []Tool {
	return []Tool{
		{
			Name: "lint_file",
			Description: "Lint a specific file or service. Automatically detects Go files vs " +
				"Vue.js/TypeScript frontend files and runs the appropriate linter " +
				"(golangci-lint for Go, ESLint for frontend).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type": "string",
						"description": "Absolute or relative path to the file to lint " +
							"(e.g., '/home/jones/dev/north-cloud/crawler/main.go' or 'crawler/main.go')",
					},
					"service_name": map[string]any{
						"type": "string",
						"description": "Service name to lint entire service " +
							"(Go: crawler, source-manager, classifier, publisher, index-manager, search, auth, mcp-north-cloud | " +
							"Frontend: dashboard, search-frontend). " +
							"If provided, lints the entire service instead of a single file.",
					},
				},
			},
		},
		{
			Name:        "build_service",
			Description: "Build a North Cloud service. Runs 'task build' for Go services or 'npm run build' for frontend.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"service_name": map[string]any{
						"type": "string",
						"description": "Service to build: crawler, source-manager, classifier, publisher, auth, " +
							"index-manager, search, mcp-north-cloud, dashboard, search-frontend",
					},
				},
				"required": []string{"service_name"},
			},
		},
		{
			Name:        "test_service",
			Description: "Run tests for a North Cloud service. Runs 'task test' or 'npm run test'.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"service_name": map[string]any{
						"type": "string",
						"description": "Service to test: crawler, source-manager, classifier, publisher, auth, " +
							"index-manager, search, mcp-north-cloud, dashboard, search-frontend",
					},
					"with_coverage": map[string]any{
						"type":        "boolean",
						"description": "If true, run tests with coverage (Go: task test:coverage)",
					},
				},
				"required": []string{"service_name"},
			},
		},
	}
}
