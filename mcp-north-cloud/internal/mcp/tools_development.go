package mcp

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
