# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with the mcp-north-cloud service.

## Quick Reference

```bash
# Development
task dev              # Start with hot reload
task test             # Run tests
task lint             # Run linter
./test-tools.sh       # Verify tool registration (18 local / 24 prod)

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
│   │   ├── server.go    # Request routing, prompts/resources handlers
│   │   ├── types.go     # JSON-RPC types, Prompt/Resource types
│   │   ├── tools.go     # 27 tool definitions (filtered by MCP_ENV)
│   │   ├── handlers.go  # Tool implementations
│   │   ├── prompts.go  # prompts/list, prompts/get (4 prompts)
│   │   └── resources.go # resources/list, resources/read (static docs)
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

## Environment-Based Tool Registry

Set `MCP_ENV` to control which tools are available:

| Environment | Tools | Count |
|-------------|-------|-------|
| `local` (default) | shared + local-only | 18 |
| `prod` | shared + prod-only | 24 |

**Shared (15):** list_sources, add_source, update_source, test_source, list_indexes, search_articles, list_crawl_jobs, get_crawl_stats, list_routes, list_channels, preview_route, get_publish_history, get_publisher_stats, classify_article, onboard_source

**Local-only (3):** lint_file, build_service, test_service

**Prod-only (9):** delete_index, delete_source, delete_route, control_crawl_job, start_crawl, schedule_crawl, create_route, create_channel, get_auth_token

## 27 Tools by Category

| Category | Tools |
|----------|-------|
| **Auth (1)** | get_auth_token |
| **Workflow (1)** | onboard_source |
| **Crawler (5)** | start_crawl, schedule_crawl, list_crawl_jobs, control_crawl_job, get_crawl_stats |
| **Source Manager (5)** | add_source, list_sources, update_source, delete_source, test_source |
| **Publisher (8)** | create_route, list_routes, create_channel, list_channels, delete_route, preview_route, get_publish_history, get_publisher_stats |
| **Search (1)** | search_articles |
| **Classifier (1)** | classify_article |
| **Index Manager (2)** | list_indexes, delete_index |
| **Development (3)** | lint_file, build_service, test_service |

## MCP Protocol Methods

- `initialize` - Returns protocol version & capabilities (tools, prompts, resources)
- `tools/list` - Returns tools for current MCP_ENV (18 local / 24 prod)
- `tools/call` - Routes to specific handler
- `prompts/list` - Returns prompt templates (e.g. onboard_new_source, debug_crawl_job)
- `prompts/get` - Returns messages for a prompt with argument substitution
- `resources/list` - Returns static doc resources (northcloud://docs/*)
- `resources/read` - Returns content for a resource URI
- `ping` - Keepalive

## Handler Pattern

All tool handlers take context and follow this structure (handlers are registered in `server.go` in `toolHandlers` map):

```go
func (s *Server) handleToolName(ctx context.Context, id any, arguments json.RawMessage) *Response {
    // 1. Unmarshal arguments
    var args struct { ... }
    if err := json.Unmarshal(arguments, &args); err != nil {
        return s.errorResponse(id, InvalidParams, "message")
    }

    // 2. Validate required fields
    if args.RequiredField == "" {
        return s.errorResponse(id, InvalidParams, "field is required")
    }

    // 3. Call service client (pass ctx)
    result, err := s.client.DoSomething(ctx, args)
    if err != nil {
        return s.errorResponse(id, InternalError, err.Error())
    }

    // 4. Return success
    return s.successResponse(id, result)
}
```

## Adding a New Tool

1. **Define tool** in `internal/mcp/tools.go` (add to `getAllTools()` slice). Set `Scope: ScopeShared`, `ScopeLocal`, or `ScopeProd`.

2. **Register handler** in `server.go`: add to `toolHandlers` map, e.g. `"tool_name": (*Server).handleToolName`.

3. **Implement handler** in `handlers.go` with signature `(ctx context.Context, id any, arguments json.RawMessage) *Response`; pass `ctx` into client calls.

4. **Update** `scope_test.go` expected counts, `test-tools.sh`, README tool list, and this CLAUDE.md table. Optionally update `internal/mcp/resources.go` static tool-reference text.

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
- `PUBLISHER_URL` → `http://localhost:8070`
- `SEARCH_URL` → `http://localhost:8090`
- `CLASSIFIER_URL` → `http://localhost:8070`
- `INDEX_MANAGER_URL` → `http://localhost:8090`
- `MCP_HTTP_TIMEOUT_SECONDS` → `30` (HTTP client timeout; request timeout in main is 60s)

## Common Gotchas

1. **No authentication**: MCP server doesn't authenticate - relies on network isolation.

2. **Notifications have no ID**: Requests without `id` field don't get responses.

3. **EOF is graceful shutdown**: When stdin closes, server exits cleanly.

4. **lint_file uses configurable project root**: Set via `NORTH_CLOUD_ROOT` env var or defaults to current directory.

5. **All build output to stderr**: The `run-mcp.sh` wrapper ensures clean stdout.

## Cursor IDE Integration

For local binary, config in `.cursor/mcp.json` points at `run-mcp.sh` or the binary. For MCP running in production Docker, use `docker exec -i` (see README "Deploying MCP in production"):
```json
{
  "mcpServers": {
    "north-cloud": {
      "command": "docker",
      "args": ["exec", "-i", "north-cloud-mcp-north-cloud-1", "/app/mcp-north-cloud"]
    }
  }
}
```
Adjust container name if different (`docker ps`).

## JSON-RPC Error Codes

| Code | Meaning |
|------|---------|
| -32700 | Parse error (invalid JSON) |
| -32600 | Invalid request |
| -32601 | Method not found (unknown tool name) |
| -32602 | Invalid params |
| -32603 | Internal error |
| -32002 | Resource not found (unknown resource URI) |

## Testing

```bash
# Verify tools registered (default: local=18)
./test-tools.sh

# Verify prod tools (24)
MCP_ENV=prod ./test-tools.sh

# Optional: test prompts and resources (set MCP_TEST_PROMPTS=1)
MCP_TEST_PROMPTS=1 ./test-tools.sh

# Manual tool call
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_sources","arguments":{}}}' | ./bin/mcp-north-cloud
```
