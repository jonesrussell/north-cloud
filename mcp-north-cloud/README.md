# MCP North Cloud

> Model Context Protocol server exposing North Cloud tools to AI assistants (Claude, Cursor, etc.)

## Overview

This MCP server is a unified interface to the entire North Cloud microservices platform. It uses **stdio-based JSON-RPC 2.0** — the server reads from stdin and writes to stdout with no HTTP port. AI clients (Cursor, Claude Code, Claude Desktop) start the process and communicate over its stdio pipes.

The server exposes 27 tools across crawling, source management, classification, publishing, search, index management, and local development. Tool availability is filtered by `MCP_ENV`: 18 tools in local mode, 24 in production mode.

## Features

- **27 tools** across 9 categories (18 in `local` mode, 24 in `prod` mode)
- Environment-based tool filtering via `MCP_ENV=local|prod`
- Prompts and resources for guided workflows
- Service-to-service JWT auth (no user-facing auth required)

## Tools

### By Category

| Category | Count | Tools |
|----------|-------|-------|
| Workflow | 1 | onboard_source |
| Crawler | 5 | start_crawl, schedule_crawl, list_crawl_jobs, control_crawl_job, get_crawl_stats |
| Source Manager | 5 | add_source, list_sources, update_source, delete_source, test_source |
| Publisher | 8 | create_route, list_routes, create_channel, list_channels, delete_route, preview_route, get_publish_history, get_publisher_stats |
| Search | 1 | search_articles |
| Classifier | 1 | classify_article |
| Index Manager | 2 | delete_index, list_indexes |
| Auth | 1 | get_auth_token |
| Development | 3 | lint_file, build_service, test_service |

### Environment Scope

Tools are scoped to control which environment they appear in:

| Scope | Tools | Available in |
|-------|-------|-------------|
| Shared (15) | onboard_source, list_crawl_jobs, get_crawl_stats, add_source, list_sources, update_source, test_source, list_routes, list_channels, preview_route, get_publish_history, get_publisher_stats, search_articles, classify_article, list_indexes | local + prod |
| Local-only (3) | lint_file, build_service, test_service | local only |
| Prod-only (9) | start_crawl, schedule_crawl, control_crawl_job, delete_source, create_route, create_channel, delete_route, delete_index, get_auth_token | prod only |

Set `MCP_ENV=prod` to get the prod tool set (default is `local`).

## Quick Start

### Build the binary

```bash
# From repository root (preferred)
task mcp:build

# Or from inside mcp-north-cloud/
task build
# or: go build -o bin/mcp-north-cloud ./main.go
```

This writes `mcp-north-cloud/bin/mcp-north-cloud`. Re-run after changing MCP server code.

### Claude Code / Claude Desktop

This repo includes `.mcp.json` at the root. It points at the binary and loads environment from `.env`. Ensure the binary is built and `.env` contains `AUTH_JWT_SECRET`.

```json
{
  "mcpServers": {
    "north-cloud": {
      "command": "/path/to/north-cloud/mcp-north-cloud/run-mcp.sh"
    }
  }
}
```

The `run-mcp.sh` wrapper loads `.env` and starts the binary with localhost service URLs matching `docker-compose.dev.yml`.

### Cursor IDE

Cursor reads `.cursor/mcp.json` in the repo root. It runs `run-mcp.sh`, which loads `.env` and starts the MCP binary. After changing `.cursor/mcp.json` or rebuilding the binary, **restart Cursor** to apply changes.

### Production (Docker exec)

When Cursor runs on the same host as the production Docker stack:

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

When Cursor runs on a different host (e.g., your laptop connecting to the prod server):

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

Use `docker ps` to confirm the exact container name.

## Configuration

### Environment Variables

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
| `MCP_HTTP_TIMEOUT_SECONDS` | `30` | HTTP client timeout for backend calls |
| `NORTH_CLOUD_ROOT` | cwd | Project root used by lint_file/build_service/test_service |

### Authentication

Tools that call protected APIs require JWT authentication. The server generates service-to-service tokens using `AUTH_JWT_SECRET`. Load it from `.env` (the `run-mcp.sh` wrapper does this automatically).

Without `AUTH_JWT_SECRET`, protected tools (e.g., `onboard_source`, `add_source`, `create_route`) will fail with "missing authorization".

### Pointing at production backends

Set each `*_URL` variable to your production service URLs and set `AUTH_JWT_SECRET` to match the production auth service. Example `.cursor/mcp.json` snippet:

```json
{
  "mcpServers": {
    "north-cloud": {
      "command": "/path/to/mcp-north-cloud/bin/mcp-north-cloud",
      "env": {
        "MCP_ENV": "prod",
        "CRAWLER_URL": "https://crawler.example.com",
        "SOURCE_MANAGER_URL": "https://source-manager.example.com",
        "PUBLISHER_URL": "https://publisher.example.com",
        "CLASSIFIER_URL": "https://classifier.example.com",
        "SEARCH_URL": "https://search.example.com",
        "INDEX_MANAGER_URL": "https://index-manager.example.com",
        "AUTH_JWT_SECRET": "<same-as-production-auth>"
      }
    }
  }
}
```

## Prompts

The server supports `prompts/list` and `prompts/get`. Clients can expose these as slash commands.

| Name | Description |
|------|-------------|
| `onboard_new_source` | Add a new website/source and optionally start crawling and create a publishing route |
| `debug_crawl_job` | Inspect a crawl job: status, stats, and suggestions |
| `publishing_review` | Preview a route and review publish history and stats |
| `classify_and_search` | Search classified content and optionally classify a sample |

## Resources

The server supports `resources/list` and `resources/read`. Static documentation is available under the `northcloud://` URI scheme:

| URI | Description |
|-----|-------------|
| `northcloud://docs/tool-reference` | List of MCP tools and when to use them |
| `northcloud://docs/selectors` | CSS selectors cheatsheet for source extraction |
| `northcloud://docs/pipeline` | Crawl → Classify → Publish flow overview |

## Architecture

```
mcp-north-cloud/
├── main.go                  # Stdio processing loop (reads stdin, writes stdout)
├── run-mcp.sh               # Wrapper: loads .env, ensures clean stdout
├── test-tools.sh            # Smoke test: verifies tool count by env
├── internal/
│   ├── mcp/
│   │   ├── server.go        # Request routing, toolHandlers map
│   │   ├── types.go         # JSON-RPC types, Prompt/Resource types
│   │   ├── tools.go         # 27 tool definitions (scoped by MCP_ENV)
│   │   ├── handlers.go      # Tool implementations
│   │   ├── prompts.go       # prompts/list and prompts/get (4 prompts)
│   │   └── resources.go     # resources/list and resources/read (static docs)
│   ├── client/              # HTTP clients per service
│   │   ├── crawler.go
│   │   ├── publisher.go
│   │   ├── source_manager.go
│   │   ├── search.go
│   │   ├── classifier.go
│   │   └── index_manager.go
│   └── config/
└── bin/mcp-north-cloud      # Built binary (gitignored)
```

## Development

```bash
# Hot reload
task dev

# Tests
task test

# Lint
task lint

# Verify tool registration
./test-tools.sh             # local mode (expects 18)
MCP_ENV=prod ./test-tools.sh  # prod mode (expects 24)
MCP_TEST_PROMPTS=1 ./test-tools.sh  # also test prompts and resources

# Manual tool call
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | ./bin/mcp-north-cloud
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_sources","arguments":{}}}' | ./bin/mcp-north-cloud
```

### When adding a tool

1. Define the tool in `internal/mcp/tools.go` (add to the appropriate `get*Tools()` function, set `Scope`)
2. Register the handler in `server.go` `toolHandlers` map
3. Implement the handler in `handlers.go`
4. Update `scope_test.go` expected counts, `test-tools.sh`, and this README tool table

See CLAUDE.md for the full handler pattern and service client pattern.

## Troubleshooting

**MCP works locally but not against production**
- Confirm each `*_URL` is reachable from your machine (VPN or public HTTPS)
- `AUTH_JWT_SECRET` must match production exactly; mismatches cause 401 errors

**Server not responding**
```bash
# Check all North Cloud services are running
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml ps

# Verify health endpoints
curl http://localhost:8060/health  # crawler
curl http://localhost:8050/health  # source-manager
curl http://localhost:8070/health  # publisher

# Check MCP server logs
docker logs north-cloud-mcp-north-cloud-1
```

**Cursor not detecting the MCP server**
1. Verify `.cursor/mcp.json` exists in the project root
2. Ensure the binary is built: `task mcp:build`
3. Restart Cursor (or reload the window) after any config change
