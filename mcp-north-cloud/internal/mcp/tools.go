package mcp

// Tool represents an MCP tool definition
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// getAllTools returns all available MCP tools grouped by service
func getAllTools() []Tool {
	tools := []Tool{}
	tools = append(tools, getCrawlerTools()...)
	tools = append(tools, getSourceManagerTools()...)
	tools = append(tools, getPublisherTools()...)
	tools = append(tools, getSearchTools()...)
	tools = append(tools, getClassifierTools()...)
	tools = append(tools, getIndexManagerTools()...)
	return tools
}

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
			Description: "List all crawl jobs with optional status filter.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status": map[string]any{
						"type":        "string",
						"description": "Filter by status (pending, scheduled, running, completed, failed, paused, cancelled)",
						"enum":        []string{"pending", "scheduled", "running", "completed", "failed", "paused", "cancelled"},
					},
				},
			},
		},
		{
			Name:        "pause_crawl_job",
			Description: "Pause a running or scheduled crawl job.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"job_id": map[string]any{
						"type":        "string",
						"description": "The ID of the job to pause",
					},
				},
				"required": []string{"job_id"},
			},
		},
		{
			Name:        "resume_crawl_job",
			Description: "Resume a paused crawl job.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"job_id": map[string]any{
						"type":        "string",
						"description": "The ID of the job to resume",
					},
				},
				"required": []string{"job_id"},
			},
		},
		{
			Name:        "cancel_crawl_job",
			Description: "Cancel a crawl job.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"job_id": map[string]any{
						"type":        "string",
						"description": "The ID of the job to cancel",
					},
				},
				"required": []string{"job_id"},
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

func getSourceManagerTools() []Tool {
	return []Tool{
		{
			Name:        "add_source",
			Description: "Add a new content source for crawling.",
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
						"type":        "object",
						"description": "CSS selectors for extracting content (JSON object)",
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
			Description: "List all configured content sources.",
			InputSchema: map[string]any{
				"type": "object",
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
			Name:        "test_source",
			Description: "Test crawl a source without saving the results. Useful for validating selectors before adding a source.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{
						"type":        "string",
						"description": "URL to test crawl",
					},
					"selectors": map[string]any{
						"type":        "object",
						"description": "CSS selectors to test (JSON object)",
					},
				},
				"required": []string{"url", "selectors"},
			},
		},
	}
}

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
			Description: "List all publishing routes with optional filters.",
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
			Name:        "search_articles",
			Description: "Full-text search across all classified content with filtering and facets.",
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
			Description: "List all Elasticsearch indexes.",
			InputSchema: map[string]any{
				"type": "object",
			},
		},
	}
}
