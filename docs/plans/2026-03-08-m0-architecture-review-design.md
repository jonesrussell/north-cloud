# M0: Architecture Review & Versioning

**Date:** 2026-03-08
**Status:** Approved
**Goal:** Codify NorthCloud's current architecture into a versioned, governed platform so every future milestone builds on solid ground.

---

## Context

NorthCloud has grown organically from a single crawler to a 17-service content platform. The architecture is documented (ARCHITECTURE.md, 15 service CLAUDE.md files, 8 specs), CI/CD is smart (change detection, parallelized pipelines), and the codebase is well-structured.

But the platform lacks governance scaffolding:

- **No version identity** — "What version is North Cloud?" has no answer
- **No living roadmap** — 104 archived plan docs but no forward-looking document
- **Spec drift** — docs/specs/ files may diverge from implementation
- **Implicit contracts** — services communicate via undocumented HTTP APIs
- **GitHub governance gaps** — one milestone (v0.6 GIS), incomplete labels, no issue templates
- **Module path inconsistency** — `jonesrussell/north-cloud` vs `north-cloud` in go.mod files

M0 fixes these gaps without touching application code. It's a documentation and governance milestone.

---

## What M0 Is NOT

- Not a rewrite or refactor
- Not new features
- Not performance optimization
- Not fixing broken sources (that's M1: Smart Extraction)
- Not adding observability (that's M3)

---

## Production Data That Informed This Design

Before designing M0, we audited production:

- **468 enabled sources** (390 spider, 66 crawl, 12 feed mode)
- **242,965 raw_content documents** across 660 ES indices
- **182,238 (75%) have word_count = 0** — extraction completely fails
- **291 sources** with avg word count < 30

Root cause analysis on the worst-performing sources (Postmedia, Torstar, etc.) revealed that most failures are **selector mismatches on static HTML** — not JS rendering problems. The content IS in the HTML; the CSS selectors just don't match. This finding shapes the roadmap: Smart Extraction (M1) before Dynamic Crawling (M2).

---

## Tasks

### Task 1 — Platform Version & CHANGELOG

**What:** Establish a single version identity for the NorthCloud platform.

**Deliverables:**
- `VERSION` file at repo root containing `0.5.0`
- `CHANGELOG.md` using [keepachangelog](https://keepachangelog.com/) format
- Backfill key changes from git history and docs/plans/ archive
- Document versioning strategy in ARCHITECTURE.md:
  - Platform uses semver (MAJOR.MINOR.PATCH)
  - Services follow platform version (no per-service versioning)
  - Git tags: `v0.5.0`, `v0.5.1`, etc.
  - MAJOR: breaking API changes, MINOR: new features/milestones, PATCH: bugfixes

**Success criteria:** `cat VERSION` returns a semver string. CHANGELOG covers at least the last 3 months of significant changes.

---

### Task 2 — Roadmap Document

**What:** Replace 104 archived plan docs with a single living roadmap.

**Deliverables:**
- `docs/ROADMAP.md` with milestone sequence:
  - M0: Architecture Review & Versioning (this milestone)
  - M1: Smart Extraction — fix 75% of broken sources via better selectors and fallback extraction
  - M2: Dynamic Crawling — headless browser for true JS-rendered sites
  - M3: Observability Hardening — SLAs, alerting, dashboards, incident runbooks
  - M4: Contract Formalization — OpenAPI specs, API versioning strategy
  - M5: Product Layer (v0.6+) — GIS, community indexing, search UX
- Each milestone includes: goal (1 sentence), scope (bullet list), success criteria, dependencies, estimated size (S/M/L)
- Archive notice added to docs/plans/ explaining these are historical, not active

**Success criteria:** A developer reading ROADMAP.md knows what's next and why.

---

### Task 3 — Spec Drift Audit & Fix

**What:** Verify every docs/specs/ file matches current implementation. Fix discrepancies.

**Deliverables:**
- Audit each of the 8 existing specs:
  - `content-acquisition.md` — verify against crawler/ code
  - `classification.md` — verify against classifier/ + all 5 ML sidecars
  - `content-routing.md` — verify against publisher/ (11 routing layers now, spec may show fewer)
  - `discovery-querying.md` — verify against search/ + index-manager/
  - `shared-infrastructure.md` — verify against infrastructure/
  - `mcp-server.md` — verify against mcp-north-cloud/
  - `social-publisher.md` — verify against social-publisher/
  - `rfp-ingestor.md` — verify against rfp-ingestor/
- Fix each spec where code has diverged (code wins)
- Add `Last verified: YYYY-MM-DD` header to each spec
- Create stub specs for undocumented services: auth, pipeline, dashboard, click-tracker, ai-observer

**Success criteria:** Every spec has a "Last verified" date within the current week. No known divergences between specs and code.

---

### Task 4 — GitHub Governance

**What:** Set up milestones, labels, and issue templates so the roadmap is actionable in GitHub.

**Deliverables:**
- Create GitHub milestones:
  - `M0: Architecture Review & Versioning`
  - `M1: Smart Extraction`
  - `M2: Dynamic Crawling`
  - `M3: Observability Hardening`
  - `M4: Contract Formalization`
- Create missing service labels: `publisher`, `search`, `source-manager`, `social-publisher`, `rfp-ingestor`, `pipeline`, `click-tracker`, `auth`, `dashboard`, `index-manager`
- Create category labels: `governance`, `spec-drift`, `versioning`, `roadmap`
- Create issue templates in `.github/ISSUE_TEMPLATE/`:
  - `bug.md` — bug report with reproduction steps
  - `feature.md` — feature request with acceptance criteria
  - `spec-update.md` — spec drift fix with before/after
- Migrate existing open issues into correct milestones
- Create issues for each M0 task (this document becomes the source of truth)

**Success criteria:** `gh milestone list` shows 5+ milestones. Every open issue belongs to a milestone.

---

### Task 5 — Module Path Cleanup

**What:** Fix the go.mod import path inconsistency across services.

**Current state:**
- Most services: `github.com/jonesrussell/north-cloud/{service}`
- nc-http-proxy, infrastructure: `github.com/north-cloud/{service}`

**Deliverables:**
- Decide canonical module path (recommend: `github.com/jonesrussell/north-cloud/{service}` since that's the majority)
- Update all go.mod files to use the canonical path
- Update all import statements accordingly
- Update go.work
- Run `task ci` to verify nothing breaks

**Success criteria:** `grep -r 'github.com/north-cloud/' */go.mod` returns 0 results (or all results use the canonical path).

---

### Task 6 — Service Dependency Map

**What:** Document which services call which, and how.

**Deliverables:**
- `docs/SERVICE-DEPENDENCIES.md` containing:
  - Mermaid diagram of service-to-service communication
  - Table: source service, target service, protocol, endpoint pattern
  - Sections: HTTP calls, Elasticsearch reads/writes, Redis pub/sub, PostgreSQL ownership
- Covers all 17 services

**Communication patterns to document:**
- Crawler → Source Manager (HTTP: fetch sources)
- Crawler → Elasticsearch (HTTP: index raw_content)
- Classifier → Elasticsearch (HTTP: read raw, write classified)
- Classifier → ML Sidecars (HTTP: classification requests)
- Publisher → Elasticsearch (HTTP: read classified)
- Publisher → Redis (Pub/Sub: route to channels)
- Search → Elasticsearch (HTTP: query classified + rfp)
- Dashboard → All backends (HTTP: proxied via nginx)
- Pipeline → Elasticsearch (HTTP: read events)
- Social Publisher → Redis (Sub: consume channels)
- RFP Ingestor → Elasticsearch (HTTP: index rfp_classified_content)
- AI Observer → Elasticsearch (HTTP: read for analysis)

**Success criteria:** A new developer can look at one diagram and understand how data flows through the platform.

---

## Milestone Sequence After M0

```
M0: Architecture Review     ← YOU ARE HERE
 ↓
M1: Smart Extraction         (fix 75% of broken sources)
 ↓
M2: Dynamic Crawling          (headless browser for JS sites)
 ↓  (can run parallel with M2)
M3: Observability Hardening   (SLAs, alerting, runbooks)
 ↓
M4: Contract Formalization    (OpenAPI, API versioning)
 ↓
M5: Product Layer (v0.6+)     (GIS, community indexing, search UX)
```

---

## Estimated Size

**M0 is a documentation milestone.** No application code changes except Task 5 (module paths).

- Task 1 (Version): Small — 1-2 hours
- Task 2 (Roadmap): Medium — 2-3 hours
- Task 3 (Spec Drift): Large — 4-8 hours (8 specs to audit against code)
- Task 4 (GitHub): Medium — 1-2 hours
- Task 5 (Module Paths): Medium — 2-3 hours (code change, needs CI verification)
- Task 6 (Dependency Map): Medium — 2-3 hours

**Total: ~2-3 focused sessions**
