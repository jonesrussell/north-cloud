# MCP Server Spec

Covers `mcp-north-cloud/`: the Claude Code / Cursor MCP server that exposes north-cloud pipeline operations as tools.

## File Map

| File | Purpose |
|------|---------|
| `mcp-north-cloud/main.go` | Stdio JSON-RPC 2.0 processing loop |
| `mcp-north-cloud/run-mcp.sh` | Wrapper: loads .env, ensures clean stdout |
| `mcp-north-cloud/test-tools.sh` | Smoke test: verifies tool count by env |
| `mcp-north-cloud/internal/mcp/server.go` | Request routing, toolHandlers map, prompts/resources dispatch |
| `mcp-north-cloud/internal/mcp/tools.go` | 26 tool definitions scoped by MCP_ENV |
| `mcp-north-cloud/internal/mcp/handlers.go` | Tool implementations (one func per tool) |
| `mcp-north-cloud/internal/mcp/types.go` | JSON-RPC types, Scope constants |
| `mcp-north-cloud/internal/mcp/prompts.go` | 4 prompt templates |
| `mcp-north-cloud/internal/mcp/resources.go` | Static doc resources |
| `mcp-north-cloud/internal/mcp/scope_test.go` | Verifies tool counts per env (must be updated when tools added/removed) |
| `mcp-north-cloud/internal/mcp/audit.go` | Audit logging for all tool calls |
| `mcp-north-cloud/internal/mcp/ratelimit.go` | Per-client rate limiting |
| `mcp-north-cloud/internal/mcp/errors.go` | Error sanitization (no internal path/stack leaks) |
| `mcp-north-cloud/internal/mcp/health.go` | Health check endpoints |
| `mcp-north-cloud/internal/client/` | HTTP clients, one file per service |
| `mcp-north-cloud/internal/config/` | Config struct with env tags |

## Interface / API

### JSON-RPC 2.0 Methods

| Method | Description |
|--------|-------------|
| `initialize` | Returns protocol version `2024-11-05` + capabilities |
| `tools/list` | Returns tools for current `MCP_ENV` (18 local / 23 prod) |
| `tools/call` | Routes `params.name` to registered handler |
| `prompts/list` | Returns 4 prompt templates |
| `prompts/get` | Returns messages for a named prompt |
| `resources/list` | Returns static doc resources under `northcloud://docs/*` |
| `resources/read` | Returns content for a given resource URI |
| `ping` | Keepalive; empty result |

### Tool Counts (update scope_test.go + test-tools.sh when changed)

| Environment | Count | Scope |
|-------------|-------|-------|
| `local` (default) | 18 | shared (15) + local-only (3) |
| `prod` | 23 | shared (15) + prod-only (8) |
| Total definitions | 26 | 15 shared + 3 local + 8 prod |

### Tools by Category

| Category | Tools |
|----------|-------|
| Workflow | onboard_source |
| Crawler (5) | start_crawl, schedule_crawl, list_crawl_jobs, control_crawl_job, get_crawl_stats |
| Source Manager (5) | add_source, list_sources, update_source, delete_source, test_source |
| Publisher (6) | create_channel, list_channels, delete_channel, preview_channel, get_publish_history, get_publisher_stats |
| Search | search_content |
| Classifier | classify_content |
| Index Manager (2) | list_indexes, delete_index |
| Auth | get_auth_token |
| Observability | get_grafana_alerts |
| Development (3) | lint_file, build_service, test_service |

## Data Flow

```
AI client (Claude Code / Cursor)
  → spawns binary as subprocess (stdio)
  → stdin: JSON-RPC request
  → server routes to handler
  → handler calls HTTP client for appropriate service
  → stdout: JSON-RPC response

Security hardening layer:
  → audit.go: logs all tool calls (tool name, args, caller, timestamp)
  → ratelimit.go: throttles excess requests per client
  → errors.go: sanitizes error responses (no internal paths, stack traces)
  → health.go: exposes health check for monitoring
```

## Config Vars

| Variable | Default | Description |
|----------|---------|-------------|
| `MCP_ENV` | `local` | Tool scope: `local` (18) or `prod` (23) |
| `CRAWLER_URL` | `http://localhost:8080` | Crawler service |
| `SOURCE_MANAGER_URL` | `http://localhost:8050` | Source manager |
| `PUBLISHER_URL` | `http://localhost:8070` | Publisher |
| `CLASSIFIER_URL` | `http://localhost:8071` | Classifier |
| `SEARCH_URL` | `http://localhost:8092` | Search service |
| `INDEX_MANAGER_URL` | `http://localhost:8090` | Index manager |
| `GRAFANA_URL` | `http://localhost:3000` | Grafana |
| `AUTH_JWT_SECRET` | — | Required for protected tools |
| `MCP_HTTP_TIMEOUT_SECONDS` | `30` | HTTP client timeout |
| `NORTH_CLOUD_ROOT` | cwd | Repo root for lint_file/build_service |

## Known Constraints

- **Stdout/stderr discipline (CRITICAL)**: Only JSON-RPC responses go to stdout. Any stray bytes (debug prints, build logs) corrupt the protocol. Loggers MUST write to stderr only.
- **Stdio-only**: No HTTP port. AI client starts binary as subprocess, communicates over stdin/stdout.
- **EOF = graceful shutdown**: When stdin closes the server exits cleanly. Not an error.
- **No authentication at MCP layer**: Callers are not authenticated by the server itself. Protected tools use `AUTH_JWT_SECRET` for service-to-service JWT tokens.
- **Scope counts are test fixtures**: `scope_test.go` and `test-tools.sh` hardcode expected tool counts. Update both whenever tools are added or removed.
- **Adding a tool (4-step workflow)**: (1) define in `tools.go`, (2) register handler in `server.go`, (3) implement in `handlers.go`, (4) update counts in `scope_test.go` + `test-tools.sh`.
