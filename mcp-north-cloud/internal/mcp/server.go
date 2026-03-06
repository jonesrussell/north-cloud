package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/client"
	"github.com/north-cloud/infrastructure/logger"
)

// Server handles MCP protocol requests
type Server struct {
	env              string
	log              logger.Logger
	rateLimiter      *RateLimiter
	serviceURLs      map[string]string
	indexClient      *client.IndexManagerClient
	crawlerClient    *client.CrawlerClient
	sourceClient     *client.SourceManagerClient
	publisherClient  *client.PublisherClient
	searchClient     *client.SearchClient
	classifierClient *client.ClassifierClient
	authClient       *client.AuthenticatedClient
	grafanaClient    *client.GrafanaClient
	ollamaURL        string // empty = extract_schema unavailable
	ollamaModel      string
	rendererURL      string // empty = js_render unavailable
}

// ServerOption configures optional Server fields.
type ServerOption func(*Server)

// WithLogger sets the server's logger for audit logging.
func WithLogger(log logger.Logger) ServerOption {
	return func(s *Server) { s.log = log }
}

// WithServiceURLs sets the map of service name → base URL for health checks.
func WithServiceURLs(urls map[string]string) ServerOption {
	return func(s *Server) { s.serviceURLs = urls }
}

// NewServer creates a new MCP server
func NewServer(
	env string,
	indexClient *client.IndexManagerClient,
	crawlerClient *client.CrawlerClient,
	sourceClient *client.SourceManagerClient,
	publisherClient *client.PublisherClient,
	searchClient *client.SearchClient,
	classifierClient *client.ClassifierClient,
	authClient *client.AuthenticatedClient,
	grafanaClient *client.GrafanaClient,
	ollamaURL, ollamaModel, rendererURL string,
	opts ...ServerOption,
) *Server {
	s := &Server{
		env:              env,
		rateLimiter:      NewRateLimiter(),
		indexClient:      indexClient,
		crawlerClient:    crawlerClient,
		sourceClient:     sourceClient,
		publisherClient:  publisherClient,
		searchClient:     searchClient,
		classifierClient: classifierClient,
		authClient:       authClient,
		grafanaClient:    grafanaClient,
		ollamaURL:        ollamaURL,
		ollamaModel:      ollamaModel,
		rendererURL:      rendererURL,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
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
	tools := getToolsForEnv(s.env)

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

// handleToolsCall executes a tool call with audit logging and rate limiting.
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

	// Rate limiting
	if !s.rateLimiter.Allow(params.Name) {
		return s.errorResponse(id, RateLimited, "Rate limit exceeded for tool: "+params.Name+". Try again shortly.")
	}

	// Execute with timing
	start := time.Now()
	resp := s.routeToolCall(ctx, id, params.Name, params.Arguments)
	duration := time.Since(start)

	// Audit log
	entry := AuditEntry{
		ToolName:   params.Name,
		RequestID:  id,
		DurationMs: duration.Milliseconds(),
		Success:    resp.Error == nil,
		Timestamp:  start,
		ParamKeys:  extractParamKeys(params.Arguments),
	}
	if resp.Error != nil {
		entry.ErrorCode = resp.Error.Code
	}
	if resp.Result != nil {
		entry.ResultBytes = len(resp.Result)
	}
	logToolAudit(s.log, entry)

	return resp
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
	"create_channel":      (*Server).handleCreateChannel,
	"list_channels":       (*Server).handleListChannels,
	"delete_channel":      (*Server).handleDeleteChannel,
	"preview_channel":     (*Server).handlePreviewChannel,
	"get_publish_history": (*Server).handleGetPublishHistory,
	"get_publisher_stats": (*Server).handleGetPublisherStats,
	"search_content":      (*Server).handleSearchContent,
	"classify_content":    (*Server).handleClassifyContent,
	"delete_index":        (*Server).handleDeleteIndex,
	"list_indexes":        (*Server).handleListIndexes,
	"get_grafana_alerts":  (*Server).handleGetGrafanaAlerts,
	"fetch_url":           (*Server).handleFetchURL,
	"lint_file":           (*Server).handleLintFile,
	"build_service":       (*Server).handleBuildService,
	"test_service":        (*Server).handleTestService,
	"health_check":        (*Server).handleHealthCheck,
}

// toolScopeMap builds a name->scope lookup from all tools.
func toolScopeMap() map[string]ToolScope {
	all := getAllTools()
	m := make(map[string]ToolScope, len(all))
	for _, t := range all {
		m[t.Name] = t.Scope
	}
	return m
}

var toolScopes = toolScopeMap()

func (s *Server) routeToolCall(ctx context.Context, id any, toolName string, arguments json.RawMessage) *Response {
	scope, exists := toolScopes[toolName]
	if !exists {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &ErrorObject{Code: MethodNotFound, Message: "Unknown tool: " + toolName},
		}
	}
	if !scope.IsAllowed(s.env) {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &ErrorObject{Code: MethodNotFound, Message: "Tool not available in " + s.env + " environment: " + toolName},
		}
	}
	if h, ok := toolHandlers[toolName]; ok {
		return h(s, ctx, id, arguments)
	}
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &ErrorObject{Code: MethodNotFound, Message: "Unknown tool: " + toolName},
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
		"nextCursor": "",
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
