# Signal Crawler Sprint: Deploy + Renderer + Job Boards

**Date:** 2026-04-05
**Status:** Approved
**Scope:** Three tasks to get signal-crawler running in production with expanded source coverage

---

## Task 1: VPS Deploy Pipeline

### Problem

Signal-crawler has systemd unit files (`deploy/signal-crawler.service`, `deploy/signal-crawler.timer`) but no CI/CD step to build and ship the binary to the VPS.

### Design

Add a deploy job to `.github/workflows/deploy.yml` triggered on main when `signal-crawler/` changes.

**Steps:**
1. Cross-compile: `GOOS=linux GOARCH=amd64 go build -o bin/signal-crawler ./signal-crawler`
2. SCP binary + systemd files to VPS at `/opt/north-cloud/signal-crawler/`
3. SSH to install:
   - Copy `signal-crawler.service` and `signal-crawler.timer` to `/etc/systemd/system/`
   - `systemctl daemon-reload`
   - `systemctl enable --now signal-crawler.timer`
4. First deploy creates `.env` from `.env.example` if absent (secrets require manual edit)

**No rollback mechanism.** The crawler is a timer-based oneshot. If the binary is bad, the next run fails and the journal shows why.

**Config on VPS:**
- Binary: `/opt/north-cloud/signal-crawler/signal-crawler`
- Env: `/opt/north-cloud/signal-crawler/.env`
- Timer: daily 06:00 UTC, 5-minute jitter, persistent (catches up if missed)

---

## Task 2: Headless Renderer Client

### Problem

The funding adapter scrapes OTF static HTML. Some pages (OTF, future job boards) require JavaScript rendering. North-cloud already runs a `playwright-renderer` service on port 8095.

### Design

New shared package: `internal/render/render.go`

**Client struct:**
```go
type Client struct {
    baseURL    string
    httpClient *http.Client
}

func (c *Client) Render(ctx context.Context, url string) (string, error)
```

- Calls `POST /render` on playwright-renderer with `{url, wait_for: "networkidle"}`
- Returns rendered HTML string
- Respects context timeout, returns clean errors

**Integration with adapters:**
- Adapters that need JS rendering accept an optional `*render.Client`
- If renderer is nil, adapter falls back to plain HTTP fetch
- This allows dev mode (no renderer) and prod mode (renderer available)

**Config addition to `config.yml`:**
```yaml
renderer:
  url: "http://localhost:8095"
  enabled: false
```

When `enabled: true`, `main.go` creates a `render.Client` and passes it to adapters that need it. When false, adapters use static fetch.

---

## Task 3: Job Boards Adapter

### Problem

Signal-crawler currently monitors Hacker News and government funding. Job postings are a high-signal source for identifying companies with infrastructure needs. GitHub issue #590.

### Design

New package: `internal/adapter/jobs/jobs.go`

**Target boards:**

| Board | URL | Format | Rendering |
|-------|-----|--------|-----------|
| WeWorkRemotely | weworkremotely.com/categories/remote-devops-sysadmin-jobs | Static HTML | Plain fetch |
| RemoteOK | remoteok.com/api | JSON API | Plain fetch |
| HN "Who's Hiring" | Firebase API (monthly thread) | JSON | Plain fetch |
| GC Jobs | jobs-emplois.gc.ca | Static HTML | Plain fetch |
| WorkBC | workbc.ca | Static HTML | Renderer (optional) |

**Architecture:**

```go
type BoardConfig struct {
    Name       string
    URL        string
    FetchMode  FetchMode // Static, Renderer, JSON
    Parser     BoardParser
}

type BoardParser interface {
    Parse(html string) ([]JobPosting, error)
}

type JobPosting struct {
    Title   string
    Company string
    URL     string
    ID      string
    Body    string
}
```

The `jobs.Adapter` implements `adapter.Source`:
- Iterates configured boards
- Fetches content (plain HTTP, renderer, or JSON depending on board)
- Parses postings via board-specific parsers
- Scores each posting with `scoring.Score()` against title + body
- Emits signals for postings that score > 0

**Board-specific parsers:**
- `wwr.go` — HTML parser for WeWorkRemotely
- `remoteok.go` — JSON parser for RemoteOK API
- `hnhiring.go` — Firebase API parser for monthly "Who's Hiring" threads
- `gcjobs.go` — HTML parser for GC Jobs
- `workbc.go` — HTML parser for WorkBC

**New scoring keywords** (added to `scoring.go`):

| Score | Phrases |
|-------|---------|
| 90 (DirectAsk) | "hiring platform engineer", "need cloud architect", "looking for devops" |
| 70 (StrongSignal) | "monolith to microservices", "cloud migration", "infrastructure overhaul", "platform modernization" |
| 40 (WeakSignal) | "scaling challenges", "growing engineering team", "modernizing stack" |

**Signal mapping:**
- `Label`: `"{Company} — {Title}"` (or just title if no company)
- `SourceURL`: direct link to job posting
- `ExternalID`: `"{board}|{posting-id}"` for dedup
- `Sector`: `"tech"` for tech boards, `"government"` for GC/WorkBC
- `Notes`: matched keyword + board name
- `Endpoint()`: `/api/leads/ingest/signal`

**HN "Who's Hiring" vs existing HN adapter:**
The existing HN adapter scans all new stories for intent keywords. The jobs adapter targets the monthly "Who's Hiring" thread specifically, parsing individual comments as job postings. Different data source (thread comments vs stories), different parsing, distinct dedup keys. Lives in the jobs adapter, not `hn.go`.

**Registration in `main.go`:**
```go
all := []adapter.Source{
    hn.New(cfg.HN.BaseURL, cfg.HN.MaxItems, log),
    funding.New(cfg.Funding.URLs),
    jobs.New(cfg.Jobs, renderer, log),  // new
}
```

Filterable via `--source jobs`.

---

## Dependencies

- Task 2 (renderer client) should land before Task 3, since the jobs adapter may use it for WorkBC
- Task 1 (deploy) is independent and can be done in parallel
- NorthOps `/api/leads/ingest/signal` endpoint must exist (already shipped in PR #127)

## Testing

- Each board parser gets unit tests with fixture HTML/JSON
- Renderer client gets tests with httptest mock server
- Integration: `--dry-run --source jobs` against live boards
- Deploy: manual verification of systemd timer status after first CI deploy
