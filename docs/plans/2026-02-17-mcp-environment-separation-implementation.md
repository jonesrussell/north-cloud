# MCP Environment Separation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `MCP_ENV` (local/prod) to the MCP server so each environment gets only its relevant tools: shared (15), local-only (3), or prod-only (9).

**Architecture:** Add a `Scope` field to `Tool` type. Each tool definition gets tagged `shared`, `local`, or `prod`. `getAllTools()` accepts the environment and filters. Config reads `MCP_ENV` env var. Startup logs the active environment and tool counts.

**Tech Stack:** Go 1.25, existing mcp-north-cloud codebase

---

### Task 1: Add Scope Type and Tool Filtering

**Files:**
- Modify: `mcp-north-cloud/internal/mcp/types.go`
- Test: `mcp-north-cloud/internal/mcp/server_test.go`

**Step 1: Write the failing test**

Add to `mcp-north-cloud/internal/mcp/server_test.go` (or create `mcp-north-cloud/internal/mcp/scope_test.go`):

```go
package mcp

import "testing"

func TestToolScope_IsAllowed(t *testing.T) {
	t.Helper()
	tests := []struct {
		name    string
		scope   ToolScope
		env     string
		allowed bool
	}{
		{"shared in local", ScopeShared, EnvLocal, true},
		{"shared in prod", ScopeShared, EnvProd, true},
		{"local in local", ScopeLocal, EnvLocal, true},
		{"local in prod", ScopeLocal, EnvProd, false},
		{"prod in prod", ScopeProd, EnvProd, true},
		{"prod in local", ScopeProd, EnvLocal, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.scope.IsAllowed(tt.env); got != tt.allowed {
				t.Errorf("ToolScope(%q).IsAllowed(%q) = %v, want %v", tt.scope, tt.env, got, tt.allowed)
			}
		})
	}
}

func TestGetToolsForEnv_Counts(t *testing.T) {
	t.Helper()
	localTools := getToolsForEnv(EnvLocal)
	prodTools := getToolsForEnv(EnvProd)

	// Local = 15 shared + 3 local = 18
	expectedLocal := 18
	if len(localTools) != expectedLocal {
		t.Errorf("local tools = %d, want %d", len(localTools), expectedLocal)
	}

	// Prod = 15 shared + 9 prod = 24
	expectedProd := 24
	if len(prodTools) != expectedProd {
		t.Errorf("prod tools = %d, want %d", len(prodTools), expectedProd)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/jones/dev/north-cloud/mcp-north-cloud && GOWORK=off go test ./internal/mcp/ -run TestToolScope -v`
Expected: FAIL (ToolScope not defined)

**Step 3: Add scope types to types.go**

Add to `mcp-north-cloud/internal/mcp/types.go` after the `Tool` struct:

```go
// ToolScope controls which environment a tool is available in.
type ToolScope string

const (
	// ScopeShared tools are available in both local and production.
	ScopeShared ToolScope = "shared"
	// ScopeLocal tools are only available in local development.
	ScopeLocal ToolScope = "local"
	// ScopeProd tools are only available in production.
	ScopeProd ToolScope = "prod"
)

// Environment constants.
const (
	EnvLocal = "local"
	EnvProd  = "prod"
)

// IsAllowed returns true if this scope is permitted in the given environment.
func (s ToolScope) IsAllowed(env string) bool {
	if s == ScopeShared {
		return true
	}
	return string(s) == env
}
```

Also add `Scope` field to the `Tool` struct (the field is excluded from JSON since the MCP protocol doesn't need it):

```go
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Scope       ToolScope      `json:"-"`
}
```

**Step 4: Run test to verify IsAllowed passes**

Run: `cd /home/jones/dev/north-cloud/mcp-north-cloud && GOWORK=off go test ./internal/mcp/ -run TestToolScope_IsAllowed -v`
Expected: PASS

**Step 5: Commit**

```bash
git add mcp-north-cloud/internal/mcp/types.go mcp-north-cloud/internal/mcp/scope_test.go
git commit -m "feat(mcp): add ToolScope type with IsAllowed method"
```

---

### Task 2: Tag All 27 Tools with Scope

**Files:**
- Modify: `mcp-north-cloud/internal/mcp/tools.go`

**Step 1: Add Scope field to every tool definition**

In `mcp-north-cloud/internal/mcp/tools.go`, add `Scope: ScopeXxx` to each tool. The complete mapping:

**Shared (15):**
- `onboard_source`: `ScopeShared`
- `list_crawl_jobs`: `ScopeShared`
- `get_crawl_stats`: `ScopeShared`
- `add_source`: `ScopeShared`
- `list_sources`: `ScopeShared`
- `update_source`: `ScopeShared`
- `test_source`: `ScopeShared`
- `list_routes`: `ScopeShared`
- `list_channels`: `ScopeShared`
- `preview_route`: `ScopeShared`
- `get_publish_history`: `ScopeShared`
- `get_publisher_stats`: `ScopeShared`
- `search_articles`: `ScopeShared`
- `classify_article`: `ScopeShared`
- `list_indexes`: `ScopeShared`

**Local-only (3):**
- `lint_file`: `ScopeLocal`
- `build_service`: `ScopeLocal`
- `test_service`: `ScopeLocal`

**Prod-only (9):**
- `start_crawl`: `ScopeProd`
- `schedule_crawl`: `ScopeProd`
- `control_crawl_job`: `ScopeProd`
- `delete_source`: `ScopeProd`
- `create_route`: `ScopeProd`
- `create_channel`: `ScopeProd`
- `delete_route`: `ScopeProd`
- `delete_index`: `ScopeProd`
- `get_auth_token`: `ScopeProd`

Example for one tool (apply this pattern to all 27):

```go
{
    Name:  "onboard_source",
    Scope: ScopeShared,
    Description: "Set up a complete content pipeline...",
    InputSchema: map[string]any{...},
},
```

**Step 2: Add `getToolsForEnv` function**

Replace `getAllTools()` with an env-aware version. Keep `getAllTools()` as internal, add `getToolsForEnv()`:

```go
// getToolsForEnv returns tools filtered by environment scope.
func getToolsForEnv(env string) []Tool {
	all := getAllTools()
	filtered := make([]Tool, 0, len(all))
	for _, t := range all {
		if t.Scope.IsAllowed(env) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
```

**Step 3: Run scope count test**

Run: `cd /home/jones/dev/north-cloud/mcp-north-cloud && GOWORK=off go test ./internal/mcp/ -run TestGetToolsForEnv_Counts -v`
Expected: PASS (18 local, 24 prod)

**Step 4: Run linter**

Run: `cd /home/jones/dev/north-cloud/mcp-north-cloud && GOWORK=off golangci-lint run`
Expected: No new errors

**Step 5: Commit**

```bash
git add mcp-north-cloud/internal/mcp/tools.go
git commit -m "feat(mcp): tag all 27 tools with environment scope"
```

---

### Task 3: Wire Environment Into Server and Config

**Files:**
- Modify: `mcp-north-cloud/internal/config/config.go`
- Modify: `mcp-north-cloud/internal/mcp/server.go`
- Modify: `mcp-north-cloud/main.go`

**Step 1: Add `MCP_ENV` to config**

In `mcp-north-cloud/internal/config/config.go`, add to `Config` struct:

```go
type Config struct {
	Env      string         `env:"MCP_ENV" yaml:"env"`
	Services ServicesConfig `yaml:"services"`
	Client   ClientConfig   `yaml:"client"`
	Logging  LoggingConfig  `yaml:"logging"`
	Auth     AuthConfig     `yaml:"auth"`
}
```

Add default in `setDefaults`:

```go
func setDefaults(cfg *Config) {
	if cfg.Env == "" {
		cfg.Env = "local"
	}
	// ... existing defaults
}
```

**Step 2: Pass environment into Server**

In `mcp-north-cloud/internal/mcp/server.go`, add `env` field to `Server`:

```go
type Server struct {
	env              string
	indexClient      *client.IndexManagerClient
	// ... rest unchanged
}
```

Update `NewServer` to accept env as first param:

```go
func NewServer(
	env string,
	indexClient *client.IndexManagerClient,
	crawlerClient *client.CrawlerClient,
	sourceClient *client.SourceManagerClient,
	publisherClient *client.PublisherClient,
	searchClient *client.SearchClient,
	classifierClient *client.ClassifierClient,
	authClient *client.AuthenticatedClient,
) *Server {
	return &Server{
		env:              env,
		indexClient:      indexClient,
		// ... rest unchanged
	}
}
```

**Step 3: Use `getToolsForEnv` in `handleToolsList`**

In `mcp-north-cloud/internal/mcp/server.go`, change `handleToolsList`:

```go
func (s *Server) handleToolsList(_ *Request, id any) *Response {
	tools := getToolsForEnv(s.env)
	// ... rest unchanged
}
```

**Step 4: Guard `routeToolCall` against filtered-out tools**

In `mcp-north-cloud/internal/mcp/server.go`, update `routeToolCall` to check scope before dispatching. Add a `toolScopes` map built from `getAllTools()`:

```go
// toolScopeMap builds a name->scope lookup from all tools.
func toolScopeMap() map[string]ToolScope {
	all := getAllTools()
	m := make(map[string]ToolScope, len(all))
	for _, t := range all {
		m[t.Name] = t.Scope
	}
	return m
}

var toolScopes = toolScopeMap()

func (s *Server) routeToolCall(ctx context.Context, id any, toolName string, arguments json.RawMessage) *Response {
	// Check if tool exists and is allowed in this environment
	scope, exists := toolScopes[toolName]
	if !exists {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &ErrorObject{Code: MethodNotFound, Message: "Unknown tool: " + toolName},
		}
	}
	if !scope.IsAllowed(s.env) {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &ErrorObject{Code: MethodNotFound, Message: "Tool not available in " + s.env + " environment: " + toolName},
		}
	}

	if h, ok := toolHandlers[toolName]; ok {
		return h(s, ctx, id, arguments)
	}
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &ErrorObject{Code: MethodNotFound, Message: "Unknown tool: " + toolName},
	}
}
```

**Step 5: Update main.go to pass env**

In `mcp-north-cloud/main.go`, update the `NewServer` call and add startup logging:

```go
	log.Info("Starting MCP server",
		logger.String("env", cfg.Env),
	)

	// ... after clients init ...

	server := mcp.NewServer(
		cfg.Env,
		clients.indexManager,
		clients.crawler,
		clients.sourceManager,
		clients.publisher,
		clients.search,
		clients.classifier,
		clients.authClient,
	)
```

**Step 6: Run all tests**

Run: `cd /home/jones/dev/north-cloud/mcp-north-cloud && GOWORK=off go test ./... -v`
Expected: PASS

**Step 7: Run linter**

Run: `cd /home/jones/dev/north-cloud/mcp-north-cloud && GOWORK=off golangci-lint run`
Expected: No errors

**Step 8: Commit**

```bash
git add mcp-north-cloud/internal/config/config.go mcp-north-cloud/internal/mcp/server.go mcp-north-cloud/main.go
git commit -m "feat(mcp): wire MCP_ENV into server, filter tools by environment"
```

---

### Task 4: Update .mcp.json and Docker Compose

**Files:**
- Modify: `.mcp.json`
- Modify: `.cursor/mcp.json`
- Modify: `docker-compose.prod.yml` (mcp-north-cloud service environment)

**Step 1: Update .mcp.json**

```json
{
  "mcpServers": {
    "North Cloud (Local)": {
      "command": "./mcp-north-cloud/run-mcp.sh",
      "env": {
        "MCP_ENV": "local",
        "INDEX_MANAGER_URL": "http://localhost:8090",
        "CRAWLER_URL": "http://localhost:8060",
        "SOURCE_MANAGER_URL": "http://localhost:8050",
        "PUBLISHER_URL": "http://localhost:8070",
        "SEARCH_URL": "http://localhost:8092",
        "CLASSIFIER_URL": "http://localhost:8071"
      }
    },
    "North Cloud (Production)": {
      "command": "ssh",
      "args": [
        "jones@northcloud.biz",
        "docker exec -i -e MCP_ENV=prod north-cloud-mcp-north-cloud-1 /app/mcp-north-cloud"
      ]
    }
  }
}
```

**Step 2: Update .cursor/mcp.json identically**

Same content as `.mcp.json`.

**Step 3: Add MCP_ENV to docker-compose.prod.yml**

Find the `mcp-north-cloud` service in `docker-compose.prod.yml` and add `MCP_ENV: prod` to its environment block.

**Step 4: Commit**

```bash
git add .mcp.json .cursor/mcp.json docker-compose.prod.yml
git commit -m "feat(mcp): set MCP_ENV in local/prod configs and docker compose"
```

---

### Task 5: Update test-tools.sh and Documentation

**Files:**
- Modify: `mcp-north-cloud/test-tools.sh`
- Modify: `mcp-north-cloud/CLAUDE.md`

**Step 1: Update test-tools.sh**

The test script currently checks for 27 tools. Update it to accept `MCP_ENV` and check the correct count:
- `MCP_ENV=local` (default): expect 18 tools
- `MCP_ENV=prod`: expect 24 tools

Change the expected count line. If the script uses a hardcoded `27`, replace with conditional logic:

```bash
if [ "${MCP_ENV:-local}" = "prod" ]; then
  EXPECTED_TOOLS=24
else
  EXPECTED_TOOLS=18
fi
```

**Step 2: Update CLAUDE.md**

Update the tool count references (27 -> environment-dependent) and add a section about MCP_ENV:

In the Quick Reference section, change `./test-tools.sh` comment from "27 tools" to "18 local / 24 prod tools".

Add after the "27 Tools by Category" table:

```markdown
## Environment-Based Tool Registry

Set `MCP_ENV` to control which tools are available:

| Environment | Tools | Count |
|-------------|-------|-------|
| `local` (default) | shared + local-only | 18 |
| `prod` | shared + prod-only | 24 |

**Shared (15):** list_sources, add_source, update_source, test_source, list_indexes, search_articles, list_crawl_jobs, get_crawl_stats, list_routes, list_channels, preview_route, get_publish_history, get_publisher_stats, classify_article, onboard_source

**Local-only (3):** lint_file, build_service, test_service

**Prod-only (9):** delete_index, delete_source, delete_route, control_crawl_job, start_crawl, schedule_crawl, create_route, create_channel, get_auth_token
```

**Step 3: Run updated test script**

Run: `cd /home/jones/dev/north-cloud/mcp-north-cloud && MCP_ENV=local ./test-tools.sh`
Expected: 18 tools registered

Run: `cd /home/jones/dev/north-cloud/mcp-north-cloud && MCP_ENV=prod ./test-tools.sh`
Expected: 24 tools registered

**Step 4: Commit**

```bash
git add mcp-north-cloud/test-tools.sh mcp-north-cloud/CLAUDE.md
git commit -m "docs(mcp): update tool counts and add MCP_ENV documentation"
```
