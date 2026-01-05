package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/client"
	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/mcp"
)

func main() {
	// Read from stdin, write to stdout
	// IMPORTANT: Only JSON should go to stdout for MCP protocol
	// All errors/logs should go to stderr
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	// Initialize index-manager client
	indexManagerURL := os.Getenv("INDEX_MANAGER_URL")
	if indexManagerURL == "" {
		indexManagerURL = "http://localhost:8090"
	}

	indexClient := client.NewIndexManagerClient(indexManagerURL)

	// Create MCP server
	server := mcp.NewServer(indexClient)

	// Process requests
	// MCP protocol expects compact JSON (no indentation) for better compatibility
	decoder := json.NewDecoder(reader)
	encoder := json.NewEncoder(writer)
	// Don't use SetIndent - MCP clients expect compact JSON

	for {
		var request mcp.Request
		if err := decoder.Decode(&request); err != nil {
			if err == io.EOF {
				break
			}
			// For parse errors, we can't get the ID from the request, so use a default
			// JSON-RPC requires ID to be string or number, not null
			sendError(writer, encoder, 0, mcp.ParseError, "Failed to parse request", nil)
			continue
		}

		// Handle request
		// JSON-RPC notifications (requests without ID) don't require responses
		response := server.HandleRequest(&request)
		if response != nil {
			// Only send response if this was a request (has ID), not a notification
			// Preserve the original request ID exactly as sent
			if response.ID == nil && request.ID != nil {
				response.ID = request.ID
			}
			// Don't send response if request had no ID (notification)
			if request.ID == nil {
				continue
			}
			if encodeErr := encoder.Encode(response); encodeErr != nil {
				fmt.Fprintf(os.Stderr, "Failed to encode response: %v\n", encodeErr)
			}
		}
	}
}

func sendError(_ io.Writer, encoder *json.Encoder, id any, code int, message string, data any) {
	errorResponse := mcp.ErrorResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: mcp.ErrorObject{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	if encodeErr := encoder.Encode(errorResponse); encodeErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode error response: %v\n", encodeErr)
	}
}
