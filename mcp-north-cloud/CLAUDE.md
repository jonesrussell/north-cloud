# MCP North Cloud — Developer Guide

This file provides guidance to Claude Code (claude.ai/code) when working with the mcp-north-cloud service.

## Quick Reference

```bash
# Development
task dev              # Hot reload with Air
task test             # Run tests
task lint             # Run linter

# Verify tool registration
./test-tools.sh                   # local mode (expects 18 tools)
MCP_ENV=prod ./test-tools.sh      # prod mode (expects 24 tools)
MCP_TEST_PROMPTS=1 ./test-tools.sh  # also test prompts and resources

# Manual tool calls (binary must be built first: task build)
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | ./bin/mcp-north-cloud
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_sources","arguments":{}}}' | ./bin/mcp-north-cloud
```

## Architecture

**MCP Server** — stdio-based JSON-RPC 2.0 (no HTTP server).

```
mcp-north-cloud/
├── main.go                  # Stdio processing loop (reads stdin, writes stdout)
├── run-mcp.sh               # Wrapper: loads .env, ensures clean stdout
├── test-tools.sh            # Smoke test: verifies tool count by env
├── internal/
│   ├── mcp/
│   │   ├── server.go        # Request routing, toolHandlers map, prompts/resources dispatch
│   │   ├── types.go         # JSON-RPC types, Prompt/Resource types, Scope constants
│   │   ├── tools.go         # 27 tool definitions (scoped by MCP_ENV)
│   │   ├── handlers.go      # Tool implementations (one func per tool)
│   │   ├── prompts.go       # prompts/list and prompts/get (4 prompts)
│   │   ├── resources.go     # resources/list and resources/read (static docs)
│   │   └── scope_test.go    # Verifies tool counts per env
│   ├── client/              # HTTP clients, one file per service
│   │   ├── crawler.go
│   │   ├── publisher.go
│   │   ├── source_manager.go
│   │   ├── search.go
│   │   ├── classifier.go
│   │   └── index_manager.go
│   └── config/
└── bin/mcp-north-cloud      # Built binary (gitignored)
```

## Key Concepts

### Stdio-only JSON-RPC 2.0

The server has no HTTP port. The AI client (Cursor, Claude Code) starts the binary as a subprocess and communicates over stdin/stdout. The server runs until stdin closes (EOF = graceful shutdown).

### Stdout/Stderr Discipline (CRITICAL)

**Only JSON-RPC responses go to stdout. Everything else must go to stderr.**

```go
// CORRECT — log to stderr only
logger.OutputPaths = []string{"stderr"}

// WRONG — corrupts the JSON-RPC stream
fmt.Println("debug info")  // goes to stdout, breaks the protocol
```

The `run-mcp.sh` wrapper enforces this with `>&2` redirects for build output. Any stray bytes on stdout (debug prints, build logs, startup messages) will cause the client to fail parsing responses.

### MCP_ENV Tool Filtering

Set `MCP_ENV` to control which tools appear in `tools/list`:

| Environment | Count | Includes |
|-------------|-------|---------|
| `local` (default) | 18 | shared (15) + local-only (3) |
| `prod` | 24 | shared (15) + prod-only (9) |

**Shared (15):** onboard_source, list_crawl_jobs, get_crawl_stats, add_source, list_sources, update_source, test_source, list_indexes, search_articles, list_routes, list_channels, preview_route, get_publish_history, get_publisher_stats, classify_article

**Local-only (3):** lint_file, build_service, test_service

**Prod-only (9):** delete_index, delete_source, delete_route, control_crawl_job, start_crawl, schedule_crawl, create_route, create_channel, get_auth_token

### Scope Types

Defined as constants in `types.go`: `ScopeShared`, `ScopeLocal`, `ScopeProd`. Each tool has a `Scope` field set in `tools.go`. The `getToolsForEnv()` function in `tools.go` filters by calling `t.Scope.IsAllowed(env)`.

## API Reference (MCP Protocol)

The server implements these JSON-RPC 2.0 methods:

| Method | Description |
|--------|-------------|
| `initialize` | Returns protocol version `2024-11-05` and capabilities (tools, prompts, resources) |
| `tools/list` | Returns tools for current `MCP_ENV` (18 local / 24 prod) |
| `tools/call` | Routes `params.name` to the registered handler |
| `prompts/list` | Returns 4 prompt templates |
| `prompts/get` | Returns messages for a prompt with argument substitution |
| `resources/list` | Returns static doc resources under `northcloud://docs/*` |
| `resources/read` | Returns content for a given resource URI |
| `ping` | Keepalive; responds with empty result |

Requests without an `id` field are notifications and receive no response.

### JSON-RPC Error Codes

| Code | Meaning |
|------|---------|
| -32700 | Parse error (invalid JSON) |
| -32600 | Invalid request |
| -32601 | Method not found (unknown tool name) |
| -32602 | Invalid params |
| -32603 | Internal error |
| -32002 | Resource not found (unknown resource URI) |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MCP_ENV` | `local` | Tool scope: `local` (18 tools) or `prod` (24 tools) |
| `CRAWLER_URL` | `http://localhost:8060` | Crawler service URL |
| `SOURCE_MANAGER_URL` | `http://localhost:8050` | Source manager service URL |
| `PUBLISHER_URL` | `http://localhost:8070` | Publisher service URL |
| `CLASSIFIER_URL` | `http://localhost:8071` | Classifier service URL |
| `SEARCH_URL` | `http://localhost:8092` | Search service URL |
| `INDEX_MANAGER_URL` | `http://localhost:8090` | Index manager service URL |
| `AUTH_JWT_SECRET` | — | Shared JWT secret (required for protected tools) |
| `MCP_HTTP_TIMEOUT_SECONDS` | `30` | HTTP client timeout (request timeout in main loop is 60s) |
| `NORTH_CLOUD_ROOT` | cwd | Project root used by lint_file/build_service/test_service |

## Common Gotchas

1. **Stdout/stderr discipline** — Any non-JSON-RPC output on stdout corrupts the protocol. Never use `fmt.Println` or `log.Print` in server code; always configure loggers to write to stderr. The `run-mcp.sh` wrapper enforces clean stdout for build output but cannot catch runtime prints.

2. **No authentication** — The MCP server itself does not authenticate callers. Protected North Cloud tools rely on `AUTH_JWT_SECRET` for service-to-service JWT tokens. If `AUTH_JWT_SECRET` is missing, those tools fail with "missing authorization".

3. **Notifications have no ID** — JSON-RPC requests without an `id` field are notifications. The server must not send a response for them.

4. **EOF is graceful shutdown** — When stdin closes, the server exits cleanly. This is normal; do not treat it as an error.

5. **lint_file uses configurable project root** — Set via `NORTH_CLOUD_ROOT` env var or defaults to cwd. This must point at the repository root for service paths to resolve correctly.

6. **All build output to stderr** — The `run-mcp.sh` wrapper redirects build/compile output to stderr. If you run the binary directly without the wrapper, build messages may appear on stdout.

7. **Cursor requires restart** — After changing `.cursor/mcp.json` or rebuilding the binary, restart Cursor (or reload the window) for changes to take effect.

## Testing

```bash
# Verify tool registration counts
./test-tools.sh               # local mode, expects 18
MCP_ENV=prod ./test-tools.sh  # prod mode, expects 24

# Also exercise prompts and resources
MCP_TEST_PROMPTS=1 ./test-tools.sh

# Unit tests (includes scope_test.go verifying counts)
task test

# Manual JSON-RPC calls
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | ./bin/mcp-north-cloud
echo '{"jsonrpc":"2.0","id":2,"method":"prompts/list"}' | ./bin/mcp-north-cloud
echo '{"jsonrpc":"2.0","id":3,"method":"resources/list"}' | ./bin/mcp-north-cloud
echo '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_sources","arguments":{}}}' | ./bin/mcp-north-cloud
```

## Code Patterns

### Handler Pattern

All tool handlers are registered in `server.go` in the `toolHandlers` map and implemented in `handlers.go`. Every handler follows this structure:

```go
func (s *Server) handleToolName(ctx context.Context, id any, arguments json.RawMessage) *Response {
    // 1. Unmarshal arguments
    var args struct {
        RequiredField string `json:"required_field"`
        OptionalField string `json:"optional_field"`
    }
    if err := json.Unmarshal(arguments, &args); err != nil {
        return s.errorResponse(id, InvalidParams, "invalid arguments")
    }

    // 2. Validate required fields
    if args.RequiredField == "" {
        return s.errorResponse(id, InvalidParams, "required_field is required")
    }

    // 3. Call service client (always pass ctx)
    result, err := s.client.DoSomething(ctx, args.RequiredField)
    if err != nil {
        return s.errorResponse(id, InternalError, err.Error())
    }

    // 4. Return success
    return s.successResponse(id, result)
}
```

Registration in `server.go`:

```go
toolHandlers: map[string]toolHandlerFunc{
    "tool_name": (*Server).handleToolName,
    // ...
}
```

### Adding a New Tool (4-step workflow)

1. **Define the tool** in `internal/mcp/tools.go`: add to the appropriate `get*Tools()` slice, set `Name`, `Scope` (`ScopeShared`, `ScopeLocal`, or `ScopeProd`), `Description`, and `InputSchema`.

2. **Register the handler** in `server.go`: add `"tool_name": (*Server).handleToolName` to the `toolHandlers` map.

3. **Implement the handler** in `handlers.go`: signature must be `func (s *Server) handleToolName(ctx context.Context, id any, arguments json.RawMessage) *Response`. Pass `ctx` into all client calls.

4. **Update test fixtures**: update `scope_test.go` expected counts, `test-tools.sh` expected count, the README tools table, and optionally `internal/mcp/resources.go` static tool-reference text.

### Service Client Pattern

All HTTP clients live in `internal/client/`, one file per service. Each uses a configurable timeout:

```go
type Client struct {
    baseURL    string
    httpClient *http.Client  // timeout set from MCP_HTTP_TIMEOUT_SECONDS (default 30s)
}
```

Clients are constructed in `server.go` using URL and timeout from config. Always pass `ctx` through to `http.NewRequestWithContext` — do not use `http.NewRequest`.

### 27 Tools by Category

| Category | Tools |
|----------|-------|
| **Workflow (1)** | onboard_source |
| **Crawler (5)** | start_crawl, schedule_crawl, list_crawl_jobs, control_crawl_job, get_crawl_stats |
| **Source Manager (5)** | add_source, list_sources, update_source, delete_source, test_source |
| **Publisher (8)** | create_route, list_routes, create_channel, list_channels, delete_route, preview_route, get_publish_history, get_publisher_stats |
| **Search (1)** | search_articles |
| **Classifier (1)** | classify_article |
| **Index Manager (2)** | list_indexes, delete_index |
| **Auth (1)** | get_auth_token |
| **Development (3)** | lint_file, build_service, test_service |

## Cursor IDE Integration

For local binary, `.cursor/mcp.json` points at `run-mcp.sh` or the binary directly. For production Docker:

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

For remote production (laptop → prod server):

```json
{
  "mcpServers": {
    "North Cloud (Production)": {
      "command": "ssh",
      "args": ["jones@northcloud.biz", "docker exec -i north-cloud-mcp-north-cloud-1 /app/mcp-north-cloud"]
    }
  }
}
```

Adjust container name as needed (`docker ps` to confirm).
