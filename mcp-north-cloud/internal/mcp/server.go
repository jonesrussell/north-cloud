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

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &ErrorObject{
				Code:    InternalError,
				Message: fmt.Sprintf("Failed to marshal result: %v", err),
			},
		}
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  json.RawMessage(resultJSON),
	}
}

// handleToolsList returns the list of available tools
func (s *Server) handleToolsList(_ *Request, id any) *Response {
	tools := getAllTools()

	result := map[string]any{
		"tools": tools,
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &ErrorObject{
				Code:    InternalError,
				Message: fmt.Sprintf("Failed to marshal result: %v", err),
			},
		}
	}

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

	return s.routeToolCall(id, params.Name, params.Arguments)
}

// routeToolCall routes tool calls to appropriate handlers
func (s *Server) routeToolCall(id any, toolName string, arguments json.RawMessage) *Response {
	switch toolName {
	// Workflow tools (high-level, multi-service)
	case "onboard_source":
		return s.handleOnboardSource(id, arguments)

	// Crawler tools
	case "start_crawl":
		return s.handleStartCrawl(id, arguments)
	case "schedule_crawl":
		return s.handleScheduleCrawl(id, arguments)
	case "list_crawl_jobs":
		return s.handleListCrawlJobs(id, arguments)
	case "control_crawl_job":
		return s.handleControlCrawlJob(id, arguments)
	case "get_crawl_stats":
		return s.handleGetCrawlStats(id, arguments)

	// Source Manager tools
	case "add_source":
		return s.handleAddSource(id, arguments)
	case "list_sources":
		return s.handleListSources(id, arguments)
	case "update_source":
		return s.handleUpdateSource(id, arguments)
	case "delete_source":
		return s.handleDeleteSource(id, arguments)
	case "test_source":
		return s.handleTestSource(id, arguments)

	// Publisher tools
	case "create_route":
		return s.handleCreateRoute(id, arguments)
	case "list_routes":
		return s.handleListRoutes(id, arguments)
	case "create_channel":
		return s.handleCreateChannel(id, arguments)
	case "list_channels":
		return s.handleListChannels(id, arguments)
	case "delete_route":
		return s.handleDeleteRoute(id, arguments)
	case "preview_route":
		return s.handlePreviewRoute(id, arguments)
	case "get_publish_history":
		return s.handleGetPublishHistory(id, arguments)
	case "get_publisher_stats":
		return s.handleGetPublisherStats(id, arguments)

	// Search tools
	case "search_articles":
		return s.handleSearchArticles(id, arguments)

	// Classifier tools
	case "classify_article":
		return s.handleClassifyArticle(id, arguments)

	// Index Manager tools
	case "delete_index":
		return s.handleDeleteIndex(id, arguments)
	case "list_indexes":
		return s.handleListIndexes(id, arguments)

	// Development tools
	case "lint_file":
		return s.handleLintFile(id, arguments)

	default:
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error: &ErrorObject{
				Code:    InvalidParams,
				Message: "Unknown tool: " + toolName,
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
		return s.errorResponse(id, InvalidParams, "Invalid arguments: index_name is required")
	}

	if args.IndexName == "" {
		return s.errorResponse(id, InvalidParams, "index_name cannot be empty")
	}

	// Call index-manager API
	err := s.indexClient.DeleteIndex(args.IndexName)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to delete index: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"index_name": args.IndexName,
		"message":    fmt.Sprintf("Successfully deleted index: %s", args.IndexName),
	})
}
