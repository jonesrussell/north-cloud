# Research — Community Alert Pipeline

**Mission ID**: `01KQZC7A7SJJZ6EKHZ9JW3AZJG` (mid8: `01KQZC7A`)
**Mission Slug**: `community-alert-pipeline-01KQZC7A`
**Date**: 2026-05-06
**Spec**: [spec.md](./spec.md)
**Phase**: Research (Phase 0)

---

## Purpose

This document resolves the four research items deferred from spec to research phase (`R-001` through `R-004`) and surfaces the new decisions, risks, and spec-update recommendations that emerged. It is the grounded input for the upcoming plan phase.

---

## R-001 — safersites.ca Feed Availability

### Decision

**Feed-primary acquisition. Binding.** Use the RSS 2.0 feed at `https://www.safersites.ca/drugalerts.rss`. HTML scraping is no longer the fallback path; it is excluded from this mission's plan. If Cloudflare or the source ever takes the feed offline, fallback to a Playwright-based scraper becomes a separate spec amendment, not a v1 implementation concern.

### Evidence

- Live HTTP probe confirms `https://www.safersites.ca/drugalerts.rss` returns `200 OK`, `Content-Type: application/rss+xml; charset=utf-8`, ~55 KB payload, 20 items per page.
- The feed is auto-discovered via `<link rel="alternate" type="application/rss+xml" href="https://www.safersites.ca/drugalerts.rss" title="RSS 2.0">` in the `<head>` of `/drugalerts`. It is **not** at any standard probe path (`/feed`, `/rss.xml`, `/atom.xml`).
- Each `<item>` element carries: `<title>`, `<description>` (full HTML body with date, location, substance, composition percentages, PDF handbill link, photo URLs), `<pubDate>` (RFC-822 with timezone), `<author>`, `<link>`. **No `<guid>` element** is present.
- Sample item: `title="Drug Alert: Winnipeg – Fentanyl – Tue. May 5, 2026"`, `pubDate="Tue, 05 May 2026 17:50:31 -0500"`, `link="http://www.safersites.ca/20260505fentanyl"`. URL pattern: `/YYYYMMDD{substance}`.
- Cache headers: `ETag` present (weak, e.g. `W/"031..."`), no `Last-Modified`, `Cache-Control: public, max-age=14400` (4-hour server-side cache).
- No JSON API. Probes of `/api`, `/api/alerts`, `/api/v1/alerts` return `404`; `/wp-json/` returns `403`.
- CMS is **NationBuilder** (confirmed by asset hosts, form action `/forms/signups`, themed origin `safersites.nationbuilder.com`). NationBuilder does not expose a public REST API, so feed parsing is the only structured ingestion path.
- `/sitemap.xml`, `/sitemap_index.xml`, and `/robots.txt` all return `403` to non-browser agents (Cloudflare Bot Management). The RSS feed itself bypasses Cloudflare bot-block, signaling the site's intent for the feed to be machine-consumed.
- Email subscription at `/join` is backed by NationBuilder native email (not Mailchimp/Substack/GovDelivery). No third-party feed surface to substitute for the RSS.

### Implications for plan and code

- **Polling**: every 30 to 60 minutes against `https://www.safersites.ca/drugalerts.rss` with `If-None-Match` (ETag conditional GET). Most polls will return `304 Not Modified` because of the 4-hour server cache.
- **Stable identifier**: the canonical `<link>` URL is the alert ID. Hash or URL-derive a slug suitable for ES `_id` and Redis payload references. **No `<guid>` is available** (FR-006 needs to acknowledge this).
- **Pubdate / issued_at**: parse `<pubDate>` as RFC-822 with timezone. Map to envelope `issued_at`.
- **Structured payload**: `<description>` is HTML-encoded. Plan must include an HTML-to-fields parser (substance composition list, location, severity inference, handbill PDF URL on `assets.nationbuilder.com`). Because NationBuilder content is templated, this is stable across alerts of the same kind. The substance composition follows a predictable pattern (`Substance (XX.X%)`).
- **No explicit rescission signal**: an alert disappearing from the feed is the only "rescinded" indicator. The implementation must keep its own catalog of currently-active alerts and detect disappearance as a delta. This is fundamental to satisfying FR-008 / AS-03.
- **No Cloudflare bot challenge on the feed itself**: simple stdlib HTTP + ETag handling is sufficient.
- **`expires_at` is not in the feed**: the source does not publish an explicit expiry. The alert-crawler must derive a default expiry policy (recommendation: 30 days from `issued_at`, configurable per source) and surface a clarification to the spec on whether expiry semantics should match upstream removal time when the alert disappears from the feed before its calculated expiry.

### Open RSS gaps (carried forward)

- **Pagination**: the feed shows 20 items; the listing page has 10 pages. It is unconfirmed whether `?page=N` is honoured on the RSS endpoint. Plan must verify before initial backfill or accept "20 most recent" as the historical window.
- **Feed item ordering**: confirmed reverse-chronological by observation; not contractually guaranteed by NationBuilder.

---

## R-002 — signal-crawler Operational Surface

### Decision

**Mirror signal-crawler's deployment topology** (oneshot binary, systemd timer, Docker Compose `restart: "no"`, multi-stage Alpine Dockerfile, uid 1000 non-root user, `.layers`-enforced 3-tier internal package model). **Do not adopt** signal-crawler's HTTP POST publish path; alert-crawler publishes to Elasticsearch and Redis instead.

> **Caveat**: signal-crawler is in maintenance mode (`signal-crawler/CLAUDE.md` notes new adapters go to `signal-producer`, issue #592). Spec C-007 references "signal-crawler's deployment topology", which both `signal-crawler` and the successor `signal-producer` share (oneshot, systemd timer). The spec wording is fine; the plan should explicitly note that operational mirroring applies to the topology pattern, not to signal-crawler-specific code.

### Evidence

- **Process model**: oneshot. `main.go` calls `r.Run(ctx)` once, iterates adapters synchronously, exits. Context wired to `SIGINT`/`SIGTERM` via `signal.NotifyContext`. No ticker, no daemon loop.
- **Config**: YAML + env, unified through `infrastructure/config.LoadWithDefaults[T]`. Precedence is `env > YAML > SetDefaults`. **Pitfall**: a non-empty YAML field silently wins over `SetDefaults`. Plan must avoid writing real defaults into `config.yml` for any field whose default lives in `SetDefaults`.
- **Source registration**: hardcoded in `buildSources()` in `main.go`. No source-manager integration. Single sequential pass per invocation; cadence is the systemd timer frequency.
- **State / dedup**: SQLite store at `data/seen.db`, single `seen(source TEXT, external_id TEXT)` table. Volume-mounted to persist across container runs. CGO is required for `go-sqlite3`.
- **Lifecycle**: no Uber FX. Manual wiring in `main.go`. Phase order: flags → setup (config, logger, dedup) → buildSources → ingest client → runner → run → stats → exit.
- **Build / Taskfile**: per-service `Taskfile.yml` with targets `build`, `run`, `run:dry`, `recall:check`, `test`, `test:cover`, `lint`, `lint:no-cache`, `vuln`, `clean`. Lint uses root `.golangci.yml` via `--config ../.golangci.yml`.
- **Docker**: multi-stage `golang:1.26.2-alpine` builder + `alpine:3.21` runtime. Non-root user uid/gid 1000 (`signal-crawler`). `config.yml` baked into the image.
- **Compose**: `docker-compose.base.yml` entry has `restart: "no"`, network `north-cloud-network`, named volume `signal-crawler-data:/app/data`, **no health check**.
- **Deploy**: `scripts/deploy.sh` lists the service in the oneshot skip list (no health endpoint check).
- **Ansible / systemd**: role `northcloud-ansible/roles/north-cloud`. Two templates: `signal-crawler.timer.j2` (`OnCalendar={{ nc_signal_crawler_schedule }}`, default `*-*-* 06:00:00 UTC`, `Persistent=true`, `RandomizedDelaySec=300`) and `signal-crawler.service.j2` (`Type=oneshot`, `ExecStartPre` pulls image, `ExecStart` runs `docker compose ... run --rm signal-crawler`). No `User=` directive; runs as `deploy_user` on the host. Inside the container, runs as uid 1000.
- **`.layers`** enforces 3 tiers: L0 (`config`, `scoring`), L1 (`adapter`, `dedup`, `ingest`, `render`), L2 (`runner`). No `allow` exceptions. Clean DAG.
- **Lead spec at `docs/specs/lead-pipeline.md`** is the authoritative cross-producer contract; the `Signal` struct lives in `infrastructure/signal/` and is shared. Alert-crawler does **not** share this struct (different domain), but should use `infrastructure/redis`, `infrastructure/logger`, `infrastructure/config`, and any other shared infra packages.

### Implications for plan and code

- `alert-crawler/` directory layout: `main.go`, `internal/{config,adapter,catalog,publisher,ingestor,runner}`, plus `Taskfile.yml`, `Dockerfile`, `config.yml`, `.layers`.
- Cadence: parameterize via Ansible `nc_alert_crawler_schedule` (default e.g. `*-*-* *:30:00 UTC` for hourly half-past, with `RandomizedDelaySec=120` to spread load).
- Add the service to `GO_SERVICES` in `scripts/detect-changed-services.sh`, the oneshot skip list in `scripts/deploy.sh`, and add a `vuln:alert-crawler` delegation in the root `Taskfile.yml`.
- Volume ownership: if the container user is uid 1000, Ansible `file:` tasks for any data directory must use `owner: "1000"`, not `deploy_user` (uid 1001).
- Layer model: at least L0 (`config`), L1 (`adapter`, `catalog`, `publisher`, `elasticsearch`), L2 (`ingestor`, `runner`).

---

## R-003 — Classifier / Publisher Bypass Pattern

### Decision

**Mirror rfp-ingestor's classifier/publisher bypass** (write directly to Elasticsearch; downstream consumers query ES; the `*_classified_content` wildcard is the integration seam). **Diverge from rfp-ingestor on three points** that the spec demands:

1. **Index family**: do NOT use `*_classified_content`. Use a fresh family `community_alerts` (or `community_alert_events`). Alerts are not articles; they have a lifecycle, not a quality score; consumers should not have to filter by `content_type == "alert"` inside an article-shaped index.
2. **Lifecycle events**: rfp-ingestor has none. Alert-crawler MUST publish to Redis on every `created` / `updated` / `rescinded` transition. ES is the canonical store; Redis is the live notification bus.
3. **Process model**: rfp-ingestor is a long-running daemon; alert-crawler is a oneshot per `C-007`. Use rfp-ingestor's ES client and indexing approach, but wrap it in signal-crawler's oneshot lifecycle.

### Evidence

- rfp-ingestor writes via `elasticsearch.Indexer.BulkIndex` (raw `POST /_bulk` NDJSON, batched by `bulkSize` default 500). The ES client is plain `net/http.Client`. No ES Go SDK dependency.
- `EnsureIndex` does HEAD check on startup; if 404, issues `PUT /{index}` with the mapping JSON. Mapping is in `rfp-ingestor/internal/elasticsearch/mapping.go`. **No `infrastructure/esmapping` or `index-manager` involvement.**
- Index name: `rfp_classified_content` (env `ES_RFP_INDEX`). Search service queries the `*_classified_content` wildcard, so naming alone is the registration.
- **`content_type` MUST be `text` with an explicit `.keyword` sub-field** because the search service queries `content_type.keyword`. This is a search-side coupling worth knowing even if alert-crawler does not use the same family.
- rfp-ingestor publishes nothing to Redis. Publisher Layer 11 reads `content_type == "rfp"` from the search index and routes downstream — this is a separate, asynchronous step that alert-crawler does not require.
- `RFPDocument` envelope: `title`, `url`, `source_name`, `content_type`, `quality_score`, `snippet`, `raw_text`, `topics`, `crawled_at`, plus a nested `rfp` object with domain-specific fields.
- Status API: `GET /api/v1/status` returns last run, fetched/indexed/failed counts, duration. **No Prometheus metrics**, only Zap structured logs.
- `docs/specs/rfp-ingestor.md` rationalizes the bypass: pre-structured government data with known schema; classification adds no value when document type, topic, and quality are determinable at parse time. **The same logic applies to community alerts**: the issuing organization has already classified the alert; structural routing by category is correct.

### Implications for plan and code

- ES client: copy rfp-ingestor's pattern (`internal/elasticsearch/{indexer.go,mapping.go}`), raw-HTTP, bulk-only or single-doc as appropriate. Alert volume is far lower than RFPs, so individual `Index` calls per state change are acceptable; bulk is unnecessary unless backfilling.
- Index name: `community_alerts` (lowercase, snake-case to match existing NC convention).
- Mapping owned by alert-crawler in `internal/elasticsearch/mapping.go`. `EnsureIndex` on startup. Idempotent.
- ES write is synchronous and authoritative. Redis publish happens **after** a successful ES write, as a side effect. A Redis publish failure does not roll back the ES write; it produces an observability signal but the alert is durably stored.
- Add Prometheus or structured-metrics observability **from day 1**, given the safety-critical context. Do not adopt rfp-ingestor's "Zap-only" posture.
- Define explicit lifecycle event payloads: `event_type`, `alert_id`, `payload` (the full envelope or a reference), `event_timestamp`. Channel name follows existing NC conventions; final name resolved in plan.

---

## R-004 — Indigenous-Taxonomy Extension Scope

### Decision

**Bump `jonesrussell/indigenous-taxonomy` to v1.1.0** with additive-only changes. The mission scope includes:

1. New `Treaty` namespace (11 constants, slugs `treaty:1` through `treaty:11`).
2. City-level region children under existing province nodes (`canada:manitoba:winnipeg`, etc.).
3. At least one sample First Nations community slug per treaty area to establish the pattern (e.g., `canada:manitoba:sagkeeng-fn`).
4. A new `ParentRegion(Region) (Region, bool)` walk function generated from existing YAML `children:` data.
5. Alert-crawler imports `github.com/jonesrussell/indigenous-taxonomy/generated/go/taxonomy` directly. The hand-rolled shim at `north-cloud/infrastructure/indigenous/region.go` is **not extended in this mission**; migrating classifier and publisher to the package is a separate backlog item.

### Evidence

- Package surface: three namespaces with type aliases. `Category` (10 constants, slugs `culture`, `language`, `land-rights`, `environment`, `sovereignty`, `education`, `health`, `justice`, `history`, `community`). `Region` (16 constants, colon-delimited hierarchy in slug names, e.g. `canada:manitoba:southern`). `DialectCode` (10 entries, struct type).
- Hierarchy is **only** implicit in slug colon-delimitation. **No `ParentRegion`, `Ancestors`, or `IsChildOf` function** exists in the generated Go output. The YAML source file `schema/regions.yaml` has a `children:` field, but `scripts/generate.py` does not emit a parent-lookup table.
- **Treaty territories: entirely absent.** No First Nations / Métis / Inuit community-level slugs. No urban Indigenous city designations. National scope is `canada` (no separate `all`).
- **Critical surprise**: `grep -r 'indigenous-taxonomy' /home/jones/dev/north-cloud/ --include='*.go'` returned zero hits. North-cloud is **not currently importing** the package. Classifier and publisher use a hand-rolled shim at `north-cloud/infrastructure/indigenous/region.go` with seven continental-scale slugs (`canada`, `us`, etc.). The CLAUDE.md mandate ("NC classifier must import category/region slugs from `jonesrussell/indigenous-taxonomy` Go package") is aspirational and not yet enforced in code.
- Versioning convention from the package's CLAUDE.md: adding a category/region/dialect = minor bump; renaming/removing = major bump.

### Implications for plan and code

- Plan must include a sub-package addition step: `schema/treaties.yaml`, generated `taxonomy/treaties.go`, and city/community children added to `schema/regions.yaml`, plus regeneration via `scripts/generate.py`.
- `generate.py` extension is needed: emit a `ParentRegion()` lookup table (or `ancestors()` walker) from existing YAML `children:` relationships. Plan must scope this code change.
- Audit the package's tests for any hardcoded `len(AllRegions)` or counter assertions before adding new region entries.
- Decision flagged for the spec: alert-crawler imports the package directly. The hand-rolled NC shim is not the source of truth for alert-crawler's scope vocabulary. Spec C-003 already binds scope to the package; this research confirms direct import is the correct interpretation.

### What is **out of scope** for this mission

- Migrating classifier and publisher to import `indigenous-taxonomy` directly. (Separate backlog item; flag at end of mission.)
- Filling in the entire community-slug catalogue for Canada. (Establish pattern with one or two communities per treaty area.)
- Métis Settlements full enumeration, Inuit nunangat boundaries, modern-treaty territories beyond Numbered Treaties 1-11. (Future expansion.)
- Follow-on tracker: [#717](https://github.com/jonesrussell/north-cloud/issues/717).

---

## New Decisions Surfaced (cross-cutting)

| ID | Decision | Source |
|---|---|---|
| D-001 | Acquisition is RSS feed only; HTML scraping deferred until/unless feed becomes unavailable | R-001 |
| D-002 | Stable alert ID derives from `<link>` URL (no `<guid>` in feed) | R-001 |
| D-003 | Rescission detection requires alert-crawler to maintain its own active-alert catalogue and diff against feed contents per poll | R-001 |
| D-004 | `expires_at` is not provided by upstream; alert-crawler computes a default (e.g., 30 days from `issued_at`), spec amendment recommended | R-001 |
| D-005 | Process model: oneshot binary, systemd timer, manual wiring (no Uber FX) | R-002 |
| D-006 | Persistent state via SQLite (mirroring signal-crawler's dedup pattern), CGO required | R-002 |
| D-007 | ES index family: `community_alerts` (NEW), not `*_classified_content` | R-003 |
| D-008 | ES mapping owned in `alert-crawler/internal/elasticsearch/mapping.go`, applied via idempotent `EnsureIndex` on startup | R-003 |
| D-009 | ES write is synchronous and authoritative; Redis publish is post-write side effect | R-003 |
| D-010 | Add structured metrics from day 1 (do not copy rfp-ingestor's Zap-only posture) | R-003 |
| D-011 | Alert-crawler imports `jonesrussell/indigenous-taxonomy` directly; existing NC shim untouched | R-004 |
| D-012 | Indigenous-taxonomy v1.1.0 adds Treaty namespace, city regions, sample community slugs, `ParentRegion` walk | R-004 |

---

## New Risks Surfaced (cross-cutting)

| ID | Risk | Mitigation |
|---|---|---|
| RR-001 | Cloudflare may eventually block RSS feed access from the alert-crawler container | Monitor; document Playwright-based scraping as a deferred fallback if the feed becomes inaccessible |
| RR-002 | NationBuilder template changes alter `<description>` HTML structure, breaking the parser | Defensive parsing (look for known section markers; if missing, surface a parse failure event but persist the unstructured alert with a `parse_quality: degraded` flag) |
| RR-003 | RSS pagination behaviour unconfirmed; backfill may be limited to the 20 most recent items | Plan must verify `?page=N` support before designing initial backfill; accept "rolling-20 historical window" as the v1 contract if pagination is unsupported |
| RR-004 | Indigenous-taxonomy package tests may have hardcoded counter assertions that break with additive changes | Audit `tests/` in the taxonomy repo before opening the v1.1.0 PR |
| RR-005 | The aspirational mandate "NC must import indigenous-taxonomy" is unenforced; alert-crawler will be the first NC service to actually do so, exposing latent integration issues (go.mod drift, replace directives, CI tag pinning) | Plan a small spike to confirm clean import before scaffolding the alert-crawler module |
| RR-006 | Deriving `expires_at` server-side may diverge from issuing organization's intent; some alerts logically expire when the supply changes (days), some when the substance is fully tested (weeks) | Make per-source `expires_at` policy configurable; default to a conservative 30 days; flag for spec amendment |
| RR-007 | signal-crawler's `config.yml`-overrides-`SetDefaults` gotcha will silently regress alert-crawler if a future maintainer copies the pattern wrong | Document explicitly in alert-crawler's CLAUDE.md; consider a unit test that verifies `SetDefaults`-only fields are absent from `config.yml` |

---

## Spec-Update Recommendations

These are surfaced for the maintainer to approve. They are not unilateral edits; the spec at `kitty-specs/community-alert-pipeline-01KQZC7A/spec.md` is the authoritative artifact and remains unmodified by this research phase per DIRECTIVE_010.

| Spec ref | Recommended update | Reason |
|---|---|---|
| FR-006 | Note that the stable identifier is derived from the canonical source URL when no upstream `id` is present | R-001: RSS has no `<guid>` |
| FR-008 | Note that "no longer present in upstream source" detection requires alert-crawler to maintain a local active-alert catalogue and detect feed-deltas | R-001 |
| C-002 | Add "RSS/Atom feed parsing (stdlib `encoding/xml` or `github.com/mmcdole/gofeed`)" to the allowed building blocks | R-001 |
| FR-002 / Key Entities §7.1 | Note `expires_at` may be derived (default 30d from issuance) when upstream does not publish an explicit expiry; per-source policy is configurable | R-001 |
| C-003 | Clarify that alert-crawler imports `github.com/jonesrussell/indigenous-taxonomy` directly; the existing NC hand-rolled shim at `infrastructure/indigenous/region.go` is not the source of truth for alert scope | R-004 |
| C-007 | Optionally clarify "consistent with signal-crawler's deployment topology" applies to the topology pattern (oneshot + systemd timer), not to signal-crawler-specific code; signal-crawler is in maintenance mode and `signal-producer` is the active successor | R-002 |
| Out of Scope | Add explicit out-of-scope: "Migrating classifier and publisher to import `indigenous-taxonomy` directly" | R-004 |
| AS-05 | The `[NEEDS CLARIFICATION]` on staleness indicator can be resolved during plan; default recommendation: no end-user staleness indicator in v1; observability-only via NFR-008 | (Carry-forward from spec validation) |

---

## Open Questions Carrying Forward to Plan

1. **Per-source default expiry policy** — does the maintainer want a fixed 30-day default, or a per-source override (some alerts are short-lived, some persist)?
2. **Rescission semantics on parser-quality-degraded alerts** — if a poll cannot parse an item that was previously published cleanly, do we treat the unparseable item as still-active, rescinded, or "stale parse" for operator review?
3. **Backfill on first deploy** — do we backfill the 20-most-recent items from the feed at first deploy, or start with empty state and ingest from the next polling cycle forward?
4. **Severity inference for harm-reduction alerts** — the source publishes alerts but does not tag a severity level. The plan must include a small mapping (e.g., presence of carfentanil → `critical`; nitazenes → `high`; novel cuts → `medium`). Whose authority defines this mapping (the issuing org, an internal heuristic, a config table)?
5. **Channel naming** — Redis channel name for lifecycle events. Candidate: `community_alerts:lifecycle`. Confirm during plan against existing NC channel conventions in `publisher/`.
6. **Indigenous-taxonomy version pinning** — alert-crawler's `go.mod` will pin a specific tag. Coordinate with the taxonomy repo's release process so the v1.1.0 tag is cut before alert-crawler depends on it.

---

## Phase Outputs

- This document (`research.md`).
- Data model (`data-model.md`).
- Source register (`research/source-register.csv`).
- Evidence log (`research/evidence-log.csv`).

These artifacts are inputs to `/spec-kitty.plan` and inform the work-package decomposition during `/spec-kitty.tasks`.
