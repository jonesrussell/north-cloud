# NorthCloud Roadmap

Living document tracking the NorthCloud platform milestone sequence.
For historical design documents, see `docs/plans/`.

---

## Milestone Sequence

```
M0: Architecture Review     ← CURRENT
 ↓
M1: Smart Extraction
 ↓
M2: Dynamic Crawling
 ↓  (can run parallel with M2)
M3: Observability Hardening
 ↓
M4: Contract Formalization
 ↓
M5: Product Layer (v0.6+)
```

---

## M0: Architecture Review & Versioning

**Goal:** Codify the current architecture into a versioned, governed platform.

**Scope:**
- Platform version (VERSION file + CHANGELOG)
- Living roadmap (this document)
- Spec drift audit and fix across 8 subsystem specs
- GitHub governance (milestones, labels, issue templates)
- Go module path cleanup (consistency across 15 services)
- Service dependency map

**Success criteria:** Every spec verified against code. Platform has a version. Roadmap is documented.

**Size:** Small — documentation only (except module paths)

**Status:** In progress. Design: `docs/plans/2026-03-08-m0-architecture-review-design.md`

---

## M1: Smart Extraction

**Goal:** Make extraction page-type aware and source-aware, fixing 75% of broken sources.

**Scope:**
- URL pre-filter (skip PDFs, CDNs, off-domain, store pages)
- Page type classifier (article vs listing vs stub vs other)
- CMS template registry (Postmedia, Torstar, WordPress, Drupal, Village Media, Black Press)
- Enhanced generic extraction (text density heuristic + readability default)
- Extraction quality metrics and Grafana dashboard
- Real extraction test endpoint (replace mock test_source)
- Extraction regression suite
- Backfill and validation of top 20 worst sources

**Success criteria:** Word count > 0 for >= 60% of raw_content docs (up from current 25%).

**Dependencies:** M0 (architecture codified first)

**Size:** Large — 8 tasks across crawler and source-manager

**Status:** Designed. Design: `docs/plans/2026-03-08-m1-smart-extraction-design.md`

---

## M2: Dynamic Crawling

**Goal:** Enable crawling of JavaScript-rendered sites via headless browser.

**Scope:**
- Headless browser service (Playwright/Chrome CDP)
- Sandbox isolation per crawl
- Per-source dynamic config (wait-for selectors, scroll depth, cookie acceptance)
- Observability (render time, JS errors, crash rate)
- Fallback chain: dynamic → static → feed → AMP
- Integration with existing crawler routing

**Success criteria:** Successfully crawl and extract content from 5+ JS-rendered sources.

**Dependencies:** M0, M1 (or at least M1 design)

**Size:** Large — new service + infrastructure

**Status:** Placeholder. See GitHub milestone M2.

---

## M3: Observability Hardening

**Goal:** Define SLAs, build alerting, create incident runbooks.

**Scope:**
- SLA/SLO targets per service (uptime%, P99 latency, error rate)
- Alerting rules in Grafana
- Incident response runbooks for top 10 failure modes
- Post-deploy smoke tests in CI
- Backup/restore procedures

**Success criteria:** Every service has a defined SLO. Alerts fire within 5 minutes of SLO breach.

**Dependencies:** Can run parallel with M2

**Size:** Medium

**Status:** Not started

---

## M4: Contract Formalization

**Goal:** Machine-readable API contracts for all services.

**Scope:**
- OpenAPI 3.1 specs for all HTTP APIs
- API versioning strategy and deprecation policy
- Request tracing headers standard
- SDK generation (optional)

**Success criteria:** Every service has an OpenAPI spec. Breaking changes follow deprecation policy.

**Dependencies:** M0

**Size:** Large

**Status:** Not started

---

## M5: Product Layer (v0.6+)

**Goal:** User-facing features and search experience improvements.

**Scope:**
- GIS and community indexing (existing v0.6 milestone)
- Search UX improvements
- New content types
- People directory

**Dependencies:** M1 (extraction must work before adding more content types)

**Size:** Large

**Status:** Partially started (v0.6 GIS milestone exists with 7 issues)

---

## Historical Plans

Design documents in `docs/plans/` are historical records from past development sessions.
They are not actively maintained. For current planning, use this roadmap and the GitHub milestones.
