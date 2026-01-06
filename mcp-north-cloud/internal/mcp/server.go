package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/client"
)

// Server handles MCP protocol requests
type Server struct {
	indexClient      *client.IndexManagerClient
	crawlerClient    *client.CrawlerClient
	sourceClient     *client.SourceManagerClient
	publisherClient  *client.PublisherClient
	searchClient     *client.SearchClient
	classifierClient *client.ClassifierClient
}

// NewServer creates a new MCP server
func NewServer(
	indexClient *client.IndexManagerClient,
	crawlerClient *client.CrawlerClient,
	sourceClient *client.SourceManagerClient,
	publisherClient *client.PublisherClient,
	searchClient *client.SearchClient,
	classifierClient *client.ClassifierClient,
) *Server {
	return &Server{
		indexClient:      indexClient,
		crawlerClient:    crawlerClient,
		sourceClient:     sourceClient,
		publisherClient:  publisherClient,
		searchClient:     searchClient,
		classifierClient: classifierClient,
	}
}

// HandleRequest processes an MCP request and returns a response
// Returns nil for notifications (requests without ID) - they don't require responses
func (s *Server) HandleRequest(req *Request) *Response {
	// For notifications (no ID), we can still process but caller should not send response
	// Use the request ID if present, otherwise nil (caller will handle)
	requestID := req.ID

	// Handle initialize request
	if req.Method == "initialize" {
		return s.handleInitialize(req, requestID)
	}

	// Handle tools/list request
	if req.Method == "tools/list" {
		return s.handleToolsList(req, requestID)
	}

	// Handle tools/call request
	if req.Method == "tools/call" {
		return s.handleToolsCall(req, requestID)
	}

	// Handle ping/pong for keepalive
	if req.Method == "ping" {
		return &Response{
			JSONRPC: "2.0",
			ID:      requestID,
			Result:  json.RawMessage(`"pong"`),
		}
	}

	// Unknown method - only return error if this was a request (has ID)
	// Notifications (no ID) don't require responses
	if requestID == nil {
		return nil
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      requestID,
		Error: &ErrorObject{
			Code:    MethodNotFound,
			Message: "Method not found",
		},
	}
}

// handleInitialize handles the initialize request
func (s *Server) handleInitialize(_ *Request, id any) *Response {
	capabilities := map[string]any{
		"tools": map[string]any{},
	}

	serverInfo := map[string]any{
		"name":    "north-cloud-mcp",
		"version": "1.0.0",
	}

	result := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    capabilities,
		"serverInfo":      serverInfo,
	}

	resultJSON, _ := json.Marshal(result)

	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  json.RawMessage(resultJSON),
	}
}

// handleToolsList returns the list of available tools
func (s *Server) handleToolsList(_ *Request, id any) *Response {
	tools := []Tool{
		// Crawler tools
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
		// Source Manager tools
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
		// Publisher tools
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
		// Search tools
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
		// Classifier tools
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
		// Index Manager tools
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

	result := map[string]any{
		"tools": tools,
	}

	resultJSON, _ := json.Marshal(result)

	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  json.RawMessage(resultJSON),
	}
}

// handleToolsCall executes a tool call
func (s *Server) handleToolsCall(req *Request, id any) *Response {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &ErrorObject{
				Code:    InvalidParams,
				Message: "Invalid parameters",
			},
		}
	}

	// Route to appropriate handler based on tool name
	switch params.Name {
	// Crawler tools
	case "start_crawl":
		return s.handleStartCrawl(id, params.Arguments)
	case "schedule_crawl":
		return s.handleScheduleCrawl(id, params.Arguments)
	case "list_crawl_jobs":
		return s.handleListCrawlJobs(id, params.Arguments)
	case "pause_crawl_job":
		return s.handlePauseCrawlJob(id, params.Arguments)
	case "resume_crawl_job":
		return s.handleResumeCrawlJob(id, params.Arguments)
	case "cancel_crawl_job":
		return s.handleCancelCrawlJob(id, params.Arguments)
	case "get_crawl_stats":
		return s.handleGetCrawlStats(id, params.Arguments)

	// Source Manager tools
	case "add_source":
		return s.handleAddSource(id, params.Arguments)
	case "list_sources":
		return s.handleListSources(id, params.Arguments)
	case "update_source":
		return s.handleUpdateSource(id, params.Arguments)
	case "delete_source":
		return s.handleDeleteSource(id, params.Arguments)
	case "test_source":
		return s.handleTestSource(id, params.Arguments)

	// Publisher tools
	case "create_route":
		return s.handleCreateRoute(id, params.Arguments)
	case "list_routes":
		return s.handleListRoutes(id, params.Arguments)
	case "delete_route":
		return s.handleDeleteRoute(id, params.Arguments)
	case "preview_route":
		return s.handlePreviewRoute(id, params.Arguments)
	case "get_publish_history":
		return s.handleGetPublishHistory(id, params.Arguments)
	case "get_publisher_stats":
		return s.handleGetPublisherStats(id, params.Arguments)

	// Search tools
	case "search_articles":
		return s.handleSearchArticles(id, params.Arguments)

	// Classifier tools
	case "classify_article":
		return s.handleClassifyArticle(id, params.Arguments)

	// Index Manager tools
	case "delete_index":
		return s.handleDeleteIndex(id, params.Arguments)
	case "list_indexes":
		return s.handleListIndexes(id, params.Arguments)

	default:
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &ErrorObject{
				Code:    InvalidParams,
				Message: "Unknown tool: " + params.Name,
			},
		}
	}
}

// handleDeleteIndex handles the delete_index tool call
func (s *Server) handleDeleteIndex(id any, arguments json.RawMessage) *Response {
	var args struct {
		IndexName string `json:"index_name"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &ErrorObject{
				Code:    InvalidParams,
				Message: "Invalid arguments: index_name is required",
			},
		}
	}

	if args.IndexName == "" {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &ErrorObject{
				Code:    InvalidParams,
				Message: "index_name cannot be empty",
			},
		}
	}

	// Call index-manager API
	err := s.indexClient.DeleteIndex(args.IndexName)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &ErrorObject{
				Code:    InternalError,
				Message: fmt.Sprintf("Failed to delete index: %v", err),
			},
		}
	}

	result := map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": fmt.Sprintf("Successfully deleted index: %s", args.IndexName),
			},
		},
		"isError": false,
	}

	resultJSON, _ := json.Marshal(result)

	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  json.RawMessage(resultJSON),
	}
}
