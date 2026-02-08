package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
	authClient       *client.AuthenticatedClient
}

// NewServer creates a new MCP server
func NewServer(
	indexClient *client.IndexManagerClient,
	crawlerClient *client.CrawlerClient,
	sourceClient *client.SourceManagerClient,
	publisherClient *client.PublisherClient,
	searchClient *client.SearchClient,
	classifierClient *client.ClassifierClient,
	authClient *client.AuthenticatedClient,
) *Server {
	return &Server{
		indexClient:      indexClient,
		crawlerClient:    crawlerClient,
		sourceClient:     sourceClient,
		publisherClient:  publisherClient,
		searchClient:     searchClient,
		classifierClient: classifierClient,
		authClient:       authClient,
	}
}

// HandleRequest processes an MCP request and returns a response.
// Returns nil for notifications (requests without ID). Uses context.Background().
func (s *Server) HandleRequest(req *Request) *Response {
	return s.HandleRequestWithContext(context.Background(), req)
}

// HandleRequestWithContext processes an MCP request with the given context and returns a response.
// Returns nil for notifications (requests without ID) - they don't require responses.
func (s *Server) HandleRequestWithContext(ctx context.Context, req *Request) *Response {
	requestID := req.ID

	if req.Method == "initialize" {
		return s.handleInitialize(req, requestID)
	}
	if req.Method == "tools/list" {
		return s.handleToolsList(req, requestID)
	}
	if req.Method == "tools/call" {
		return s.handleToolsCall(ctx, req, requestID)
	}
	if req.Method == "prompts/list" {
		return s.handlePromptsList(req, requestID)
	}
	if req.Method == "prompts/get" {
		return s.handlePromptsGet(req, requestID)
	}
	if req.Method == "resources/list" {
		return s.handleResourcesList(req, requestID)
	}
	if req.Method == "resources/read" {
		return s.handleResourcesRead(req, requestID)
	}
	if req.Method == "ping" {
		return &Response{
			JSONRPC: "2.0",
			ID:      requestID,
			Result:  json.RawMessage(`"pong"`),
		}
	}
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
		"tools":     map[string]any{},
		"prompts":   map[string]any{"listChanged": false},
		"resources": map[string]any{"subscribe": false, "listChanged": false},
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
func (s *Server) handleToolsCall(ctx context.Context, req *Request, id any) *Response {
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
	return s.routeToolCall(ctx, id, params.Name, params.Arguments)
}

// toolHandlerFunc matches (*Server).handleX(ctx, id, args); receiver is bound when calling h(s, ctx, id, arguments).
type toolHandlerFunc func(s *Server, ctx context.Context, id any, arguments json.RawMessage) *Response

var toolHandlers = map[string]toolHandlerFunc{
	"get_auth_token":      (*Server).handleGetAuthToken,
	"onboard_source":      (*Server).handleOnboardSource,
	"start_crawl":         (*Server).handleStartCrawl,
	"schedule_crawl":      (*Server).handleScheduleCrawl,
	"list_crawl_jobs":     (*Server).handleListCrawlJobs,
	"control_crawl_job":   (*Server).handleControlCrawlJob,
	"get_crawl_stats":     (*Server).handleGetCrawlStats,
	"add_source":          (*Server).handleAddSource,
	"list_sources":        (*Server).handleListSources,
	"update_source":       (*Server).handleUpdateSource,
	"delete_source":       (*Server).handleDeleteSource,
	"test_source":         (*Server).handleTestSource,
	"create_route":        (*Server).handleCreateRoute,
	"list_routes":         (*Server).handleListRoutes,
	"create_channel":      (*Server).handleCreateChannel,
	"list_channels":       (*Server).handleListChannels,
	"delete_route":        (*Server).handleDeleteRoute,
	"preview_route":       (*Server).handlePreviewRoute,
	"get_publish_history": (*Server).handleGetPublishHistory,
	"get_publisher_stats": (*Server).handleGetPublisherStats,
	"search_articles":     (*Server).handleSearchArticles,
	"classify_article":    (*Server).handleClassifyArticle,
	"delete_index":        (*Server).handleDeleteIndex,
	"list_indexes":        (*Server).handleListIndexes,
	"lint_file":           (*Server).handleLintFile,
	"build_service":       (*Server).handleBuildService,
	"test_service":        (*Server).handleTestService,
}

func (s *Server) routeToolCall(ctx context.Context, id any, toolName string, arguments json.RawMessage) *Response {
	if h, ok := toolHandlers[toolName]; ok {
		return h(s, ctx, id, arguments)
	}
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &ErrorObject{
			Code:    MethodNotFound,
			Message: "Unknown tool: " + toolName,
		},
	}
}

// handleDeleteIndex handles the delete_index tool call
func (s *Server) handleDeleteIndex(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		IndexName string `json:"index_name"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: index_name is required")
	}

	if args.IndexName == "" {
		return s.errorResponse(id, InvalidParams, "index_name cannot be empty")
	}

	err := s.indexClient.DeleteIndex(ctx, args.IndexName)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to delete index: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"index_name": args.IndexName,
		"message":    fmt.Sprintf("Successfully deleted index: %s", args.IndexName),
	})
}

// handlePromptsList handles prompts/list: returns all prompts and null cursor.
func (s *Server) handlePromptsList(_ *Request, id any) *Response {
	prompts := getAllPrompts()
	result := map[string]any{
		"prompts":    prompts,
		"nextCursor": nil,
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &ErrorObject{Code: InternalError, Message: fmt.Sprintf("Failed to marshal result: %v", err)},
		}
	}
	return &Response{JSONRPC: "2.0", ID: id, Result: json.RawMessage(resultJSON)}
}

// handlePromptsGet handles prompts/get: validates required args and returns messages.
func (s *Server) handlePromptsGet(req *Request, id any) *Response {
	name, arguments, errMsg := parsePromptsGetParams(req.Params)
	if errMsg != "" {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &ErrorObject{Code: InvalidParams, Message: errMsg},
		}
	}
	messages, err := getPromptByName(name, arguments)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "missing required") {
			return &Response{JSONRPC: "2.0", ID: id, Error: &ErrorObject{Code: InvalidParams, Message: msg}}
		}
		return &Response{JSONRPC: "2.0", ID: id, Error: &ErrorObject{Code: InvalidParams, Message: msg}}
	}
	// Find description for response
	var description string
	for _, p := range getAllPrompts() {
		if p.Name == name {
			description = p.Description
			break
		}
	}
	result := map[string]any{
		"description": description,
		"messages":    messages,
	}
	resultJSON, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &ErrorObject{Code: InternalError, Message: fmt.Sprintf("Failed to marshal result: %v", marshalErr)},
		}
	}
	return &Response{JSONRPC: "2.0", ID: id, Result: json.RawMessage(resultJSON)}
}

// handleResourcesList handles resources/list: returns static resource list.
func (s *Server) handleResourcesList(_ *Request, id any) *Response {
	resources := getAllResources()
	result := map[string]any{"resources": resources}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &ErrorObject{Code: InternalError, Message: fmt.Sprintf("Failed to marshal result: %v", err)},
		}
	}
	return &Response{JSONRPC: "2.0", ID: id, Result: json.RawMessage(resultJSON)}
}

// resourcesReadParams for resources/read.
type resourcesReadParams struct {
	URI string `json:"uri"`
}

// handleResourcesRead handles resources/read: returns content for known northcloud:// URIs.
func (s *Server) handleResourcesRead(req *Request, id any) *Response {
	var params resourcesReadParams
	if unmarshalErr := json.Unmarshal(req.Params, &params); unmarshalErr != nil || params.URI == "" {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &ErrorObject{Code: InvalidParams, Message: "uri is required"},
		}
	}
	contents, err := readResource(params.URI)
	if err != nil {
		var notFound *ResourceNotFoundError
		if errors.As(err, &notFound) {
			return &Response{
				JSONRPC: "2.0",
				ID:      id,
				Error:   &ErrorObject{Code: ResourceNotFound, Message: err.Error()},
			}
		}
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &ErrorObject{Code: InternalError, Message: err.Error()},
		}
	}
	result := map[string]any{"contents": contents}
	resultJSON, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &ErrorObject{Code: InternalError, Message: fmt.Sprintf("Failed to marshal result: %v", marshalErr)},
		}
	}
	return &Response{JSONRPC: "2.0", ID: id, Result: json.RawMessage(resultJSON)}
}
