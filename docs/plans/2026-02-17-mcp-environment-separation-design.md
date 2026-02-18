# MCP Server Environment Separation Design

**Date**: 2026-02-17
**Status**: Approved

## Problem

The MCP server exposes all 27 tools in both local and production environments. Dev workflow tools (lint, build, test) shouldn't exist in prod. Destructive/operational tools (delete_index, start_crawl, control_crawl_job) shouldn't be available in local dev where they could accidentally hit production, and crawl-triggering tools make no sense locally.

## Design

One codebase, one binary, environment-selected tool registry. At startup, `MCP_ENV` (default: `local`) determines which tools are registered.

### Tool Split

**Shared (15 tools) — both environments, read-only/non-destructive:**
- `list_sources`, `add_source`, `update_source`, `test_source`
- `list_indexes`
- `search_articles`
- `list_crawl_jobs`, `get_crawl_stats`
- `list_routes`, `list_channels`, `preview_route`
- `get_publish_history`, `get_publisher_stats`
- `classify_article`
- `onboard_source`

**Local-only (3 tools) — dev workflow:**
- `lint_file`
- `build_service`
- `test_service`

**Prod-only (9 tools) — destructive/operational:**
- `delete_index`, `delete_source`, `delete_route`
- `control_crawl_job` (pause/resume/cancel)
- `start_crawl`, `schedule_crawl`
- `create_route`, `create_channel`
- `get_auth_token`

### Principles
- If it can't mutate state, it's shared
- If it touches the dev build pipeline, it stays local
- If it can break prod or trigger real-world side effects, it's prod-only

### Implementation

**Scope metadata on tool definitions:**
```go
const (
    ScopeShared = "shared"
    ScopeLocal  = "local"
    ScopeProd   = "prod"
)
```

Each tool in `tools.go` gets a `Scope` field. At startup, the server filters the tool registry: shared always loads, plus local or prod based on `MCP_ENV`.

**No filesystem reorganization** — tools.go and handlers.go stay flat. Scope is metadata, not a directory convention.

**No external config files** — the tool set is known at compile time. Code-level registry is simpler and type-safe.

### Configuration

**.mcp.json / .cursor/mcp.json:**
- Local entry gets `"MCP_ENV": "local"` in env
- Production entry passes `-e MCP_ENV=prod` to docker exec

**docker-compose.prod.yml:**
- MCP service gets `MCP_ENV: prod` in environment block

### Logging

At startup, log environment and tool count:
```json
{"level":"info","msg":"MCP server started","env":"local","tools":18,"filtered_out":9}
```

### Out of Scope
- Safety confirmation wrappers (Claude Code handles tool approval)
- External config files
- New tools (add to the right scope when built)
- Filesystem reorganization of tools/handlers
