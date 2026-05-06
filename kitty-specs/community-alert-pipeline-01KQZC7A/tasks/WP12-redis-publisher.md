---
work_package_id: WP12
title: Redis Publisher
dependencies:
- WP05
- WP06
requirement_refs:
- C-005
- FR-005
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T049
- T050
- T051
- T052
phase: B
agent: "claude:opus:reviewer:reviewer"
shell_pid: "268677"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: alert-crawler/internal/redis/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- alert-crawler/internal/redis/**
priority: P1
tags: []
---

# WP12 — Redis Publisher

## Objective

Publish `LifecycleEvent` JSON payloads to Redis pub/sub channel `community_alerts:lifecycle`. Wraps the existing `infrastructure/redis` client. Failures are observable but non-fatal (ES is canonical; Redis is the live notification bus).

## Context

- Spec §3 FR-005, §5 C-005
- Plan §Component Design (Redis publisher), §TC-013
- Contracts: `contracts/lifecycle-event.schema.json`, `contracts/redis-channels.md`

## Branch Strategy

Standard. Parallel-safe.

## Subtasks

### T049 — Create `internal/redis/publisher.go`

**Purpose**: `Publisher` type wrapping the existing infra client.

**Steps**:
1. Create `alert-crawler/internal/redis/publisher.go`:
   ```go
   package redis

   import (
       "context"
       "encoding/json"
       "fmt"

       infraredis "github.com/jonesrussell/north-cloud/infrastructure/redis"

       "github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
   )

   type Publisher struct {
       client  infraredis.Client
       channel string
   }

   type Config struct {
       URL     string
       Channel string
   }

   func New(cfg Config) (*Publisher, error) {
       client, err := infraredis.New(cfg.URL)
       if err != nil {
           return nil, fmt.Errorf("redis client: %w", err)
       }
       return &Publisher{client: client, channel: cfg.Channel}, nil
   }

   func (p *Publisher) Publish(ctx context.Context, event domain.LifecycleEvent) error {
       payload, err := json.Marshal(event)
       if err != nil {
           return fmt.Errorf("marshal lifecycle event: %w", err)
       }
       return p.client.Publish(ctx, p.channel, payload)
   }

   func (p *Publisher) Close() error { return p.client.Close() }
   ```
2. Note: the actual `infrastructure/redis` package's surface may differ; the agent should adapt to the real interface. The contract here is "wrap, don't reimplement".

**Files**:
- `alert-crawler/internal/redis/publisher.go` (new, ~50 lines).

### T050 — Channel name from config; failures are metrics, not rollbacks

**Purpose**: Honor TC-013 (channel name from config); ensure publish failures don't roll back the ES write.

**Steps**:
1. The channel comes from `Publisher.channel` initialized at construction.
2. The runner (WP15) calls Publish AFTER a successful ES write. If Publish fails, the runner increments `alert_crawler.redis.publish_failure_total{event_type=...}` (WP15 observability) and logs at WARN. The ES write is NOT rolled back; the alert is durably stored regardless.
3. Document this contract in this WP's CLAUDE.md gotcha section: "Redis publish failures are recoverable (ES is canonical). Subscribers fall back to ES query (NFR-004)."

**Files**:
- `alert-crawler/internal/redis/publisher.go` (no changes; contract is enforced by WP15 caller).

### T051 — Wrap `infrastructure/redis`; do not write a bespoke client

**Purpose**: Charter compliance (C-002: no new scaffold-level dependencies).

**Steps**:
1. Verify the agent uses `infrastructure/redis` for the underlying connection management.
2. Do NOT directly import `github.com/redis/go-redis/v9` or similar; only via the infra package.
3. If `infrastructure/redis` is missing a needed method, propose extending the infra package as a follow-on (out of this WP scope) — but for v1, verify what's available is sufficient.

**Files**:
- No new files.

### T052 — Unit tests with mock Redis

**Purpose**: Verify serialization shape and channel routing.

**Steps**:
1. Create `alert-crawler/internal/redis/publisher_test.go`:
   - **TestPublish_Serializes**: build a LifecycleEvent; call Publish on a mocked client; assert the JSON payload conforms to `contracts/lifecycle-event.schema.json`.
   - **TestPublish_ChannelHonored**: assert the publish goes to `community_alerts:lifecycle` (not a different channel).
   - **TestPublish_PropagatesError**: mock client returns error; Publisher returns wrapped error.
   - **TestPublish_NilContextRejected**: defensive (avoid passing nil context downstream).
2. Use a small interface-based mock for `infraredis.Client` so we can assert calls.
3. `t.Helper()`. Coverage ≥80%.

**Files**:
- `alert-crawler/internal/redis/publisher_test.go` (new, ~150 lines).

**Validation**:
- All tests pass.
- Coverage ≥80%.
- The marshaled payload matches the JSON schema (validate via a stdlib JSON validator or string-shape assertions).

## Definition of Done

- Publisher wraps infrastructure/redis.
- Publishes to `community_alerts:lifecycle` (configurable).
- Serialization matches the schema.
- Failures are non-fatal (caller-observable).
- Coverage ≥80%.

## Risks

- **Production Redis password requirement**: per repo CLAUDE.md, prod Redis requires `REDIS_PASSWORD`. The infrastructure package should already handle this; verify in T051.
- **Pub/sub durability**: by design, Redis pub/sub is ephemeral. Subscribers that miss messages fall back to ES query (NFR-004). Document this.
- **Channel name conflict**: cross-checked in research; verify again at PR review by grepping for any other `community_alerts:` usage in the publisher service or other NC services.

## Reviewer Guidance

- Verify no direct redis client import (must go through `infrastructure/redis`).
- Verify the JSON payload matches the schema.
- Verify the channel name is the one specified in `contracts/redis-channels.md`.
- Verify error propagation does not cause callers to roll back ES writes.

## Implementation Command

```bash
spec-kitty agent action implement WP12 --agent <name>
```

Depends on WP05, WP06.

## Activity Log

- 2026-05-06T23:12:28Z – claude:sonnet:implementer:implementer – shell_pid=263484 – Started implementation via action command
- 2026-05-06T23:21:25Z – claude:sonnet:implementer:implementer – shell_pid=263484 – Redis publisher complete with tests
- 2026-05-06T23:21:50Z – claude:opus:reviewer:reviewer – shell_pid=268677 – Started review via action command
