package mcp

// getAllTools returns all available MCP tools grouped by service
func getAllTools() []Tool {
	const (
		toolGroupCount         = 10
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
	tools = append(tools, getObservabilityTools()...)
	tools = append(tools, getFetchTools()...)
	tools = append(tools, getDevelopmentTools()...)
	tools = append(tools, getSystemTools()...)
	return tools
}

// getSystemTools returns system-level tools for health checking and diagnostics.
func getSystemTools() []Tool {
	return []Tool{
		{
			Name:  "health_check",
			Scope: ScopeShared,
			Description: "Check connectivity to all North Cloud backend services. " +
				"Use when: You suspect a service is down, experiencing errors, or want to verify " +
				"the system is healthy before performing operations. " +
				"Returns: status of each service (reachable/unreachable) with response times.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

// getToolsForEnv returns tools filtered by environment scope.
func getToolsForEnv(env string) []Tool {
	all := getAllTools()
	filtered := make([]Tool, 0, len(all))
	for _, t := range all {
		if t.Scope.IsAllowed(env) {
			filtered = append(filtered, t)
		}
	}
	return filtered
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

//nolint:funlen // Tool definitions are data structures, not complex logic
func getCrawlerTools() []Tool {
	return []Tool{
		{
			Name:  "start_crawl",
			Scope: ScopeProd,
			Description: "Start a one-off crawl job that runs immediately without scheduling. " +
				"Use when: You need to crawl a source once right now. Requires source_id and url. Returns: job_id and status.",
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
			Name:  "schedule_crawl",
			Scope: ScopeProd,
			Description: "Schedule a recurring crawl job with interval-based scheduling. " +
				"Use when: You want a source crawled automatically on a repeating interval. " +
				"Requires source_id, url, interval_minutes, interval_type. Returns: job_id, next_run_at.",
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
			Name:  "list_crawl_jobs",
			Scope: ScopeShared,
			Description: "List all crawl jobs with optional status filter and pagination. " +
				"Use when: You need to see what crawl jobs exist, check their statuses, or find a specific job_id. " +
				"Returns: paginated list of jobs (default 20, max 100).",
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
			Scope:       ScopeProd,
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
			Name:  "get_crawl_stats",
			Scope: ScopeShared,
			Description: "Get statistics for a crawl job including success rate and execution history. " +
				"Use when: Debugging a failing crawl or reviewing crawl performance. " +
				"Requires job_id. Returns: total executions, success/failure counts, average duration.",
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

func getDevelopmentTools() []Tool {
	return []Tool{
		{
			Name:  "lint_file",
			Scope: ScopeLocal,
			Description: "Lint a specific file or service. Automatically detects Go files vs " +
				"Vue.js/TypeScript frontend files and runs the appropriate linter " +
				"(golangci-lint for Go, ESLint for frontend). " +
				"Use when: You want to check code quality before committing.",
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
			Name:  "build_service",
			Scope: ScopeLocal,
			Description: "Build a North Cloud service. Runs 'task build' for Go services or 'npm run build' for frontend. " +
				"Use when: You need to verify a service compiles successfully after code changes. Requires service_name.",
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
			Name:  "test_service",
			Scope: ScopeLocal,
			Description: "Run tests for a North Cloud service. Runs 'task test' or 'npm run test'. " +
				"Use when: You need to verify tests pass after code changes. Requires service_name. " +
				"Optional: with_coverage for Go coverage report.",
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

// getFetchTools returns the fetch_url tool for ad-hoc URL content extraction.
func getFetchTools() []Tool {
	return []Tool{
		{
			Name:  "fetch_url",
			Scope: ScopeProd,
			Description: "Fetch and extract text content from a single URL without requiring a pre-configured source. " +
				"Use for one-off lookups: job postings, documentation, news articles, or any page you want to read. " +
				"Optionally render JavaScript (js_render) or extract structured fields (extract_schema).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{
						"type":        "string",
						"description": "URL to fetch and extract content from",
					},
					"js_render": map[string]any{
						"type":        "boolean",
						"description": "Use headless browser for JS-heavy pages (requires renderer sidecar). Default: false",
					},
					"extract_schema": map[string]any{
						"type": "object",
						"description": "Field extraction schema. Keys are field names, values are descriptions. " +
							"Example: {\"title\": \"Job title\", \"salary\": \"Salary range\"}. Requires OLLAMA_URL.",
						"additionalProperties": map[string]any{
							"type": "string",
						},
					},
				},
				"required": []string{"url"},
			},
		},
	}
}
