package mcp

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
