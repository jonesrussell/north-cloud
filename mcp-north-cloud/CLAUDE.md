# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with the mcp-north-cloud service.

## Quick Reference

```bash
# Development
task dev              # Start with hot reload
task test             # Run tests
task lint             # Run linter
./test-tools.sh       # Verify tool registration (23 tools)

# Test tool execution
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | ./bin/mcp-north-cloud
```

## Architecture

**MCP Server** - stdio-based JSON-RPC 2.0 (no HTTP server)

```
mcp-north-cloud/
├── main.go              # Stdio processing loop
├── internal/
│   ├── mcp/
│   │   ├── server.go    # Request routing
│   │   ├── types.go     # JSON-RPC types
│   │   ├── tools.go     # 23 tool definitions
│   │   └── handlers.go  # Tool implementations
│   ├── client/          # HTTP clients for all services
│   │   ├── crawler.go
│   │   ├── publisher.go
│   │   ├── source_manager.go
│   │   ├── search.go
│   │   ├── classifier.go
│   │   └── index_manager.go
│   └── config/
└── run-mcp.sh          # Build wrapper (ensures stdout clean)
```

## Critical: Stdout/Stderr Discipline

**Only JSON-RPC goes to stdout. Everything else to stderr.**

```go
// CORRECT - logging to stderr
logger.OutputPaths = []string{"stderr"}

// WRONG - will break MCP protocol
fmt.Println("debug info")  // Goes to stdout, corrupts JSON-RPC
```

The `run-mcp.sh` wrapper enforces this with `>&2` redirects.

## 23 Tools by Category

| Category | Tools |
|----------|-------|
| **Crawler (7)** | start_crawl, schedule_crawl, list_crawl_jobs, pause_crawl_job, resume_crawl_job, cancel_crawl_job, get_crawl_stats |
| **Source Manager (5)** | add_source, list_sources, update_source, delete_source, test_source |
| **Publisher (6)** | create_route, list_routes, delete_route, preview_route, get_publish_history, get_publisher_stats |
| **Search (1)** | search_articles |
| **Classifier (1)** | classify_article |
| **Index Manager (2)** | list_indexes, delete_index |
| **Development (1)** | lint_file |

## MCP Protocol Methods

- `initialize` - Returns protocol version & capabilities
- `tools/list` - Returns all 23 tool definitions
- `tools/call` - Routes to specific handler
- `ping` - Keepalive

## Handler Pattern

All tool handlers follow this structure:

```go
func (s *Server) handleToolName(id any, arguments json.RawMessage) *Response {
    // 1. Unmarshal arguments
    var args struct { ... }
    if err := json.Unmarshal(arguments, &args); err != nil {
        return s.errorResponse(id, InvalidParams, "message")
    }

    // 2. Validate required fields
    if args.RequiredField == "" {
        return s.errorResponse(id, InvalidParams, "field is required")
    }

    // 3. Call service client
    result, err := s.client.DoSomething(args)
    if err != nil {
        return s.errorResponse(id, InternalError, err.Error())
    }

    // 4. Return success
    return s.successResponse(id, result)
}
```

## Adding a New Tool

1. **Define tool** in `internal/mcp/tools.go`:
```go
{
    Name:        "tool_name",
    Description: "What the tool does",
    InputSchema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "param": map[string]any{"type": "string", "description": "..."},
        },
        "required": []string{"param"},
    },
}
```

2. **Add routing** in `server.go:routeToolCall()`:
```go
case "tool_name":
    return s.handleToolName(id, arguments)
```

3. **Implement handler** in `handlers.go`

4. **Update test** in `test-tools.sh` (tool count)

## Service Client Pattern

All HTTP clients use 30-second timeout:

```go
type Client struct {
    baseURL    string
    httpClient *http.Client  // 30s timeout
}
```

**Configuration** (env vars with defaults):
- `CRAWLER_URL` → `http://localhost:8060`
- `SOURCE_MANAGER_URL` → `http://localhost:8050`
- `PUBLISHER_URL` → `http://localhost:8080`
- `SEARCH_URL` → `http://localhost:8090`
- `CLASSIFIER_URL` → `http://localhost:8070`
- `INDEX_MANAGER_URL` → `http://localhost:8090`

## Common Gotchas

1. **No authentication**: MCP server doesn't authenticate - relies on network isolation.

2. **Notifications have no ID**: Requests without `id` field don't get responses.

3. **EOF is graceful shutdown**: When stdin closes, server exits cleanly.

4. **lint_file uses configurable project root**: Set via `NORTH_CLOUD_ROOT` env var or defaults to current directory.

5. **All build output to stderr**: The `run-mcp.sh` wrapper ensures clean stdout.

## Cursor IDE Integration

Config in `.cursor/mcp.json`:
```json
{
  "mcpServers": {
    "north-cloud": {
      "command": "docker",
      "args": ["exec", "-i", "north-cloud-mcp-north-cloud-1", "/app/tmp/mcp-north-cloud"]
    }
  }
}
```

Requires container running with binary at `/app/tmp/mcp-north-cloud`.

## JSON-RPC Error Codes

| Code | Meaning |
|------|---------|
| -32700 | Parse error (invalid JSON) |
| -32600 | Invalid request |
| -32601 | Method not found |
| -32602 | Invalid params |
| -32603 | Internal error |

## Testing

```bash
# Verify 23 tools registered
./test-tools.sh

# Manual tool call
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_sources","arguments":{}}}' | ./bin/mcp-north-cloud
```
