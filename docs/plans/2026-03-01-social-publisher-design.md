# Social Publisher: Design Document

**Date:** 2026-03-01
**Status:** Approved
**Approach:** NorthCloud microservice (Approach A)

## Overview

`social-publisher` is a new Go microservice in the NorthCloud monorepo. It subscribes to Redis Pub/Sub channels, transforms content per platform, publishes to external APIs, and tracks delivery status. It fans content out to 9 platforms: X, LinkedIn, Dev.to, Reddit, Facebook, Blogger, YouTube, Gumroad, and the Hugo blog (via Dev.to draft activation).

This service extends NorthCloud's existing content pipeline without coupling platform logic into the main publisher.

---

## 1. Service Architecture

### Placement

Downstream consumer of Redis Pub/Sub events, positioned after publisher and classifier in the pipeline.

```
Content -> Classifier -> Publisher -> Redis Pub/Sub
                                          |
                                    social-publisher
                                          |
              +--------+--------+---------+--------+---------+
              v        v        v         v        v         v
             X    LinkedIn   Dev.to    Reddit   Facebook   Blogger
                                                  Gumroad   YouTube
```

### Service Details

- **Port:** 8077
- **Config:** TOML/env, consistent with other NorthCloud services
- **Per-platform `enabled` flag** for incremental rollout

### Redis Channel Topology

- **Subscribe:** `social:publish` (content ready for distribution)
- **Publish:** `social:delivery-status` (delivery outcomes back to pipeline)
- **Publish:** `social:dead-letter` (failed deliveries after max retries, includes full original message)

### Service Boundaries

- **Input:** Normalized content events from Redis or HTTP API
- **Processing:** Transform, validate, publish, track
- **Output:** Delivery status events to Redis

Does not: fetch content, classify content, generate content, or store long-term state beyond delivery tracking.

---

## 2. Platform Adapters

### Interface

```go
type PlatformAdapter interface {
    Name() string
    Capabilities() PlatformCapabilities
    Transform(content Content) (PlatformPost, error)
    Validate(post PlatformPost) error
    Publish(ctx context.Context, post PlatformPost) (DeliveryResult, error)
}

type PlatformCapabilities struct {
    SupportsImages    bool
    SupportsThreading bool
    SupportsMarkdown  bool
    SupportsHTML      bool
    MaxLength         int
    RequiresMetadata  []string // e.g., ["subreddit"] for Reddit
}
```

Each adapter owns: content transformation, validation, publishing, platform-specific error handling, and platform-specific rate limits. The orchestrator uses `Capabilities()` to avoid "if platform == X" branching.

### Platform Transformation Rules

| Platform | Format | Limits | Notes |
|----------|--------|--------|-------|
| X | Short text + link | 280 chars | Thread splitter preserving URLs and markdown |
| LinkedIn | Professional post + link | 3,000 chars | Article format supported |
| Dev.to | Draft-to-published flipper | N/A | RSS creates drafts; adapter flips via API |
| Reddit | Title + link or self-post | Subreddit-specific | Subreddit required in metadata |
| Facebook | Post + link | 63,206 chars | Normalize link metadata for preview consistency |
| Blogger | Full HTML | No practical limit | Canonical URL injection |
| Gumroad | Product update | Varies | Product ID required in metadata |
| YouTube | Community post | 5,000 chars | Optional image support via capabilities |

### Authentication Flows (5 distinct for 9 platforms)

- **X:** OAuth 2.0 with PKCE, handle elevated access tiers
- **LinkedIn:** OAuth 2.0
- **Dev.to:** API key (simplest)
- **Reddit:** OAuth 2.0, short-lived tokens with automatic renewal
- **Google (Blogger + YouTube):** OAuth 2.0 with shared credentials, different scopes
- **Facebook:** Page access token via Graph API, include refresh helper for unpredictable expiry
- **Gumroad:** API token

All OAuth tokens stored with refresh tokens and expiry timestamps. Auto-refresh by adapters with `token_refreshed` event emission.

### Error Types

| Error Type | Behavior |
|------------|----------|
| `RateLimitError` | Retry after adapter-specified cooldown (`RetryAfter` duration) |
| `TransientError` | Retry with exponential backoff |
| `AuthError` | Attempt token refresh, retry once; further failures are permanent |
| `PermanentError` | Dead-letter immediately; include platform's raw error code |
| `ValidationError` | Dead-letter immediately; raised before any API call |

---

## 3. Content Model

### Canonical Message Format (all trigger paths converge here)

```go
type PublishMessage struct {
    ContentID   string            `json:"content_id"`
    Type        ContentType       `json:"type"`
    Title       string            `json:"title,omitempty"`
    Body        string            `json:"body,omitempty"`
    Summary     string            `json:"summary"`
    URL         string            `json:"url,omitempty"`
    Images      []string          `json:"images,omitempty"`
    Tags        []string          `json:"tags,omitempty"`
    Project     string            `json:"project"`
    Targets     []TargetConfig    `json:"targets,omitempty"`
    ScheduledAt *time.Time        `json:"scheduled_at,omitempty"`
    Metadata    map[string]string `json:"metadata,omitempty"`
    RetryCount  int               `json:"retry_count"`
    Source      string            `json:"source"`
}

type TargetConfig struct {
    Platform string  `json:"platform"`
    Account  string  `json:"account"`
    Override *string `json:"override,omitempty"` // nil = no override
}

type ContentType string

const (
    BlogPost            ContentType = "blog_post"
    SocialUpdate        ContentType = "social_update"
    ProductAnnouncement ContentType = "product_announcement"
)
```

### Field Semantics

- `Summary` is an authored short-form field, not a truncation of `Body`
- `Override` uses pointer type; nil means "no override," empty string is not valid
- `Account` validates against the credential registry, not free-form
- `blog_post` requires canonical URL (enforced in validation)
- `social_update` allows no URL
- `Tags` are advisory metadata; used by Dev.to and Reddit, ignored by most platforms
- `Images` are optional per platform based on capabilities
- `Source` is for observability only ("github_action", "claudia", "northcloud_pipeline")
- Missing required metadata is a permanent error, straight to dead-letter
- Validation runs before transformation

### Content Types

| Type | Source | Flow |
|------|--------|------|
| `blog_post` | Hugo publish triggers Redis event | Auto-routed to configured platforms |
| `social_update` | Manual or Claudia-initiated | On-demand to specified targets |
| `product_announcement` | Project-specific | Routed through project account mappings |

---

## 4. Trigger Flow

### Path 1: Blog Post Published (automatic)

```
git push main
    -> GitHub Actions builds Hugo, deploys to Pages
    -> New workflow step detects newly published posts (diff + draft:false check)
    -> Sends PublishMessage to social-publisher API (or Redis)
    -> social-publisher resolves routing, fans out to platforms
```

Includes stable content ID (slug) for idempotency. Multi-post merges handled by scanning all changed files.

### Path 2: Manual/Claudia-Initiated (on demand)

```
User tells Claudia -> Claudia drafts per-platform content
    -> User approves -> Claudia submits via MCP tool or HTTP API
    -> social-publisher delivers
```

### Path 3: Internal NorthCloud Pipeline (Redis)

```
NorthCloud publisher emits to social:publish channel
    -> social-publisher consumes, transforms, validates, publishes
```

### Cross-Path Behaviors

- All paths produce identical `PublishMessage` format
- All paths produce identical `DeliveryEvent` format
- Idempotency via stable content ID across all paths
- Scheduling respected across all paths
- Same validation logic across all paths

### Network Considerations

- Path 1: social-publisher needs to be reachable from GitHub Actions (public endpoint or Redis bridge)
- Path 2: Claudia needs API access or MCP tools
- Path 3: Internal only, no external exposure

---

## 5. Delivery Tracking & Error Handling

### Database Schema

```sql
CREATE TABLE content (
    id          TEXT PRIMARY KEY,
    type        TEXT NOT NULL,
    title       TEXT,
    body        TEXT,
    summary     TEXT,
    url         TEXT,
    images      JSONB,
    tags        JSONB,
    project     TEXT NOT NULL,
    metadata    JSONB,
    source      TEXT NOT NULL,
    published   BOOLEAN NOT NULL DEFAULT false,
    scheduled_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE deliveries (
    id            TEXT PRIMARY KEY,
    content_id    TEXT NOT NULL REFERENCES content(id),
    platform      TEXT NOT NULL,
    account       TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending',
    platform_id   TEXT,
    platform_url  TEXT,
    error         TEXT,
    attempts      INT NOT NULL DEFAULT 0,
    max_attempts  INT NOT NULL DEFAULT 3,
    next_retry_at TIMESTAMPTZ,
    last_error_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at  TIMESTAMPTZ,
    UNIQUE(content_id, platform, account)
);

CREATE INDEX idx_deliveries_retry ON deliveries (status, next_retry_at);

CREATE TABLE accounts (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL UNIQUE,
    platform      TEXT NOT NULL,
    project       TEXT NOT NULL,
    enabled       BOOLEAN NOT NULL DEFAULT true,
    credentials   BYTEA,
    token_expiry  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Status Transitions

```
pending -> publishing -> delivered        (happy path)
pending -> publishing -> retrying         (transient error)
retrying -> publishing -> delivered       (retry succeeds, attempts preserved)
retrying -> publishing -> retrying        (retry fails, attempts < max)
retrying -> failed                        (attempts >= max, dead-letter)
```

Content-level status: `publish_failed` when all targets fail.

### Delivery Events (published to `social:delivery-status`)

```go
type DeliveryEvent struct {
    ContentID   string    `json:"content_id"`
    ContentType string    `json:"content_type"`
    DeliveryID  string    `json:"delivery_id"`
    Platform    string    `json:"platform"`
    Account     string    `json:"account"`
    Status      string    `json:"status"`
    PlatformID  string    `json:"platform_id,omitempty"`
    PlatformURL string    `json:"platform_url,omitempty"`
    Error       string    `json:"error,omitempty"`
    RetryAfter  *int      `json:"retry_after,omitempty"`
    Attempts    int       `json:"attempts"`
    Timestamp   time.Time `json:"timestamp"`
}
```

### Dead-Letter Events

Same shape as `DeliveryEvent` plus:
- Full original `PublishMessage` for resubmission
- `error_type` (validation, auth, permanent)
- `platform_response` when available

### Observability Events

- `publishing` when delivery enters in-flight state
- `retry_scheduled` when next_retry_at is set
- `token_refreshed` when adapter refreshes OAuth credentials
- `rate_limited` when limiter defers a publish
- `routing_resolved` when routing rules are applied
- `routing_warning` when disabled/credentialless accounts are skipped
- `scheduled_publish_triggered` when scheduler fires a due content item
- `retry_requested` when manual retry is submitted

---

## 6. Account Management

### Account Registry (TOML config)

```toml
[accounts.personal-x]
platform = "x"
project = "personal"
enabled = true

[accounts.personal-linkedin]
platform = "linkedin"
project = "personal"
enabled = true

[accounts.northcloud-x]
platform = "x"
project = "northcloud"
enabled = true

[accounts.goformx-x]
platform = "x"
project = "goformx"
enabled = true

[accounts.personal-devto]
platform = "devto"
project = "personal"
enabled = true

[accounts.personal-reddit]
platform = "reddit"
project = "personal"
enabled = true

[accounts.personal-facebook]
platform = "facebook"
project = "personal"
enabled = true

[accounts.personal-blogger]
platform = "blogger"
project = "personal"
enabled = true

[accounts.personal-youtube]
platform = "youtube"
project = "personal"
enabled = true

[accounts.personal-gumroad]
platform = "gumroad"
project = "personal"
enabled = true
```

TOML is source of truth for which accounts exist. Database is source of truth for credentials and token state. On startup, reconcile: missing DB rows created, missing credentials trigger OAuth flows, disabled accounts skipped.

### Routing Rules

```toml
[routing.blog_post]
targets = ["personal-x", "personal-linkedin", "personal-devto",
           "personal-reddit", "personal-facebook"]

[routing."blog_post:northcloud"]
targets = ["northcloud-x", "personal-linkedin", "personal-devto",
           "personal-reddit", "personal-facebook"]

[routing."blog_post:goformx"]
targets = ["goformx-x", "personal-linkedin", "personal-devto",
           "personal-reddit", "personal-facebook"]

[routing.social_update]
targets = ["personal-x", "personal-linkedin"]

[routing."social_update:northcloud"]
targets = ["northcloud-x"]

[routing.product_announcement]
targets = ["personal-x", "personal-linkedin", "personal-facebook",
           "personal-gumroad"]
```

### Routing Resolution Order

1. Explicit targets in PublishMessage (highest priority)
2. Project-specific rule: `routing."<type>:<project>"` (fully overrides default, no merging)
3. Default rule: `routing."<type>"`
4. No match: reject with validation error

### Startup Validation

- Every account's `platform` matches a registered adapter
- Every routing rule target maps to a known account
- Disabled accounts excluded from routing but logged
- Missing credentials trigger warning, not crash

---

## 7. Claudia Integration

### `/publish` Claudia Skill

High-touch AI-assisted publishing for manual/on-demand content.

**Workflow:**
1. Identifies content type and targets from user intent
2. Drafts platform-specific content using adapter capabilities (length, tone, format)
3. Shows resolved routing rule and account names in preview
4. Validates required metadata before submission (e.g., "Reddit requires subreddit")
5. Submits only after explicit user approval
6. Reports delivery status via `social.status`

**Skill boundaries:**
- Does not manage OAuth or credentials
- Does not handle retry logic
- Does not override routing rules unless user specifies explicit targets
- Does not schedule unless user provides ScheduledAt

### NorthCloud MCP Tool Extensions

Four tools added to `mcp-north-cloud` (currently 27 tools):

| Tool | Purpose |
|------|---------|
| `social.publish` | Submit content; returns ContentID for immediate status check |
| `social.status` | Check delivery status for a content ID |
| `social.accounts` | List configured accounts with enabled/credential status |
| `social.retry` | Resubmit a failed delivery; emits `retry_requested` event |

### Workflow by Trigger Type

- **Blog posts:** Claudia monitors via `social.status`, alerts on failures, offers retry
- **Manual posts:** Claudia drafts, user approves, Claudia submits via `social.publish`, monitors
- **Internal pipeline:** Claudia optional; can monitor and retry failures

---

## 8. Retry Worker & Scheduling Loop

### Retry Worker

Polls `deliveries` table every 30 seconds for items where `status = 'retrying' AND next_retry_at <= NOW()`. Processes in batches of 50.

**Backoff:** 30s, 2min, 10min. Rate-limit errors use adapter's `RetryAfter` duration instead.

**Behaviors:**
- Does not reset attempt count on success (preserves audit trail)
- Permanent errors skip backoff, dead-letter immediately
- Clock-drift protection: next retry calculated relative to now, not previous timestamp
- Emits `retry_scheduled` event on every retry deferral

### Scheduling Loop

Polls `content` table every 60 seconds for `scheduled_at <= NOW() AND published = false`.

**Behaviors:**
- Uses transaction to mark content published + create delivery records atomically
- Skips disabled accounts and accounts missing credentials (logs + emits `routing_warning`)
- Content that fails all targets marked `publish_failed`
- Emits `scheduled_publish_triggered` event

### Concurrency Control

```go
type PublishOrchestrator struct {
    adapters     map[string]PlatformAdapter
    rateLimiters map[string]*rate.Limiter  // per-platform, shared between real-time and retries
    realtime     chan PublishJob            // high priority
    retries      chan PublishJob            // low priority
    sem          *semaphore.Weighted       // global concurrency limit
}
```

Two-queue priority model: real-time publishes always drain first. Retries flow when there's capacity. Per-platform rate limiters shared between both queues to prevent burst spikes.

---

## 9. Phasing & Rollout

### Phase 0.5: Smoke Test

Before wiring GitHub Actions, hit `/api/publish` manually with a test `social_update`. Confirm:
- Delivery record created in Postgres
- Retry behavior works (simulate transient error)
- Events appear on `social:delivery-status` Redis channel

### Phase 1: Foundation + X Adapter (week 1)

- Service skeleton (config, logging, health check, graceful shutdown)
- PostgreSQL schema
- Redis subscriber/publisher
- Core orchestrator with priority queue, rate limiters, semaphore
- Retry worker and scheduler goroutines
- **Adapter template with test harness** (canonical pattern for all future adapters)
- X adapter (OAuth 2.0, tweet posting, thread splitting)
- HTTP API (publish, status, accounts)
- Blog integration: GitHub Action step

**Deliverable:** Push a blog post, it automatically appears on X.

### Phase 2: Core Platform Adapters (week 2)

- LinkedIn adapter
- Dev.to adapter (draft-to-published flipper)
- Reddit adapter (with subreddit targeting)

**Deliverable:** Blog posts fan out to X, LinkedIn, Dev.to, and Reddit.

### Phase 3: MCP Tools + Remaining Adapters + Claudia (week 3)

**First:** MCP tools (`social.status`, `social.retry`, `social.accounts`, `social.publish`)
**Then:**
- Facebook adapter
- Blogger adapter
- YouTube adapter
- Gumroad adapter
- Claudia `/publish` skill

**Deliverable:** Full 9-channel distribution. Claudia can draft, submit, and monitor.

### Phase 4: Observability & Polish (week 4)

- Pipeline service integration
- Dead-letter inspection and resubmission
- Account health monitoring
- Routing validation CLI
- Comprehensive logging and metrics

**Deliverable:** Full production observability via NorthCloud dashboard.

### Phase 5: PipelineX Integration (future)

- Expose as PipelineX API endpoint
- Billing/quota for external users
- Dashboard publishing management

**Deliverable:** Social publishing as a PipelineX product feature.
