package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/client"
)

// Server handles MCP protocol requests
type Server struct {
	indexClient *client.IndexManagerClient
}

// NewServer creates a new MCP server
func NewServer(indexClient *client.IndexManagerClient) *Server {
	return &Server{
		indexClient: indexClient,
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

	// Handle delete_index tool
	if params.Name == "delete_index" {
		return s.handleDeleteIndex(id, params.Arguments)
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &ErrorObject{
			Code:    InvalidParams,
			Message: "Unknown tool",
		},
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
