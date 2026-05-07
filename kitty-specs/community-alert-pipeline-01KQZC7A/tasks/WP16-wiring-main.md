---
work_package_id: WP16
title: Wiring in main.go
dependencies:
- WP15
requirement_refs:
- C-001
- C-008
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T067
- T068
- T069
- T070
phase: B
agent: "claude:opus:reviewer:reviewer"
shell_pid: "339519"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: alert-crawler/main.go
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- alert-crawler/main.go
- alert-crawler/main_test.go
priority: P1
tags: []
---

# WP16 — Wiring in main.go

## Objective

Flesh out `main.go` with the full phase-order wiring: flags → setup (config, logger, sqlite, ES, redis) → buildSources → runner → exit. Add `signal.NotifyContext(SIGINT, SIGTERM)` for graceful shutdown. Mirrors signal-crawler's pattern (R-002).

## Context

- Plan §Component Design (manual wiring), §Phased Build Sequence Phase B.12
- Research R-002 (signal-crawler phase order)
- Spec §5 C-001, C-008

## Branch Strategy

Standard. Depends on WP15 (and transitively on all of Phase B).

## Subtasks

### T067 — Wire all dependencies in `main.go`

**Purpose**: Replace the WP05 stub with full dependency construction.

**Steps**:
1. Replace `alert-crawler/main.go` content with:
   ```go
   package main

   import (
       "context"
       "flag"
       "os"
       "os/signal"
       "syscall"
       "time"

       infraconfig "github.com/jonesrussell/north-cloud/infrastructure/config"
       infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"

       "github.com/jonesrussell/north-cloud/alert-crawler/internal/catalogue"
       "github.com/jonesrussell/north-cloud/alert-crawler/internal/config"
       "github.com/jonesrussell/north-cloud/alert-crawler/internal/elasticsearch"
       "github.com/jonesrussell/north-cloud/alert-crawler/internal/observability"
       "github.com/jonesrussell/north-cloud/alert-crawler/internal/redis"
       "github.com/jonesrussell/north-cloud/alert-crawler/internal/runner"
       "github.com/jonesrussell/north-cloud/alert-crawler/internal/scope"
       "github.com/jonesrussell/north-cloud/alert-crawler/internal/severity"
   )

   func main() {
       configPath := flag.String("config", "/etc/alert-crawler/config.yml", "path to config.yml")
       backfill := flag.Bool("backfill", false, "run in backfill mode (top-20 from feed; emit created events)")
       flag.Parse()

       ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
       defer stop()

       log, err := infralogger.New(infralogger.Config{Level: "info", Format: "json", Service: "alert-crawler"})
       if err != nil {
           panic(err)
       }
       defer log.Sync()

       cfg, err := config.Load(*configPath)
       if err != nil {
           log.Error("config load failed", infralogger.Error(err))
           os.Exit(1)
       }

       store, err := catalogue.Open(ctx, cfg.Database.Path)
       if err != nil {
           log.Error("catalogue open failed", infralogger.Error(err))
           os.Exit(1)
       }
       defer store.Close()

       indexer := elasticsearch.New(elasticsearch.Config{
           BaseURL: cfg.Elasticsearch.URL,
           Index:   cfg.Elasticsearch.Index,
       })
       if err := indexer.EnsureIndex(ctx); err != nil {
           log.Error("ensure index failed", infralogger.Error(err))
           os.Exit(1)
       }

       pub, err := redis.New(redis.Config{URL: cfg.Redis.URL, Channel: cfg.Redis.Channel})
       if err != nil {
           log.Error("redis publisher failed", infralogger.Error(err))
           os.Exit(1)
       }
       defer pub.Close()

       metrics := observability.New(log)
       resolver := scope.New()
       sevTable := severity.NewTable(cfg.Severity.Table)

       fetcher := /* rss.New(...) */

       r := runner.New(runner.Dependencies{
           Fetch:    fetcher,
           Store:    store,
           Indexer:  indexer,
           Pub:      pub,
           Resolver: resolver,
           SevTable: sevTable,
           Metrics:  metrics,
           Sources:  cfg.Sources,
       })

       runStart := time.Now()
       if *backfill {
           if err := r.Backfill(ctx); err != nil { /* ... */ }
       } else {
           if err := r.Run(ctx); err != nil { /* log; exit non-zero */ }
       }
       log.Info("alert-crawler exit", infralogger.Int64("duration_ms", time.Since(runStart).Milliseconds()))
   }
   ```

**Files**:
- `alert-crawler/main.go` (rewritten, ~120 lines).

### T068 — `signal.NotifyContext` for graceful shutdown

**Purpose**: SIGINT/SIGTERM cancels the context cleanly so in-flight ES/Redis ops can exit and the SQLite store can flush.

**Steps**:
1. Already in T067's main.go pattern.
2. Verify all downstream calls accept and honor the context:
   - Indexer (uses `http.NewRequestWithContext` per WP11).
   - Catalogue store (uses `*Context` SQL methods per WP10).
   - Redis publisher (passes context to `Publish` per WP12).
   - RSS client (uses `http.NewRequestWithContext` per WP08).
3. Document in CLAUDE.md: a SIGTERM mid-poll causes the current cycle to abort; idempotency (NFR-006) ensures the next run will reconcile.

**Files**:
- `alert-crawler/main.go` (already from T067).

### T069 — Phase order matches signal-crawler

**Purpose**: Match the convention so ops/CI patterns translate.

**Steps**:
1. Verify the order in main.go is:
   1. flags
   2. context with signal handler
   3. logger
   4. config
   5. catalogue (sqlite)
   6. elasticsearch (indexer + EnsureIndex)
   7. redis publisher
   8. metrics, resolver, severity table, fetcher
   9. runner
   10. r.Run(ctx) or r.Backfill(ctx)
   11. defer cleanup (Close, Sync)
2. Document the order at the top of main.go as a comment block.

**Files**:
- `alert-crawler/main.go` (already from T067; add comment block).

### T070 — Smoke test

**Purpose**: A test that boots the runner against fixtures (httptest for ES + RSS, mock redis), polls once, exits cleanly. Catches wiring bugs.

**Steps**:
1. Create `alert-crawler/main_test.go`:
   - **TestSmoke_BootAndRunOnce**: spin up httptest servers for ES (mocked behavior) and RSS (returns golden fixture); set env vars to point alert-crawler at them; run a one-shot via the runner package directly (not via the binary); assert: ES received the expected number of writes; mock Redis received the expected events; SQLite has the expected catalogue entries.
   - This test exercises the full Phase B integration without requiring a real ES/Redis cluster — that's WP18's job. WP16's smoke test is a "wiring sanity check".
2. The smoke test runs as part of `task test:alert-crawler` (no integration build tag).

**Files**:
- `alert-crawler/main_test.go` (new, ~150 lines).

**Validation**:
- Smoke test passes.
- `task run` (with appropriate env or `--config` flag pointing at a test config) actually runs end-to-end.

## Definition of Done

- main.go wires every dependency.
- SIGINT/SIGTERM handling works.
- Phase order matches signal-crawler.
- Smoke test passes.

## Risks

- **Logger initialization**: ensure no logging before logger is initialized (or use a fallback `log.Println` for that early window).
- **Config error fatal at startup**: design choice — fail fast on config load error (preferable for oneshot pattern).
- **Resource leaks**: ensure all Closers are deferred in the right order.

## Reviewer Guidance

- Verify deferred cleanup order is reverse-construction (LIFO).
- Verify the smoke test is not silently passing (assert specific counts).
- Verify `task run` works against a local docker-compose dev stack.

## Implementation Command

```bash
spec-kitty agent action implement WP16 --agent <name>
```

Depends on WP15.

## Activity Log

- 2026-05-06T23:57:24Z – claude:sonnet:implementer:implementer – shell_pid=333028 – Started implementation via action command
- 2026-05-07T00:02:23Z – claude:sonnet:implementer:implementer – shell_pid=333028 – main.go wired with full dep graph + smoke test
- 2026-05-07T00:02:57Z – claude:opus:reviewer:reviewer – shell_pid=339519 – Started review via action command
- 2026-05-07T00:04:14Z – claude:opus:reviewer:reviewer – shell_pid=339519 – Wiring approved: full dep graph, SIGINT/SIGTERM, smoke test passes, 11 packages green
