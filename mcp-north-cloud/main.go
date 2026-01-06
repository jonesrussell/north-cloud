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

	// Initialize service clients
	clients := initializeClients()

	// Create MCP server
	server := mcp.NewServer(
		clients.indexManager,
		clients.crawler,
		clients.sourceManager,
		clients.publisher,
		clients.search,
		clients.classifier,
	)

	// Process requests
	processRequests(reader, writer, server)
}

type serviceClients struct {
	indexManager *client.IndexManagerClient
	crawler      *client.CrawlerClient
	sourceManager *client.SourceManagerClient
	publisher    *client.PublisherClient
	search       *client.SearchClient
	classifier   *client.ClassifierClient
}

func initializeClients() *serviceClients {
	getURL := func(envVar, defaultURL string) string {
		if url := os.Getenv(envVar); url != "" {
			return url
		}
		return defaultURL
	}

	return &serviceClients{
		indexManager:  client.NewIndexManagerClient(getURL("INDEX_MANAGER_URL", "http://localhost:8090")),
		crawler:       client.NewCrawlerClient(getURL("CRAWLER_URL", "http://localhost:8060")),
		sourceManager: client.NewSourceManagerClient(getURL("SOURCE_MANAGER_URL", "http://localhost:8050")),
		publisher:     client.NewPublisherClient(getURL("PUBLISHER_URL", "http://localhost:8080")),
		search:        client.NewSearchClient(getURL("SEARCH_URL", "http://localhost:8090")),
		classifier:    client.NewClassifierClient(getURL("CLASSIFIER_URL", "http://localhost:8070")),
	}
}

func processRequests(reader *bufio.Reader, writer io.Writer, server *mcp.Server) {
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
