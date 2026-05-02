# WP03 — `playwright-renderer/` triage

**Mission**: `audit-dead-code-removal-01KQG57Z`
**Issue**: #699
**Spec refs**: FR-004, FR-005, FR-007 (FR-004 conditional path)
**Dependencies**: WP01, WP02

## Outcome: documented, not deleted

The 2026-04-30 audit's "no compose entry, no consumer" claim for `playwright-renderer/` was **incorrect**. The service is live in production with two consumers. Per spec FR-004's conditional clause (*"If unused (no compose entry, no service consumes its output), delete. Otherwise document and add to compose."*), this WP takes the **document the keeper** path. Adds `playwright-renderer/README.md` and `playwright-renderer/CLAUDE.md`. **No directory is deleted. No compose changed.**

## Evidence the audit was wrong

| Claim in audit | Reality |
|---|---|
| "Parallel to active `render-worker/`, no README, no `CLAUDE.md`, not in compose" | `render-worker/` parallel: true. No README/CLAUDE.md: true (addressed by this WP). **Not in compose: false** — present in `docker-compose.prod.yml` with full service definition (image, healthcheck, deploy resources). |
| Implicit "no consumer" | False — two prod consumers reference it. |

Verifying greps performed in this WP's worktree (origin/main HEAD `4ba80005`):

```bash
grep -l playwright-renderer docker-compose*.yml
# → docker-compose.prod.yml (only)

grep -B1 -A20 'playwright-renderer:' docker-compose.prod.yml
# → full service definition at line 566:
#     playwright-renderer:
#       image: docker.io/jonesrussell/playwright-renderer:${PLAYWRIGHT_RENDERER_TAG:-latest}
#       healthcheck: GET http://127.0.0.1:8095/health every 30s
#       deploy: 1.0 CPU, 1G memory

grep -B1 -A2 'RENDERER_URL' docker-compose.prod.yml
# → mcp-north-cloud line 560: RENDERER_URL: "http://playwright-renderer:8095"
# → signal-crawler line 829:  RENDERER_URL=http://playwright-renderer:8095, RENDERER_ENABLED=true
```

The audit likely scanned `docker-compose.base.yml` only and concluded "not in compose". The service is prod-only and not present in base/dev compose (use `render-worker/` for local rendering experiments).

## Why `playwright-renderer/` and `render-worker/` both exist

The audit also implied `playwright-renderer/` is redundant with the active `render-worker/`. They are not redundant — they serve different roles:

| Aspect | `render-worker/` | `playwright-renderer/` |
|---|---|---|
| Port | 3000 | 8095 |
| Container name | `north-cloud-render-worker` | `playwright-renderer` |
| Built from | local Dockerfile (in base + prod compose) | published image `docker.io/jonesrussell/playwright-renderer` (prod compose only) |
| Server | http stdlib + custom queue | express + simple concurrency limit |
| Stealth / scroll | yes (`stealth.js`, `scroll.js`) | no |
| Browser recycling | every 100 requests | persistent (no recycle) |
| Resource blocking | unknown (handler-driven) | yes (image/media/font/CSS aborted) |
| Concurrency | tab queue, max 1 | reject (503) at 3 in-flight |
| Used by | crawler's main pipeline | mcp-north-cloud `fetch_url` + signal-crawler |
| Design goal | sustained throughput inside crawl loop | on-demand rendering for ad-hoc consumers |

Consolidating them is possible but would require migrating both consumers and reworking the crawler-side queue logic — out of scope for an audit deletion WP. Documented as a possible follow-up.

## Changes in this WP

| File | Action |
|---|---|
| `playwright-renderer/README.md` | **New** — purpose, consumers, API surface, configuration, deploy, "not the same as `render-worker/`" note, audit revision note |
| `playwright-renderer/CLAUDE.md` | **New** — agent-facing quick-reference for editing this directory; cites the two consumers and the boundary with `render-worker/` |
| `kitty-specs/audit-dead-code-removal-01KQG57Z/tasks/WP03-playwright-renderer-triage.md` | **New** — this prompt file with full evidence |

No code, compose, Taskfile, or workflow changed.

## Out of scope (possible follow-ups)

- **Add `playwright-renderer` to root `CLAUDE.md` orchestration table** — would simplify discovery. DIR-004 ("no while-I'm-here") declines doing it as part of this WP.
- **Consolidate `playwright-renderer` and `render-worker`** — explicitly evaluated and rejected (different design goals; would require migrating two consumers + crawler queue logic).
- **Add `docs/specs/playwright-renderer.md`** — the service is small with a single API surface; a spec entry isn't warranted yet.

## Verification

- `task drift:check` — green (no spec touched)
- `task layers:check` — green (no Go imports changed)
- `task ports:check` — green (no compose changed)
- No Go code touched → no `task lint` / `task test` deltas

## Acceptance

- Both new docs land on `main`.
- `playwright-renderer/` remains unchanged in code.
- Issue #699 closed via `Closes #699`, with body explaining the audit revision and parallel to WP02.

## Mission A retrospective (with WP01–03 complete)

| WP | Issue | Audit's claim | Reality | Action taken |
|---|---|---|---|---|
| WP01 | #697 dashboard-waaseyaa | Orphaned PHP/Laravel rewrite | Confirmed orphaned (generic Waaseyaa CMS skeleton) | Deleted (96 files, ~14323 LOC) |
| WP02 | #698 ml-modules + ml-framework | Orphaned siblings to ml-sidecars | Wrong — active 3-tier ML architecture | Documented (2 READMEs) |
| WP03 | #699 playwright-renderer | "Not in compose, no consumer" | Wrong — prod compose service with 2 consumers | Documented (README + CLAUDE.md) |

Mission A successfully removed one true orphan (dashboard-waaseyaa) and corrected two audit errors with documentation. Two-thirds of the audit's flagged "dead code" turned out to be live code lacking READMEs.
