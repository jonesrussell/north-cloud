package mcp

import (
	"encoding/json"
	"fmt"
)

// Shared pagination constants used across handler files.
const (
	defaultLimit = 20
	maxLimit     = 100
)

// Helper methods

func (s *Server) successResponse(id, data any) *Response {
	result := map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": formatResult(data),
			},
		},
		"isError": false,
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to marshal result: %v", err))
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  json.RawMessage(resultJSON),
	}
}

func (s *Server) errorResponse(id any, code int, message string) *Response {
	// Sanitize internal error messages to avoid leaking service URLs, response bodies, etc.
	// InvalidParams messages are our own validation text and are safe to pass through.
	if code == InternalError {
		message = sanitizeErrorMessage(message)
	}
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &ErrorObject{
			Code:    code,
			Message: message,
		},
	}
}

func formatResult(data any) string {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", data)
	}
	return string(jsonData)
}
