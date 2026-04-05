# Signal Crawler Sprint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dockerize signal-crawler for production deploy, add a shared renderer client for JS-heavy pages, and build a job boards adapter scanning 5 boards for infrastructure-intent signals.

**Architecture:** Signal-crawler is a Go oneshot service in the north-cloud monorepo. It implements `adapter.Source` for each signal source, orchestrated by a `runner.Runner` pipeline (scan → dedup → ingest). New work adds Docker packaging, a render client that calls the existing `playwright-renderer` service, and a `jobs` adapter with per-board parsers.

**Tech Stack:** Go 1.26+, Docker multi-stage builds, golang.org/x/net/html, net/http, encoding/json, testify, httptest

**Spec:** `docs/superpowers/specs/2026-04-05-signal-crawler-sprint-design.md`

---

## File Map

### Task 1: Dockerized Deploy
| Action | File |
|--------|------|
| Create | `signal-crawler/Dockerfile` |
| Modify | `docker-compose.base.yml` (add signal-crawler service) |
| Modify | `docker-compose.prod.yml` (add signal-crawler prod overrides) |
| Modify | `signal-crawler/deploy/signal-crawler.service` (docker compose run) |
| Modify | `scripts/deploy.sh` (skip health check for oneshot) |

### Task 2: Renderer Client
| Action | File |
|--------|------|
| Create | `signal-crawler/internal/render/render.go` |
| Create | `signal-crawler/internal/render/render_test.go` |
| Modify | `signal-crawler/internal/config/config.go` (add RendererConfig) |
| Modify | `signal-crawler/config.yml` (add renderer section) |
| Modify | `signal-crawler/main.go` (create render client, pass to adapters) |

### Task 3: Scoring Keywords
| Action | File |
|--------|------|
| Modify | `signal-crawler/internal/scoring/scoring.go` (add job keywords) |
| Create | `signal-crawler/internal/scoring/scoring_test.go` |

### Task 4: Job Boards Adapter
| Action | File |
|--------|------|
| Create | `signal-crawler/internal/adapter/jobs/jobs.go` (adapter + types) |
| Create | `signal-crawler/internal/adapter/jobs/jobs_test.go` |
| Create | `signal-crawler/internal/adapter/jobs/remoteok.go` (JSON parser) |
| Create | `signal-crawler/internal/adapter/jobs/remoteok_test.go` |
| Create | `signal-crawler/internal/adapter/jobs/wwr.go` (HTML parser) |
| Create | `signal-crawler/internal/adapter/jobs/wwr_test.go` |
| Create | `signal-crawler/internal/adapter/jobs/hnhiring.go` (HN Who's Hiring) |
| Create | `signal-crawler/internal/adapter/jobs/hnhiring_test.go` |
| Create | `signal-crawler/internal/adapter/jobs/gcjobs.go` (GC Jobs parser) |
| Create | `signal-crawler/internal/adapter/jobs/gcjobs_test.go` |
| Create | `signal-crawler/internal/adapter/jobs/workbc.go` (WorkBC parser) |
| Create | `signal-crawler/internal/adapter/jobs/workbc_test.go` |

### Task 5: Wire & Integration
| Action | File |
|--------|------|
| Modify | `signal-crawler/internal/config/config.go` (add JobsConfig) |
| Modify | `signal-crawler/config.yml` (add jobs section) |
| Modify | `signal-crawler/main.go` (register jobs adapter) |

---

## Task 1: Dockerized Deploy

### Task 1.1: Create Dockerfile

**Files:**
- Create: `signal-crawler/Dockerfile`

- [ ] **Step 1: Write the Dockerfile**

Follow the classifier pattern: multi-stage build with Go builder and alpine runtime.

```dockerfile
# Build stage
FROM golang:1.26.1-alpine AS builder

WORKDIR /build

# Copy shared infrastructure module first (for layer caching)
COPY infrastructure/ infrastructure/
COPY go.work go.work.sum ./

# Copy signal-crawler module
COPY signal-crawler/go.mod signal-crawler/go.sum signal-crawler/
WORKDIR /build/signal-crawler
RUN go mod download

# Copy source and build
COPY signal-crawler/ .
RUN CGO_ENABLED=1 go build -ldflags="-w -s" -trimpath -o signal-crawler main.go

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app
COPY --from=builder /build/signal-crawler/signal-crawler .
COPY --from=builder /build/signal-crawler/config.yml .

ENTRYPOINT ["./signal-crawler"]
```

Note: `CGO_ENABLED=1` because the dedup store uses SQLite via `mattn/go-sqlite3`.

- [ ] **Step 2: Verify it builds locally**

Run: `cd /home/jones/dev/north-cloud && docker build -f signal-crawler/Dockerfile -t signal-crawler:dev .`
Expected: Successful build, image created.

- [ ] **Step 3: Verify it runs**

Run: `docker run --rm signal-crawler:dev --dry-run`
Expected: Runs scan in dry-run mode, prints signal counts, exits 0.

- [ ] **Step 4: Commit**

```bash
git add signal-crawler/Dockerfile
git commit -m "feat(signal-crawler): add Dockerfile for Docker-based deploy"
```

### Task 1.2: Add docker-compose entries

**Files:**
- Modify: `docker-compose.base.yml`
- Modify: `docker-compose.prod.yml`

- [ ] **Step 1: Add base compose entry**

Add to `docker-compose.base.yml` in the services section (after other Go services):

```yaml
  signal-crawler:
    build:
      context: .
      dockerfile: signal-crawler/Dockerfile
    networks:
      - north-cloud-network
    volumes:
      - signal-crawler-data:/app/data
```

Add to the `volumes:` section at the bottom:

```yaml
  signal-crawler-data:
```

- [ ] **Step 2: Add prod compose overrides**

Add to `docker-compose.prod.yml` in the services section:

```yaml
  signal-crawler:
    image: docker.io/jonesrussell/signal-crawler:${SIGNAL_CRAWLER_TAG:-latest}
    restart: "no"
    networks:
      - north-cloud-network
    volumes:
      - /opt/north-cloud/signal-crawler/data:/app/data
    environment:
      - NORTHOPS_URL=${NORTHOPS_URL}
      - PIPELINE_API_KEY=${PIPELINE_API_KEY}
      - RENDERER_URL=http://playwright-renderer:8095
      - RENDERER_ENABLED=true
      - LOG_LEVEL=info
      - LOG_FORMAT=json
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 256M
```

Note: `restart: "no"` because this is a oneshot (timer triggers it, it runs once and exits).

- [ ] **Step 3: Verify compose config**

Run: `cd /home/jones/dev/north-cloud && docker compose -f docker-compose.base.yml -f docker-compose.prod.yml config --services | grep signal-crawler`
Expected: `signal-crawler` in the output.

- [ ] **Step 4: Commit**

```bash
git add docker-compose.base.yml docker-compose.prod.yml
git commit -m "feat(signal-crawler): add docker-compose entries for prod deploy"
```

### Task 1.3: Update systemd service for Docker

**Files:**
- Modify: `signal-crawler/deploy/signal-crawler.service`

- [ ] **Step 1: Update the service file**

Replace the entire `signal-crawler/deploy/signal-crawler.service`:

```ini
[Unit]
Description=Signal crawler — scan HN, funding, and job boards for lead signals
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=oneshot
WorkingDirectory=/home/deployer/north-cloud
ExecStart=/usr/bin/docker compose -f docker-compose.base.yml -f docker-compose.prod.yml run --rm signal-crawler
StandardOutput=journal
StandardError=journal
```

- [ ] **Step 2: Commit**

```bash
git add signal-crawler/deploy/signal-crawler.service
git commit -m "feat(signal-crawler): update systemd service to use docker compose run"
```

### Task 1.4: Skip health check for oneshot services

**Files:**
- Modify: `scripts/deploy.sh`

- [ ] **Step 1: Add signal-crawler to the health check skip list**

Find the health check loop in `scripts/deploy.sh` (around line 547). Add `signal-crawler` to the list of services that should skip health checks. Look for existing skip patterns (e.g., `NON_COMPOSE_SERVICES` or conditional checks in the health loop) and follow that pattern.

If no skip list exists, add one before the health check loop:

```bash
# Oneshot services don't have health endpoints — skip them
ONESHOT_SERVICES="signal-crawler"
```

Then in the health check loop, add:

```bash
if echo "$ONESHOT_SERVICES" | grep -qw "$service"; then
  echo "  Skipping health check for oneshot service: $service"
  continue
fi
```

- [ ] **Step 2: Verify deploy script syntax**

Run: `bash -n /home/jones/dev/north-cloud/scripts/deploy.sh`
Expected: No syntax errors.

- [ ] **Step 3: Commit**

```bash
git add scripts/deploy.sh
git commit -m "fix(deploy): skip health checks for oneshot services like signal-crawler"
```

---

## Task 2: Renderer Client

### Task 2.1: Write renderer client with tests

**Files:**
- Create: `signal-crawler/internal/render/render.go`
- Create: `signal-crawler/internal/render/render_test.go`

- [ ] **Step 1: Write the failing test**

Create `signal-crawler/internal/render/render_test.go`:

```go
package render_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Render_Success(t *testing.T) {
	t.Helper()

	expectedHTML := "<html><body>rendered content</body></html>"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/render", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req map[string]string
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "https://example.com", req["url"])
		assert.Equal(t, "networkidle", req["wait_for"])

		resp := map[string]string{"html": expectedHTML, "url": "https://example.com"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := render.New(srv.URL)
	html, err := client.Render(context.Background(), "https://example.com")

	require.NoError(t, err)
	assert.Equal(t, expectedHTML, html)
}

func TestClient_Render_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "renderer busy", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := render.New(srv.URL)
	_, err := client.Render(context.Background(), "https://example.com")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestClient_Render_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	client := render.New(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Render(ctx, "https://example.com")
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go test ./internal/render/... -v`
Expected: FAIL — package does not exist yet.

- [ ] **Step 3: Write the implementation**

Create `signal-crawler/internal/render/render.go`:

```go
package render

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultTimeout    = 45 * time.Second
	maxResponseBytes  = 10 * 1024 * 1024 // 10 MB
)

// Client calls a playwright-renderer service to render JS-heavy pages.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a renderer client pointing at the given base URL.
func New(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

type renderRequest struct {
	URL     string `json:"url"`
	WaitFor string `json:"wait_for"`
}

type renderResponse struct {
	HTML string `json:"html"`
}

// Render sends a URL to the playwright-renderer and returns the rendered HTML.
func (c *Client) Render(ctx context.Context, targetURL string) (string, error) {
	body, err := json.Marshal(renderRequest{
		URL:     targetURL,
		WaitFor: "networkidle",
	})
	if err != nil {
		return "", fmt.Errorf("render: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/render", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("render: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("render: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("render: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	var result renderResponse
	if decErr := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&result); decErr != nil {
		return "", fmt.Errorf("render: decode response: %w", decErr)
	}

	return result.HTML, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go test ./internal/render/... -v`
Expected: All 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add signal-crawler/internal/render/
git commit -m "feat(signal-crawler): add renderer client for JS-heavy pages"
```

### Task 2.2: Add renderer config and wire into main

**Files:**
- Modify: `signal-crawler/internal/config/config.go`
- Modify: `signal-crawler/config.yml`
- Modify: `signal-crawler/main.go`

- [ ] **Step 1: Add RendererConfig to config.go**

Add to `signal-crawler/internal/config/config.go`, after `FundingConfig`:

```go
// RendererConfig holds playwright-renderer configuration.
type RendererConfig struct {
	URL     string `env:"RENDERER_URL"     yaml:"url"`
	Enabled bool   `env:"RENDERER_ENABLED" yaml:"enabled"`
}
```

Add the field to `Config`:

```go
type Config struct {
	NorthOps NorthOpsConfig  `yaml:"northops"`
	Dedup    DedupConfig     `yaml:"dedup"`
	Logging  LoggingConfig   `yaml:"logging"`
	HN       HNConfig        `yaml:"hn"`
	Funding  FundingConfig   `yaml:"funding"`
	Renderer RendererConfig  `yaml:"renderer"`
}
```

Add default to `SetDefaults`:

```go
if cfg.Renderer.URL == "" {
	cfg.Renderer.URL = "http://localhost:8095"
}
```

- [ ] **Step 2: Add renderer section to config.yml**

Add to the end of `signal-crawler/config.yml`:

```yaml

# Playwright renderer (for JS-heavy pages)
renderer:
  url: ""              # Leave empty for default localhost:8095 (env: RENDERER_URL)
  enabled: false       # Set true in prod with renderer available (env: RENDERER_ENABLED)
```

- [ ] **Step 3: Wire renderer into main.go**

Add import:
```go
"github.com/jonesrussell/north-cloud/signal-crawler/internal/render"
```

In `main()`, after the `setup()` call and before `buildSources()`, add:

```go
var renderer *render.Client
if cfg.Renderer.Enabled {
	renderer = render.New(cfg.Renderer.URL)
	log.Info("renderer enabled", infralogger.String("url", cfg.Renderer.URL))
}
```

Update `buildSources` signature to accept the renderer:

```go
func buildSources(cfg *config.Config, sourceFilter string, log infralogger.Logger, renderer *render.Client) ([]adapter.Source, error) {
```

Update the call in `main()`:

```go
sources, err := buildSources(cfg, *sourceFilter, log, renderer)
```

The `renderer` variable will be used in Task 5 when wiring the jobs adapter. For now it's passed through but unused.

- [ ] **Step 4: Verify it compiles**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go build -o /dev/null main.go`
Expected: Compiles with no errors.

- [ ] **Step 5: Commit**

```bash
git add signal-crawler/internal/config/config.go signal-crawler/config.yml signal-crawler/main.go
git commit -m "feat(signal-crawler): wire renderer config into main"
```

---

## Task 3: Scoring Keywords

### Task 3.1: Add job-specific keywords with tests

**Files:**
- Modify: `signal-crawler/internal/scoring/scoring.go`
- Create: `signal-crawler/internal/scoring/scoring_test.go`

- [ ] **Step 1: Write tests for existing + new keywords**

Create `signal-crawler/internal/scoring/scoring_test.go`:

```go
package scoring_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/scoring"
	"github.com/stretchr/testify/assert"
)

func TestScore_ExistingKeywords(t *testing.T) {
	tests := []struct {
		text          string
		expectedScore int
		expectedMatch string
	}{
		{"We are looking for CTO to lead engineering", scoring.ScoreDirectAsk, "looking for cto"},
		{"Time to rebuild mvp from scratch", scoring.ScoreStrongSignal, "rebuild mvp"},
		{"Our legacy system needs work", scoring.ScoreWeakSignal, "legacy system"},
		{"Just a regular post about nothing", 0, ""},
	}

	for _, tt := range tests {
		score, matched := scoring.Score(tt.text)
		assert.Equal(t, tt.expectedScore, score, "text: %s", tt.text)
		assert.Equal(t, tt.expectedMatch, matched, "text: %s", tt.text)
	}
}

func TestScore_JobKeywords(t *testing.T) {
	tests := []struct {
		text          string
		expectedScore int
		expectedMatch string
	}{
		{"We're hiring platform engineer to rebuild our infra", scoring.ScoreDirectAsk, "hiring platform engineer"},
		{"Need cloud architect for AWS migration", scoring.ScoreDirectAsk, "need cloud architect"},
		{"Looking for devops lead to automate deployments", scoring.ScoreDirectAsk, "looking for devops"},
		{"Migrating monolith to microservices architecture", scoring.ScoreStrongSignal, "monolith to microservices"},
		{"Major cloud migration project starting Q2", scoring.ScoreStrongSignal, "cloud migration"},
		{"Infrastructure overhaul across all regions", scoring.ScoreStrongSignal, "infrastructure overhaul"},
		{"Platform modernization initiative underway", scoring.ScoreStrongSignal, "platform modernization"},
		{"Facing scaling challenges with current setup", scoring.ScoreWeakSignal, "scaling challenges"},
		{"We're growing engineering team rapidly", scoring.ScoreWeakSignal, "growing engineering team"},
		{"Time to start modernizing stack", scoring.ScoreWeakSignal, "modernizing stack"},
	}

	for _, tt := range tests {
		score, matched := scoring.Score(tt.text)
		assert.Equal(t, tt.expectedScore, score, "text: %s", tt.text)
		assert.Equal(t, tt.expectedMatch, matched, "text: %s", tt.text)
	}
}

func TestScore_HighestWins(t *testing.T) {
	// Text matches both DirectAsk and StrongSignal — highest should win
	text := "Hiring platform engineer for cloud migration project"
	score, _ := scoring.Score(text)
	assert.Equal(t, scoring.ScoreDirectAsk, score)
}
```

- [ ] **Step 2: Run tests to verify new keyword tests fail**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go test ./internal/scoring/... -v`
Expected: `TestScore_ExistingKeywords` PASS, `TestScore_JobKeywords` FAIL (new keywords not added yet).

- [ ] **Step 3: Add job-specific keywords to scoring.go**

Add to the `keywords` slice in `signal-crawler/internal/scoring/scoring.go`, after existing entries:

```go
	// Job board — direct ask (score 90)
	{phrase: "hiring platform engineer", score: ScoreDirectAsk},
	{phrase: "need cloud architect", score: ScoreDirectAsk},
	{phrase: "looking for devops", score: ScoreDirectAsk},

	// Job board — strong signal (score 70)
	{phrase: "monolith to microservices", score: ScoreStrongSignal},
	{phrase: "cloud migration", score: ScoreStrongSignal},
	{phrase: "infrastructure overhaul", score: ScoreStrongSignal},
	{phrase: "platform modernization", score: ScoreStrongSignal},

	// Job board — weak signal (score 40)
	{phrase: "scaling challenges", score: ScoreWeakSignal},
	{phrase: "growing engineering team", score: ScoreWeakSignal},
	{phrase: "modernizing stack", score: ScoreWeakSignal},
```

- [ ] **Step 4: Run tests to verify all pass**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go test ./internal/scoring/... -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add signal-crawler/internal/scoring/
git commit -m "feat(signal-crawler): add job board scoring keywords"
```

---

## Task 4: Job Boards Adapter

### Task 4.1: Core adapter types and orchestration

**Files:**
- Create: `signal-crawler/internal/adapter/jobs/jobs.go`
- Create: `signal-crawler/internal/adapter/jobs/jobs_test.go`

- [ ] **Step 1: Write the failing test**

Create `signal-crawler/internal/adapter/jobs/jobs_test.go`:

```go
package jobs_test

import (
	"context"
	"testing"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter/jobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubBoard struct {
	name     string
	postings []jobs.Posting
	err      error
}

func (s *stubBoard) Name() string { return s.name }
func (s *stubBoard) Fetch(_ context.Context) ([]jobs.Posting, error) {
	return s.postings, s.err
}

func TestAdapter_Name(t *testing.T) {
	a := jobs.New(nil, nil)
	assert.Equal(t, "jobs", a.Name())
}

func TestAdapter_Scan_ScoresAndFilters(t *testing.T) {
	log, _ := infralogger.New(infralogger.Config{Level: "error", Format: "console"})

	boards := []jobs.Board{
		&stubBoard{
			name: "test-board",
			postings: []jobs.Posting{
				{Title: "Hiring platform engineer", Company: "Acme", URL: "https://example.com/1", ID: "1"},
				{Title: "Office Manager", Company: "Boring Co", URL: "https://example.com/2", ID: "2"},
				{Title: "Cloud migration lead needed", Company: "CloudCo", URL: "https://example.com/3", ID: "3"},
			},
		},
	}

	a := jobs.New(boards, log)
	signals, err := a.Scan(context.Background())

	require.NoError(t, err)
	assert.Len(t, signals, 2) // "Office Manager" has no matching keywords

	assert.Equal(t, "Acme — Hiring platform engineer", signals[0].Label)
	assert.Equal(t, "test-board|1", signals[0].ExternalID)
	assert.Equal(t, 90, signals[0].SignalStrength)

	assert.Equal(t, "CloudCo — Cloud migration lead needed", signals[1].Label)
	assert.Equal(t, "test-board|3", signals[1].ExternalID)
	assert.Equal(t, 70, signals[1].SignalStrength)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go test ./internal/adapter/jobs/... -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write the implementation**

Create `signal-crawler/internal/adapter/jobs/jobs.go`:

```go
package jobs

import (
	"context"
	"errors"
	"fmt"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/scoring"
)

// Posting represents a single job listing extracted by a board parser.
type Posting struct {
	Title   string
	Company string
	URL     string
	ID      string
	Body    string
	Sector  string
}

// Board fetches job postings from a single source.
type Board interface {
	Name() string
	Fetch(ctx context.Context) ([]Posting, error)
}

// Adapter scans multiple job boards for infrastructure-intent signals.
type Adapter struct {
	boards []Board
	log    infralogger.Logger
}

// New creates a jobs Adapter with the given boards and logger.
func New(boards []Board, log infralogger.Logger) *Adapter {
	return &Adapter{boards: boards, log: log}
}

// Name returns the short identifier for this adapter.
func (a *Adapter) Name() string {
	return "jobs"
}

// Scan fetches postings from all boards, scores them, and returns matching signals.
func (a *Adapter) Scan(ctx context.Context) ([]adapter.Signal, error) {
	var allSignals []adapter.Signal
	var errs []error

	for _, board := range a.boards {
		postings, err := board.Fetch(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("jobs: board %s: %w", board.Name(), err))
			if a.log != nil {
				a.log.Warn("board fetch failed",
					infralogger.String("board", board.Name()),
					infralogger.Error(err),
				)
			}
			continue
		}

		matched := 0
		for _, p := range postings {
			combined := p.Title + " " + p.Body
			score, phrase := scoring.Score(combined)
			if score == 0 {
				continue
			}

			label := p.Title
			if p.Company != "" {
				label = p.Company + " — " + p.Title
			}

			sector := p.Sector
			if sector == "" {
				sector = "tech"
			}

			allSignals = append(allSignals, adapter.Signal{
				Label:          label,
				SourceURL:      p.URL,
				ExternalID:     board.Name() + "|" + p.ID,
				SignalStrength: score,
				Sector:         sector,
				Notes:          fmt.Sprintf("Matched: %s (via %s)", phrase, board.Name()),
			})
			matched++
		}

		if a.log != nil {
			a.log.Info("board scan complete",
				infralogger.String("board", board.Name()),
				infralogger.Int("total", len(postings)),
				infralogger.Int("matched", matched),
			)
		}
	}

	return allSignals, errors.Join(errs...)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go test ./internal/adapter/jobs/... -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add signal-crawler/internal/adapter/jobs/
git commit -m "feat(signal-crawler): add jobs adapter core with Board interface"
```

### Task 4.2: RemoteOK board (JSON API)

**Files:**
- Create: `signal-crawler/internal/adapter/jobs/remoteok.go`
- Create: `signal-crawler/internal/adapter/jobs/remoteok_test.go`

- [ ] **Step 1: Write the failing test**

Create `signal-crawler/internal/adapter/jobs/remoteok_test.go`:

```go
package jobs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter/jobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoteOK_Fetch(t *testing.T) {
	// RemoteOK returns an array where first element is metadata (object with "0" key or "legal" key)
	fixture := `[
		{"legal": "https://remoteok.com/legal"},
		{"slug": "platform-engineer-acme", "company": "Acme Corp", "position": "Platform Engineer", "url": "https://remoteok.com/l/platform-engineer-acme", "id": "12345", "description": "Build cloud infrastructure"},
		{"slug": "designer-xyz", "company": "XYZ Inc", "position": "UI Designer", "url": "https://remoteok.com/l/designer-xyz", "id": "12346", "description": "Design interfaces"}
	]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fixture))
	}))
	defer srv.Close()

	board := jobs.NewRemoteOK(srv.URL)
	postings, err := board.Fetch(context.Background())

	require.NoError(t, err)
	require.Len(t, postings, 2)

	assert.Equal(t, "Platform Engineer", postings[0].Title)
	assert.Equal(t, "Acme Corp", postings[0].Company)
	assert.Equal(t, "12345", postings[0].ID)
	assert.Contains(t, postings[0].Body, "Build cloud infrastructure")

	assert.Equal(t, "UI Designer", postings[1].Title)
	assert.Equal(t, "XYZ Inc", postings[1].Company)
}

func TestRemoteOK_Name(t *testing.T) {
	board := jobs.NewRemoteOK("https://remoteok.com/api")
	assert.Equal(t, "remoteok", board.Name())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go test ./internal/adapter/jobs/... -v -run TestRemoteOK`
Expected: FAIL — `NewRemoteOK` not defined.

- [ ] **Step 3: Write the implementation**

Create `signal-crawler/internal/adapter/jobs/remoteok.go`:

```go
package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const remoteOKTimeout = 30 * time.Second

type remoteOKJob struct {
	Slug        string `json:"slug"`
	Company     string `json:"company"`
	Position    string `json:"position"`
	URL         string `json:"url"`
	ID          string `json:"id"`
	Description string `json:"description"`
}

// RemoteOKBoard fetches job postings from the RemoteOK JSON API.
type RemoteOKBoard struct {
	apiURL     string
	httpClient *http.Client
}

// NewRemoteOK creates a board that fetches from the RemoteOK API.
func NewRemoteOK(apiURL string) *RemoteOKBoard {
	return &RemoteOKBoard{
		apiURL:     apiURL,
		httpClient: &http.Client{Timeout: remoteOKTimeout},
	}
}

func (b *RemoteOKBoard) Name() string { return "remoteok" }

func (b *RemoteOKBoard) Fetch(ctx context.Context) ([]Posting, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.apiURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("remoteok: create request: %w", err)
	}
	req.Header.Set("User-Agent", "north-cloud-signal-crawler/1.0")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remoteok: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("remoteok: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("remoteok: read body: %w", err)
	}

	// RemoteOK returns a JSON array. First element is metadata (has "legal" key), rest are jobs.
	var raw []json.RawMessage
	if unmarshalErr := json.Unmarshal(body, &raw); unmarshalErr != nil {
		return nil, fmt.Errorf("remoteok: decode array: %w", unmarshalErr)
	}

	var postings []Posting
	for i, entry := range raw {
		if i == 0 {
			continue // skip metadata element
		}
		var job remoteOKJob
		if unmarshalErr := json.Unmarshal(entry, &job); unmarshalErr != nil {
			continue
		}
		if job.Position == "" {
			continue
		}
		postings = append(postings, Posting{
			Title:   job.Position,
			Company: job.Company,
			URL:     job.URL,
			ID:      job.ID,
			Body:    job.Description,
			Sector:  "tech",
		})
	}

	return postings, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go test ./internal/adapter/jobs/... -v -run TestRemoteOK`
Expected: All RemoteOK tests PASS.

- [ ] **Step 5: Commit**

```bash
git add signal-crawler/internal/adapter/jobs/remoteok.go signal-crawler/internal/adapter/jobs/remoteok_test.go
git commit -m "feat(signal-crawler): add RemoteOK job board parser"
```

### Task 4.3: WeWorkRemotely board (HTML)

**Files:**
- Create: `signal-crawler/internal/adapter/jobs/wwr.go`
- Create: `signal-crawler/internal/adapter/jobs/wwr_test.go`

- [ ] **Step 1: Write the failing test**

Create `signal-crawler/internal/adapter/jobs/wwr_test.go`:

```go
package jobs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter/jobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const wwrFixture = `<html><body>
<section class="jobs">
  <ul>
    <li>
      <a href="/remote-jobs/platform-engineer-acme">
        <span class="company">Acme Corp</span>
        <span class="title">Platform Engineer</span>
      </a>
    </li>
    <li>
      <a href="/remote-jobs/devops-lead-cloudco">
        <span class="company">CloudCo</span>
        <span class="title">DevOps Lead</span>
      </a>
    </li>
  </ul>
</section>
</body></html>`

func TestWWR_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(wwrFixture))
	}))
	defer srv.Close()

	board := jobs.NewWWR(srv.URL)
	postings, err := board.Fetch(context.Background())

	require.NoError(t, err)
	require.Len(t, postings, 2)

	assert.Equal(t, "Platform Engineer", postings[0].Title)
	assert.Equal(t, "Acme Corp", postings[0].Company)
	assert.Contains(t, postings[0].URL, "/remote-jobs/platform-engineer-acme")

	assert.Equal(t, "DevOps Lead", postings[1].Title)
	assert.Equal(t, "CloudCo", postings[1].Company)
}

func TestWWR_Name(t *testing.T) {
	board := jobs.NewWWR("https://weworkremotely.com")
	assert.Equal(t, "wwr", board.Name())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go test ./internal/adapter/jobs/... -v -run TestWWR`
Expected: FAIL — `NewWWR` not defined.

- [ ] **Step 3: Write the implementation**

Create `signal-crawler/internal/adapter/jobs/wwr.go`:

```go
package jobs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const wwrTimeout = 30 * time.Second

// WWRBoard fetches job postings from WeWorkRemotely HTML pages.
type WWRBoard struct {
	baseURL    string
	httpClient *http.Client
}

// NewWWR creates a board that scrapes WeWorkRemotely.
func NewWWR(baseURL string) *WWRBoard {
	return &WWRBoard{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: wwrTimeout},
	}
}

func (b *WWRBoard) Name() string { return "wwr" }

func (b *WWRBoard) Fetch(ctx context.Context) ([]Posting, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("wwr: create request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wwr: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("wwr: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("wwr: read body: %w", err)
	}

	return parseWWR(string(body), b.baseURL)
}

func parseWWR(htmlContent, baseURL string) ([]Posting, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("wwr: parse html: %w", err)
	}

	base, _ := url.Parse(baseURL)

	var postings []Posting
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		// Look for <li> elements containing job links
		if isElem(n, "li") {
			p := extractWWRJob(n, base)
			if p.Title != "" {
				postings = append(postings, p)
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return postings, nil
}

func extractWWRJob(li *html.Node, base *url.URL) Posting {
	var p Posting
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if isElem(n, "a") && p.URL == "" {
			href := getNodeAttr(n, "href")
			if href != "" && strings.Contains(href, "remote-jobs") {
				if ref, err := url.Parse(href); err == nil {
					p.URL = base.ResolveReference(ref).String()
				}
				// Use href as ID (last path segment)
				parts := strings.Split(strings.TrimRight(href, "/"), "/")
				if len(parts) > 0 {
					p.ID = parts[len(parts)-1]
				}
			}
		}
		if isElem(n, "span") {
			cls := getNodeAttr(n, "class")
			text := strings.TrimSpace(nodeText(n))
			switch {
			case strings.Contains(cls, "title") && text != "":
				p.Title = text
			case strings.Contains(cls, "company") && text != "":
				p.Company = text
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(li)
	return p
}

func isElem(n *html.Node, tag string) bool {
	return n.Type == html.ElementNode && n.Data == tag
}

func getNodeAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go test ./internal/adapter/jobs/... -v -run TestWWR`
Expected: All WWR tests PASS.

- [ ] **Step 5: Commit**

```bash
git add signal-crawler/internal/adapter/jobs/wwr.go signal-crawler/internal/adapter/jobs/wwr_test.go
git commit -m "feat(signal-crawler): add WeWorkRemotely job board parser"
```

### Task 4.4: HN Who's Hiring board

**Files:**
- Create: `signal-crawler/internal/adapter/jobs/hnhiring.go`
- Create: `signal-crawler/internal/adapter/jobs/hnhiring_test.go`

- [ ] **Step 1: Write the failing test**

Create `signal-crawler/internal/adapter/jobs/hnhiring_test.go`:

```go
package jobs_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter/jobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHNHiring_Fetch(t *testing.T) {
	mux := http.NewServeMux()

	// Algolia HN search API returns hiring threads
	mux.HandleFunc("/api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"hits": []map[string]any{
				{"objectID": "99001", "title": "Ask HN: Who is hiring? (April 2026)"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	// HN Firebase API for thread kids
	mux.HandleFunc("/v0/item/99001.json", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"id": 99001, "type": "story", "kids": []int{99002, 99003},
		}
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/v0/item/99002.json", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"id": 99002, "type": "comment",
			"text": "Acme Corp | Platform Engineer | Remote | We are hiring platform engineer to lead cloud migration",
			"by":   "acme_cto",
		}
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/v0/item/99003.json", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"id": 99003, "type": "comment",
			"text": "XYZ Inc | Marketing Manager | NYC",
			"by":   "xyz_hr",
		}
		json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	board := jobs.NewHNHiring(srv.URL, srv.URL, 100)
	postings, err := board.Fetch(context.Background())

	require.NoError(t, err)
	require.Len(t, postings, 2)

	assert.Equal(t, "Acme Corp | Platform Engineer | Remote", postings[0].Title)
	assert.Equal(t, "Acme Corp", postings[0].Company)
	assert.Equal(t, fmt.Sprintf("%s/item?id=99002", "https://news.ycombinator.com"), postings[0].URL)
	assert.Equal(t, "99002", postings[0].ID)
}

func TestHNHiring_Name(t *testing.T) {
	board := jobs.NewHNHiring("", "", 100)
	assert.Equal(t, "hn-hiring", board.Name())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go test ./internal/adapter/jobs/... -v -run TestHNHiring`
Expected: FAIL — `NewHNHiring` not defined.

- [ ] **Step 3: Write the implementation**

Create `signal-crawler/internal/adapter/jobs/hnhiring.go`:

```go
package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	hnHiringTimeout = 30 * time.Second
	defaultAlgoliaURL  = "https://hn.algolia.com"
	defaultFirebaseURL = "https://hacker-news.firebaseio.com"
)

type algoliaSearchResult struct {
	Hits []struct {
		ObjectID string `json:"objectID"`
		Title    string `json:"title"`
	} `json:"hits"`
}

type hnItem struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	Text string `json:"text"`
	By   string `json:"by"`
	Kids []int  `json:"kids"`
}

// HNHiringBoard fetches job postings from the monthly "Who is hiring?" thread.
type HNHiringBoard struct {
	algoliaURL  string
	firebaseURL string
	maxComments int
	httpClient  *http.Client
}

// NewHNHiring creates a board that parses HN "Who is hiring?" threads.
// algoliaURL is used to find the latest thread; firebaseURL fetches individual items.
func NewHNHiring(algoliaURL, firebaseURL string, maxComments int) *HNHiringBoard {
	if algoliaURL == "" {
		algoliaURL = defaultAlgoliaURL
	}
	if firebaseURL == "" {
		firebaseURL = defaultFirebaseURL
	}
	if maxComments <= 0 {
		maxComments = 200
	}
	return &HNHiringBoard{
		algoliaURL:  algoliaURL,
		firebaseURL: firebaseURL,
		maxComments: maxComments,
		httpClient:  &http.Client{Timeout: hnHiringTimeout},
	}
}

func (b *HNHiringBoard) Name() string { return "hn-hiring" }

func (b *HNHiringBoard) Fetch(ctx context.Context) ([]Posting, error) {
	threadID, err := b.findLatestThread(ctx)
	if err != nil {
		return nil, fmt.Errorf("hn-hiring: find thread: %w", err)
	}

	thread, err := b.fetchItem(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("hn-hiring: fetch thread: %w", err)
	}

	kids := thread.Kids
	if len(kids) > b.maxComments {
		kids = kids[:b.maxComments]
	}

	var postings []Posting
	for _, kidID := range kids {
		comment, fetchErr := b.fetchItem(ctx, kidID)
		if fetchErr != nil {
			continue
		}
		if comment.Type != "comment" || comment.Text == "" {
			continue
		}

		p := parseHiringComment(comment)
		postings = append(postings, p)
	}

	return postings, nil
}

func (b *HNHiringBoard) findLatestThread(ctx context.Context) (int, error) {
	searchURL := b.algoliaURL + "/api/v1/search?query=%22Who+is+hiring%22&tags=ask_hn&hitsPerPage=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, http.NoBody)
	if err != nil {
		return 0, fmt.Errorf("hn-hiring: create search request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("hn-hiring: search: %w", err)
	}
	defer resp.Body.Close()

	var result algoliaSearchResult
	if decErr := json.NewDecoder(resp.Body).Decode(&result); decErr != nil {
		return 0, fmt.Errorf("hn-hiring: decode search: %w", decErr)
	}

	if len(result.Hits) == 0 {
		return 0, fmt.Errorf("hn-hiring: no hiring thread found")
	}

	id, err := strconv.Atoi(result.Hits[0].ObjectID)
	if err != nil {
		return 0, fmt.Errorf("hn-hiring: parse thread ID: %w", err)
	}

	return id, nil
}

func (b *HNHiringBoard) fetchItem(ctx context.Context, id int) (*hnItem, error) {
	itemURL := fmt.Sprintf("%s/v0/item/%d.json", b.firebaseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, itemURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var item hnItem
	if decErr := json.NewDecoder(resp.Body).Decode(&item); decErr != nil {
		return nil, decErr
	}
	return &item, nil
}

// parseHiringComment extracts a job posting from an HN hiring thread comment.
// Format is typically: "Company | Role | Location | Details..."
func parseHiringComment(item *hnItem) Posting {
	text := item.Text
	// Strip HTML tags (HN comments are HTML-encoded)
	text = strings.ReplaceAll(text, "<p>", "\n")
	text = strings.ReplaceAll(text, "&#x27;", "'")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = stripHTMLTags(text)

	lines := strings.SplitN(text, "\n", 2)
	firstLine := strings.TrimSpace(lines[0])

	body := ""
	if len(lines) > 1 {
		body = strings.TrimSpace(lines[1])
	}

	// Parse "Company | Role | Location" from first line
	parts := strings.SplitN(firstLine, "|", 4)
	company := ""
	title := firstLine
	if len(parts) >= 2 {
		company = strings.TrimSpace(parts[0])
		// Title = everything except company (first 3 pipe-separated fields)
		titleParts := parts[1:]
		if len(titleParts) > 2 {
			titleParts = titleParts[:2] // Keep role + location, drop rest
		}
		trimmed := make([]string, len(titleParts))
		for i, p := range titleParts {
			trimmed[i] = strings.TrimSpace(p)
		}
		title = company + " | " + strings.Join(trimmed, " | ")
	}

	return Posting{
		Title:   title,
		Company: company,
		URL:     fmt.Sprintf("https://news.ycombinator.com/item?id=%d", item.ID),
		ID:      strconv.Itoa(item.ID),
		Body:    body,
		Sector:  "tech",
	}
}

func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	return b.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go test ./internal/adapter/jobs/... -v -run TestHNHiring`
Expected: All HNHiring tests PASS.

- [ ] **Step 5: Commit**

```bash
git add signal-crawler/internal/adapter/jobs/hnhiring.go signal-crawler/internal/adapter/jobs/hnhiring_test.go
git commit -m "feat(signal-crawler): add HN Who's Hiring job board parser"
```

### Task 4.5: GC Jobs board (Canadian government)

**Files:**
- Create: `signal-crawler/internal/adapter/jobs/gcjobs.go`
- Create: `signal-crawler/internal/adapter/jobs/gcjobs_test.go`

- [ ] **Step 1: Write the failing test**

Create `signal-crawler/internal/adapter/jobs/gcjobs_test.go`:

```go
package jobs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter/jobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const gcJobsFixture = `<html><body>
<div class="search-results">
  <article class="resultJobItem">
    <h3><a href="/en/job/1001">Cloud Infrastructure Analyst</a></h3>
    <div class="department">Treasury Board of Canada</div>
  </article>
  <article class="resultJobItem">
    <h3><a href="/en/job/1002">Administrative Assistant</a></h3>
    <div class="department">Health Canada</div>
  </article>
</div>
</body></html>`

func TestGCJobs_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(gcJobsFixture))
	}))
	defer srv.Close()

	board := jobs.NewGCJobs(srv.URL)
	postings, err := board.Fetch(context.Background())

	require.NoError(t, err)
	require.Len(t, postings, 2)

	assert.Equal(t, "Cloud Infrastructure Analyst", postings[0].Title)
	assert.Equal(t, "Treasury Board of Canada", postings[0].Company)
	assert.Contains(t, postings[0].URL, "/en/job/1001")
	assert.Equal(t, "1001", postings[0].ID)
	assert.Equal(t, "government", postings[0].Sector)

	assert.Equal(t, "Administrative Assistant", postings[1].Title)
	assert.Equal(t, "Health Canada", postings[1].Company)
}

func TestGCJobs_Name(t *testing.T) {
	board := jobs.NewGCJobs("https://emploisfp-psjobs.cfp-psc.gc.ca")
	assert.Equal(t, "gcjobs", board.Name())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go test ./internal/adapter/jobs/... -v -run TestGCJobs`
Expected: FAIL — `NewGCJobs` not defined.

- [ ] **Step 3: Write the implementation**

Create `signal-crawler/internal/adapter/jobs/gcjobs.go`:

```go
package jobs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const gcJobsTimeout = 30 * time.Second

// GCJobsBoard fetches job postings from the Government of Canada jobs portal.
type GCJobsBoard struct {
	baseURL    string
	httpClient *http.Client
}

// NewGCJobs creates a board that scrapes GC Jobs.
func NewGCJobs(baseURL string) *GCJobsBoard {
	return &GCJobsBoard{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: gcJobsTimeout},
	}
}

func (b *GCJobsBoard) Name() string { return "gcjobs" }

func (b *GCJobsBoard) Fetch(ctx context.Context) ([]Posting, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("gcjobs: create request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gcjobs: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("gcjobs: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("gcjobs: read body: %w", err)
	}

	return parseGCJobs(string(body), b.baseURL)
}

func parseGCJobs(htmlContent, baseURL string) ([]Posting, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("gcjobs: parse html: %w", err)
	}

	base, _ := url.Parse(baseURL)

	var postings []Posting
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if isElem(n, "article") && strings.Contains(getNodeAttr(n, "class"), "resultJobItem") {
			p := extractGCJob(n, base)
			if p.Title != "" {
				postings = append(postings, p)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return postings, nil
}

func extractGCJob(article *html.Node, base *url.URL) Posting {
	var p Posting
	p.Sector = "government"

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		// Title + URL from h3 > a
		if isElem(n, "h3") {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if isElem(c, "a") {
					p.Title = strings.TrimSpace(nodeText(c))
					href := getNodeAttr(c, "href")
					if href != "" {
						if ref, parseErr := url.Parse(href); parseErr == nil {
							p.URL = base.ResolveReference(ref).String()
						}
						// Extract ID from last path segment
						parts := strings.Split(strings.TrimRight(href, "/"), "/")
						if len(parts) > 0 {
							p.ID = parts[len(parts)-1]
						}
					}
				}
			}
		}
		// Department from div.department
		if isElem(n, "div") && strings.Contains(getNodeAttr(n, "class"), "department") {
			p.Company = strings.TrimSpace(nodeText(n))
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(article)

	return p
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go test ./internal/adapter/jobs/... -v -run TestGCJobs`
Expected: All GCJobs tests PASS.

- [ ] **Step 5: Commit**

```bash
git add signal-crawler/internal/adapter/jobs/gcjobs.go signal-crawler/internal/adapter/jobs/gcjobs_test.go
git commit -m "feat(signal-crawler): add GC Jobs (Canadian government) board parser"
```

### Task 4.6: WorkBC board (with optional renderer)

**Files:**
- Create: `signal-crawler/internal/adapter/jobs/workbc.go`
- Create: `signal-crawler/internal/adapter/jobs/workbc_test.go`

- [ ] **Step 1: Write the failing test**

Create `signal-crawler/internal/adapter/jobs/workbc_test.go`:

```go
package jobs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter/jobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const workBCFixture = `<html><body>
<div id="search-results">
  <div class="job-posting">
    <h2><a href="/jobs/12345">Cloud Systems Administrator</a></h2>
    <span class="employer">BC Hydro</span>
    <span class="location">Vancouver, BC</span>
  </div>
  <div class="job-posting">
    <h2><a href="/jobs/12346">Office Clerk</a></h2>
    <span class="employer">City of Victoria</span>
    <span class="location">Victoria, BC</span>
  </div>
</div>
</body></html>`

func TestWorkBC_Fetch_StaticFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(workBCFixture))
	}))
	defer srv.Close()

	// No renderer — falls back to static fetch
	board := jobs.NewWorkBC(srv.URL, nil)
	postings, err := board.Fetch(context.Background())

	require.NoError(t, err)
	require.Len(t, postings, 2)

	assert.Equal(t, "Cloud Systems Administrator", postings[0].Title)
	assert.Equal(t, "BC Hydro", postings[0].Company)
	assert.Contains(t, postings[0].URL, "/jobs/12345")
	assert.Equal(t, "12345", postings[0].ID)
	assert.Equal(t, "government", postings[0].Sector)
}

func TestWorkBC_Name(t *testing.T) {
	board := jobs.NewWorkBC("https://www.workbc.ca", nil)
	assert.Equal(t, "workbc", board.Name())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go test ./internal/adapter/jobs/... -v -run TestWorkBC`
Expected: FAIL — `NewWorkBC` not defined.

- [ ] **Step 3: Write the implementation**

Create `signal-crawler/internal/adapter/jobs/workbc.go`:

```go
package jobs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/render"
	"golang.org/x/net/html"
)

const workBCTimeout = 30 * time.Second

// WorkBCBoard fetches job postings from WorkBC (British Columbia).
type WorkBCBoard struct {
	baseURL    string
	renderer   *render.Client
	httpClient *http.Client
}

// NewWorkBC creates a board that scrapes WorkBC.
// If renderer is non-nil, it's used for JS rendering; otherwise falls back to static fetch.
func NewWorkBC(baseURL string, renderer *render.Client) *WorkBCBoard {
	return &WorkBCBoard{
		baseURL:    baseURL,
		renderer:   renderer,
		httpClient: &http.Client{Timeout: workBCTimeout},
	}
}

func (b *WorkBCBoard) Name() string { return "workbc" }

func (b *WorkBCBoard) Fetch(ctx context.Context) ([]Posting, error) {
	htmlContent, err := b.fetchHTML(ctx)
	if err != nil {
		return nil, err
	}
	return parseWorkBC(htmlContent, b.baseURL)
}

func (b *WorkBCBoard) fetchHTML(ctx context.Context) (string, error) {
	if b.renderer != nil {
		content, err := b.renderer.Render(ctx, b.baseURL)
		if err != nil {
			return "", fmt.Errorf("workbc: renderer: %w", err)
		}
		return content, nil
	}

	// Static fallback
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("workbc: create request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("workbc: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("workbc: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", fmt.Errorf("workbc: read body: %w", err)
	}

	return string(body), nil
}

func parseWorkBC(htmlContent, baseURL string) ([]Posting, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("workbc: parse html: %w", err)
	}

	base, _ := url.Parse(baseURL)

	var postings []Posting
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if isElem(n, "div") && strings.Contains(getNodeAttr(n, "class"), "job-posting") {
			p := extractWorkBCJob(n, base)
			if p.Title != "" {
				postings = append(postings, p)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return postings, nil
}

func extractWorkBCJob(div *html.Node, base *url.URL) Posting {
	var p Posting
	p.Sector = "government"

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if isElem(n, "h2") {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if isElem(c, "a") {
					p.Title = strings.TrimSpace(nodeText(c))
					href := getNodeAttr(c, "href")
					if href != "" {
						if ref, parseErr := url.Parse(href); parseErr == nil {
							p.URL = base.ResolveReference(ref).String()
						}
						parts := strings.Split(strings.TrimRight(href, "/"), "/")
						if len(parts) > 0 {
							p.ID = parts[len(parts)-1]
						}
					}
				}
			}
		}
		if isElem(n, "span") {
			cls := getNodeAttr(n, "class")
			text := strings.TrimSpace(nodeText(n))
			if strings.Contains(cls, "employer") && text != "" {
				p.Company = text
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(div)

	return p
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go test ./internal/adapter/jobs/... -v -run TestWorkBC`
Expected: All WorkBC tests PASS.

- [ ] **Step 5: Commit**

```bash
git add signal-crawler/internal/adapter/jobs/workbc.go signal-crawler/internal/adapter/jobs/workbc_test.go
git commit -m "feat(signal-crawler): add WorkBC job board parser with optional renderer"
```

---

## Task 5: Wire Jobs Adapter & Integration

### Task 5.1: Add jobs config and register adapter

**Files:**
- Modify: `signal-crawler/internal/config/config.go`
- Modify: `signal-crawler/config.yml`
- Modify: `signal-crawler/main.go`

- [ ] **Step 1: Add JobsConfig to config.go**

Add to `signal-crawler/internal/config/config.go`, after `RendererConfig`:

```go
// JobsConfig holds job board adapter configuration.
type JobsConfig struct {
	RemoteOKURL   string `env:"JOBS_REMOTEOK_URL"   yaml:"remoteok_url"`
	WWRURL        string `env:"JOBS_WWR_URL"         yaml:"wwr_url"`
	HNMaxComments int    `env:"JOBS_HN_MAX_COMMENTS" yaml:"hn_max_comments"`
	GCJobsURL     string `env:"JOBS_GCJOBS_URL"      yaml:"gcjobs_url"`
	WorkBCURL     string `env:"JOBS_WORKBC_URL"       yaml:"workbc_url"`
}
```

Add the field to `Config`:

```go
type Config struct {
	NorthOps NorthOpsConfig  `yaml:"northops"`
	Dedup    DedupConfig     `yaml:"dedup"`
	Logging  LoggingConfig   `yaml:"logging"`
	HN       HNConfig        `yaml:"hn"`
	Funding  FundingConfig   `yaml:"funding"`
	Renderer RendererConfig  `yaml:"renderer"`
	Jobs     JobsConfig      `yaml:"jobs"`
}
```

Add defaults to `SetDefaults`:

```go
if cfg.Jobs.RemoteOKURL == "" {
	cfg.Jobs.RemoteOKURL = "https://remoteok.com/api"
}
if cfg.Jobs.WWRURL == "" {
	cfg.Jobs.WWRURL = "https://weworkremotely.com/categories/remote-devops-sysadmin-jobs"
}
if cfg.Jobs.HNMaxComments == 0 {
	cfg.Jobs.HNMaxComments = 200
}
if cfg.Jobs.GCJobsURL == "" {
	cfg.Jobs.GCJobsURL = "https://emploisfp-psjobs.cfp-psc.gc.ca/psrs-srfp/applicant/page2440?selectionProcessNumber=&officialLanguage=E&title=cloud+OR+devops+OR+infrastructure+OR+platform"
}
if cfg.Jobs.WorkBCURL == "" {
	cfg.Jobs.WorkBCURL = "https://www.workbc.ca/find-jobs/browse-jobs?searchTerm=cloud+engineer+OR+devops+OR+platform"
}
```

- [ ] **Step 2: Add jobs section to config.yml**

Add to the end of `signal-crawler/config.yml`:

```yaml

# Job boards adapter
jobs:
  remoteok_url: ""       # Leave empty for default (env: JOBS_REMOTEOK_URL)
  wwr_url: ""            # Leave empty for default (env: JOBS_WWR_URL)
  hn_max_comments: 200   # Max HN hiring thread comments to scan (env: JOBS_HN_MAX_COMMENTS)
  gcjobs_url: ""         # Leave empty for default (env: JOBS_GCJOBS_URL)
  workbc_url: ""         # Leave empty for default (env: JOBS_WORKBC_URL)
```

- [ ] **Step 3: Register the jobs adapter in main.go**

Add import:
```go
"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter/jobs"
```

Update `buildSources` to create and register the jobs adapter:

```go
func buildSources(cfg *config.Config, sourceFilter string, log infralogger.Logger, renderer *render.Client) ([]adapter.Source, error) {
	boards := []jobs.Board{
		jobs.NewRemoteOK(cfg.Jobs.RemoteOKURL),
		jobs.NewWWR(cfg.Jobs.WWRURL),
		jobs.NewHNHiring("", "", cfg.Jobs.HNMaxComments),
		jobs.NewGCJobs(cfg.Jobs.GCJobsURL),
		jobs.NewWorkBC(cfg.Jobs.WorkBCURL, renderer),
	}

	all := []adapter.Source{
		hn.New(cfg.HN.BaseURL, cfg.HN.MaxItems, log),
		funding.New(cfg.Funding.URLs),
		jobs.New(boards, log),
	}

	if sourceFilter == "" {
		return all, nil
	}

	var filtered []adapter.Source
	for _, src := range all {
		if src.Name() == sourceFilter {
			filtered = append(filtered, src)
		}
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("unknown source %q", sourceFilter)
	}

	return filtered, nil
}
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go build -o /dev/null main.go`
Expected: Compiles with no errors.

- [ ] **Step 5: Run all tests**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go test ./... -v`
Expected: All tests PASS.

- [ ] **Step 6: Run linter**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && golangci-lint run --config ../.golangci.yml ./...`
Expected: No lint errors.

- [ ] **Step 7: Dry-run smoke test**

Run: `cd /home/jones/dev/north-cloud/signal-crawler && go run main.go --dry-run --source jobs 2>&1 | head -20`
Expected: Prints JSON log lines showing board scans with signal counts. No HTTP POST errors (dry-run mode).

- [ ] **Step 8: Commit**

```bash
git add signal-crawler/internal/config/config.go signal-crawler/config.yml signal-crawler/main.go
git commit -m "feat(signal-crawler): wire jobs adapter with 5 boards into main"
```

---

## Post-Implementation

After all tasks are complete:

1. Run full lint + test suite: `cd /home/jones/dev/north-cloud/signal-crawler && task lint && task test`
2. Run dry-run with all adapters: `task run:dry`
3. Create PR targeting main
4. After merge, CI will build and push the Docker image
5. SSH to VPS to install systemd timer (one-time manual step): copy `signal-crawler.service` and `signal-crawler.timer` to `/etc/systemd/system/`, enable timer
