# Documentation Overhaul Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create or update all READMEs and CLAUDE.md files across the North Cloud monorepo to be public/open-source ready with accuracy and professional polish.

**Architecture:** Sequential deep-dive approach — read actual source code for each service before writing its documentation. Root-level docs split into README.md (public), CLAUDE.md (AI quick-ref + rules), and new ARCHITECTURE.md (deep system design). All service CLAUDE.md files restructured to a consistent template.

**Tech Stack:** Markdown, Mermaid diagrams, Go 1.24+, Vue.js 3, Python/FastAPI (ML sidecars)

**Design Reference:** `docs/plans/2026-02-20-documentation-overhaul-design.md`

---

## README Template

Every service README must include (scaled to complexity):

```markdown
# {Service Name}

> One-line tagline.

## Overview
What it does and where it fits in the pipeline.

## Features
- Feature 1

## Quick Start
### Docker (Recommended)
### Local Development

## API Reference
| Method | Path | Auth | Description |

## Configuration
| Variable | Default | Description |

## Architecture
Internal package structure.

## Development
Test, lint, build commands.

## Integration
Upstream/downstream connections.
```

## CLAUDE.md Template

Every service CLAUDE.md must include:

```markdown
# {Service} — Developer Guide

## Quick Reference
Daily commands.

## Architecture
Directory tree + package roles.

## Key Concepts
Service-specific concepts.

## API Reference
Endpoint list (brief).

## Configuration
Env vars + config shape.

## Common Gotchas
Numbered trap list.

## Testing
Commands, mocks, coverage.

## Code Patterns
Idiomatic examples.
```

**Quality checklist** (apply to every file before committing):
- [ ] Present tense, active voice throughout
- [ ] No "week 1-4" or development history language
- [ ] All env vars listed
- [ ] API endpoints match actual code
- [ ] Quick Start commands actually work
- [ ] Cross-service integration described
- [ ] ≤300 lines for CLAUDE.md

---

## Phase 1: Root-Level Files

### Task 1: Read root files to understand scope

**Files to read:**
- `README.md`
- `CLAUDE.md`
- `DOCKER.md`
- `Taskfile.yml` (for accurate command reference)
- `.env.example` (for env var reference)

**Step 1:** Read all five files listed above.

**Step 2:** Note any inaccuracies or gaps against what you know about the current architecture (8 routing layers in publisher, 5 ML sidecars, click-tracker, pipeline service).

---

### Task 2: Update root README.md

**Files:**
- Modify: `README.md`

**Step 1:** The current README.md is mostly accurate but may need minor updates. Check:
- Service port table matches CLAUDE.md port table
- ML sidecar list is complete (crime-ml 8076, mining-ml 8077, coforge-ml 8078, entertainment-ml 8079, anishinaabe-ml 8080)
- click-tracker is listed
- pipeline service is listed
- Quick Start commands work

**Step 2:** Update the Documentation section at the bottom to reference ARCHITECTURE.md.

**Step 3:** Verify the Mermaid diagrams are accurate.

**Step 4:** Commit.

```bash
git add README.md
git commit -m "docs(root): update README with complete service list and ARCHITECTURE.md reference"
```

---

### Task 3: Create ARCHITECTURE.md

**Files:**
- Create: `ARCHITECTURE.md`
- Source content from: current `CLAUDE.md` sections (Project Overview, Services Quick Reference, Service-Specific Guidelines, Content Pipeline Flow, Bootstrap Pattern)

**Step 1:** Create `ARCHITECTURE.md` with this structure:

```markdown
# North Cloud Architecture

## Content Pipeline

[Mermaid flowchart showing: Source Manager → Crawler → ES raw_content → Classifier → ML Sidecars → ES classified_content → Publisher → Redis → Consumers]

## Services

### Core Services
[Table: service, port, description, database]

### ML Sidecars
[Table: sidecar, port, description, enabled by]

### Infrastructure
[Elasticsearch, Redis, PostgreSQL, Nginx, MinIO, Loki/Alloy/Grafana]

## Publisher Routing Layers

Layer 1: Auto topic channels (articles:{topic})
Layer 2: DB-backed custom channels
Layer 3: Crime classification channels (crime:homepage, crime:category:{type})
Layer 4: Location channels ({topic}:local:{city}, :province:{code}, :canada, :international)
Layer 5: Mining classification channels (articles:mining, mining:core, mining:commodity:{slug}, etc.)
Layer 6: Entertainment classification channels (entertainment:homepage, entertainment:category:{slug})
Layer 7: [check publisher code]
Layer 8: Coforge domain channels

## Elasticsearch Index Model

- {source}_raw_content: crawler output, classification_status=pending
- {source}_classified_content: enriched with quality, topics, crime/mining/entertainment fields

## Redis Channel Reference

[Full table of all channel patterns and their triggers]

## Go Service Bootstrap Pattern

[Two patterns: simple (auth, search) vs. complex (crawler, classifier)]

## Version History

[Architectural changes 2025-2026 from current CLAUDE.md]
```

**Step 2:** Read `publisher/internal/router/service.go` to verify the exact number of routing layers (currently shown as 6 in CLAUDE.md but CoforgeDomain was added as Layer 8 per recent commits — DBChannelDomain is Layer 2). Confirm layer count before writing.

**Step 3:** Write ARCHITECTURE.md with accurate, complete content.

**Step 4:** Commit.

```bash
git add ARCHITECTURE.md
git commit -m "docs(root): add ARCHITECTURE.md with complete system design and routing layers"
```

---

### Task 4: Rewrite root CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

**Step 1:** The root CLAUDE.md will be trimmed significantly. Keep only:
- Quick Reference (most common commands)
- Critical Rules (linting: interface{}, JSON errors, magic numbers, t.Helper, funlen, gocognit)
- Code Conventions (Go standards, logging, error handling, database)
- Bootstrap Pattern (brief — link to ARCHITECTURE.md for detail)
- Frontend conventions (brief)
- Docker conventions
- Git Workflow
- Troubleshooting (brief)

**Step 2:** Move all service-specific guidelines (crawler scheduler, publisher routing, etc.) to ARCHITECTURE.md or keep them only in service-level CLAUDE.md files.

**Step 3:** Remove the "Version History" section (it belongs in ARCHITECTURE.md).

**Step 4:** Remove the "Documentation References" section (now covered by ARCHITECTURE.md).

**Step 5:** Target length: ~300-400 lines (currently ~800).

**Step 6:** Commit.

```bash
git add CLAUDE.md
git commit -m "docs(root): trim CLAUDE.md to quick-ref + critical rules (architecture moved to ARCHITECTURE.md)"
```

---

## Phase 2: Auth Service

### Task 5: Read auth source code

**Files to read:**
- `auth/CLAUDE.md`
- `auth/main.go`
- `auth/auth/` directory listing
- `auth/config.yml`

**Step 1:** Read the files above to understand what the service actually does, what endpoints exist, what config it needs.

---

### Task 6: Create auth/README.md

**Files:**
- Create: `auth/README.md`

**Step 1:** Write README using the template. Key content:

```markdown
# Auth

> JWT authentication service for North Cloud. Issues 24-hour tokens accepted by all services.

## Overview
Single-user username/password authentication. Validates credentials against environment variables
and issues JWT tokens that all North Cloud services accept via shared AUTH_JWT_SECRET.

## Features
- Username/password authentication
- 24-hour JWT tokens
- Shared secret across all North Cloud services
- Health endpoint (unauthenticated)

## Quick Start
### Docker
### Local Development

## API Reference
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | /health | No | Health check |
| POST | /api/v1/auth/login | No | Authenticate and receive JWT |

## Configuration
| Variable | Default | Description |
|----------|---------|-------------|
| AUTH_USERNAME | admin | Dashboard username |
| AUTH_PASSWORD | (required) | Dashboard password |
| AUTH_JWT_SECRET | (required) | Shared JWT secret (32+ bytes hex) |
| AUTH_PORT | 8040 | HTTP server port |

## Architecture
auth/
├── main.go
├── auth/         # Token generation and validation
├── config/       # Environment config loading
└── internal/     # HTTP handlers

## Development
task test
task lint

## Integration
All services validate bearer tokens using the shared AUTH_JWT_SECRET.
Generate with: openssl rand -hex 32
```

**Step 2:** Verify against `auth/CLAUDE.md` for accuracy.

**Step 3:** Commit.

```bash
git add auth/README.md
git commit -m "docs(auth): add README.md"
```

---

### Task 7: Rewrite auth/CLAUDE.md

**Files:**
- Modify: `auth/CLAUDE.md`

**Step 1:** Restructure to standard template. Current content is good — reorganize into the standard sections. Keep all the existing technical detail (JWT format, token validation pattern, gotchas).

**Step 2:** Commit.

```bash
git add auth/CLAUDE.md
git commit -m "docs(auth): restructure CLAUDE.md to standard template"
```

---

## Phase 3: Source Manager

### Task 8: Polish source-manager/README.md

**Files:**
- Modify: `source-manager/README.md`
- Read: `source-manager/CLAUDE.md`, `source-manager/main.go`

**Step 1:** Verify API endpoints are complete (including Excel import, city mapping).

**Step 2:** Add Integration section describing how source-manager IDs are used by crawler.

**Step 3:** Remove any development-progress language.

**Step 4:** Commit.

```bash
git add source-manager/README.md
git commit -m "docs(source-manager): polish README with integration section"
```

---

### Task 9: Rewrite source-manager/CLAUDE.md

**Files:**
- Modify: `source-manager/CLAUDE.md`

**Step 1:** Restructure to standard template. Current content is comprehensive — reorganize sections.

**Step 2:** Commit.

```bash
git add source-manager/CLAUDE.md
git commit -m "docs(source-manager): restructure CLAUDE.md to standard template"
```

---

## Phase 4: Crawler

### Task 10: Read crawler source code

**Files to read:**
- `crawler/README.md` (current sparse version)
- `crawler/CLAUDE.md`
- `crawler/docs/INTERVAL_SCHEDULER.md`
- `crawler/main.go`

**Step 1:** Read all files to capture full feature set including: adaptive scheduling, frontier, proxy rotation, MinIO archiving, Redis Colly storage, readability fallback.

---

### Task 11: Rewrite crawler/README.md

**Files:**
- Modify: `crawler/README.md`

**Step 1:** Replace the current 30-line file with a full service README. Include:
- Overview: interval-based job scheduler for web crawling
- Features: 7 job states, distributed locking, adaptive scheduling, frontier-based crawling, extraction quality metrics, proxy rotation, MinIO HTML archiving, Redis Colly storage
- Quick Start: Docker + local
- API Reference: all endpoints (CRUD + pause/resume/cancel/retry + executions/stats + scheduler/metrics)
- Configuration: key env vars (interval type, adaptive scheduling, proxies, Redis storage, readability fallback)
- Architecture: directory tree
- Development: test, lint, migrate commands
- Integration: reads source_id from source-manager, writes to Elasticsearch raw_content index

**Step 2:** Commit.

```bash
git add crawler/README.md
git commit -m "docs(crawler): rewrite README from 30 lines to full service documentation"
```

---

### Task 12: Rewrite crawler/CLAUDE.md

**Files:**
- Modify: `crawler/CLAUDE.md`

**Step 1:** Restructure to standard template. Current content is excellent — reorganize into standard sections. Keep all gotchas, scheduler concepts, and code examples.

**Step 2:** Commit.

```bash
git add crawler/CLAUDE.md
git commit -m "docs(crawler): restructure CLAUDE.md to standard template"
```

---

## Phase 5: Index Manager

### Task 13: Polish index-manager/README.md

**Files:**
- Modify: `index-manager/README.md`
- Read: `index-manager/CLAUDE.md`

**Step 1:** Verify accuracy. Check: all API endpoints listed, index types correct, mappings described.

**Step 2:** Add Integration section.

**Step 3:** Remove any license section if present (not appropriate for internal project docs).

**Step 4:** Commit.

```bash
git add index-manager/README.md
git commit -m "docs(index-manager): polish README accuracy and add integration section"
```

---

### Task 14: Rewrite index-manager/CLAUDE.md

**Files:**
- Modify: `index-manager/CLAUDE.md`

**Step 1:** Restructure to standard template.

**Step 2:** Commit.

```bash
git add index-manager/CLAUDE.md
git commit -m "docs(index-manager): restructure CLAUDE.md to standard template"
```

---

## Phase 6: Classifier

### Task 15: Read classifier source code

**Files to read:**
- `classifier/README.md`
- `classifier/CLAUDE.md`
- `classifier/internal/` directory listing

**Step 1:** Read to capture the current classification pipeline, all ML sidecars integrated (crime, mining, entertainment, anishinaabe, coforge), and hybrid classification decision matrices.

---

### Task 16: Polish classifier/README.md

**Files:**
- Modify: `classifier/README.md`

**Step 1:** Remove all "Development Status (Week 1-4)" language — replace with current feature status.

**Step 2:** Update ML integration section — it currently says "ML integration design (future)" but ML is implemented. Update to reflect actual hybrid rule+ML classification for crime, mining, entertainment, anishinaabe, and coforge.

**Step 3:** Update topic taxonomy to match current rules (verify against `classifier/internal/` topic rules).

**Step 4:** Remove performance targets section or update with real numbers if known.

**Step 5:** Commit.

```bash
git add classifier/README.md
git commit -m "docs(classifier): remove dev-status language, update ML integration section"
```

---

### Task 17: Rewrite classifier/CLAUDE.md

**Files:**
- Modify: `classifier/CLAUDE.md`

**Step 1:** Restructure to standard template. The current content is excellent for accuracy — just reorganize.

**Step 2:** Ensure all 5 hybrid classifiers are documented (crime, mining, entertainment, anishinaabe, coforge).

**Step 3:** Commit.

```bash
git add classifier/CLAUDE.md
git commit -m "docs(classifier): restructure CLAUDE.md to standard template"
```

---

## Phase 7: Publisher

### Task 18: Read publisher source code

**Files to read:**
- `publisher/README.md` (current version to understand what's outdated)
- `publisher/CLAUDE.md`
- `publisher/internal/router/service.go` (to verify exact routing layers and channel names)
- `publisher/docs/REDIS_MESSAGE_FORMAT.md`

**Step 1:** Read these files. Note that the current README only shows a few crime sub-categories and is missing: Layer 3-8 routing, Coforge domain, entertainment channels, DBChannelDomain (Layer 2), location channels (Layer 4).

---

### Task 19: Rewrite publisher/README.md

**Files:**
- Modify: `publisher/README.md`

**Step 1:** Rewrite with accurate 8-layer routing model (verify layer count from `service.go`).

Key sections:
- Overview: two-process design (API + router background worker)
- Features: multi-layer routing, deduplication, dynamic configuration, Redis pub/sub
- Routing Layers: describe all layers accurately
- Redis Channels: complete channel reference table
- API Reference: all endpoints
- Configuration: env vars
- Architecture: directory tree showing cmd_api.go, cmd_router.go split
- Integration: reads from classified_content indexes, publishes to Redis

**Step 2:** Commit.

```bash
git add publisher/README.md
git commit -m "docs(publisher): rewrite README with accurate 8-layer routing documentation"
```

---

### Task 20: Rewrite publisher/CLAUDE.md

**Files:**
- Modify: `publisher/CLAUDE.md`

**Step 1:** Restructure to standard template. Current content is comprehensive — reorganize.

**Step 2:** Update routing layer count to match actual implementation.

**Step 3:** Commit.

```bash
git add publisher/CLAUDE.md
git commit -m "docs(publisher): restructure CLAUDE.md to standard template"
```

---

## Phase 8: Search

### Task 21: Polish search/README.md and CLAUDE.md

**Files:**
- Modify: `search/README.md`
- Modify: `search/CLAUDE.md`

**Step 1:** Read both files. The search README is already comprehensive (347 lines). Focus on:
- Remove "Future enhancements" section (YAGNI for docs)
- Verify API response format is current
- Check port reference (8092 dev, 8090 internal)

**Step 2:** Restructure CLAUDE.md to standard template.

**Step 3:** Commit both.

```bash
git add search/README.md search/CLAUDE.md
git commit -m "docs(search): polish README and restructure CLAUDE.md"
```

---

## Phase 9: Dashboard

### Task 22: Read dashboard source code

**Files to read:**
- `dashboard/CLAUDE.md`
- `dashboard/src/` directory listing
- `dashboard/package.json` (tech stack)

**Step 1:** Read to understand routes, components, tech stack, auth pattern.

---

### Task 23: Create dashboard/README.md

**Files:**
- Create: `dashboard/README.md`

**Step 1:** Write README:

```markdown
# Dashboard

> Management UI for North Cloud. Monitor and configure the content pipeline.

## Overview
Vue.js 3 admin dashboard for managing sources, channels, routes, crawl jobs, and viewing
classification results. Requires authentication (JWT).

## Features
- Source management (CRUD, test crawl)
- Crawl job scheduling and monitoring
- Publisher route and channel configuration
- Classification results and quality scores
- Pipeline health overview
- Index management

## Quick Start
### Docker (Recommended)
Available at http://localhost:3002 when running north-cloud stack.

### Local Development
Node.js 20+ required.
npm install
npm run dev

## Routes
| Path | Description |
|------|-------------|
| /login | Authentication |
| /sources | Source management |
| /jobs | Crawl job management |
| /channels | Publisher channels |
| /routes | Publisher routes |
| /classify | Classification testing |
| /indexes | Elasticsearch indexes |

## Configuration
| Variable | Default | Description |
|----------|---------|-------------|
| VITE_API_BASE_URL | /api | Backend API URL |

## Architecture
dashboard/src/
├── api/          # Axios clients per service
├── components/   # Shared UI components
├── composables/  # Vue composables (useAuth, useApi)
├── router/       # Vue Router with auth guards
├── stores/       # Pinia state management
├── types/        # TypeScript interfaces
└── views/        # Page components

## Development
npm run dev       # Dev server
npm run build     # Production build
npm run lint      # ESLint

## Integration
Talks to: auth (:8040), source-manager (:8050), crawler (:8060), publisher (:8070),
classifier (:8071), index-manager (:8090), search (:8092) via Nginx proxy.
```

**Step 2:** Commit.

```bash
git add dashboard/README.md
git commit -m "docs(dashboard): add README.md"
```

---

### Task 24: Rewrite dashboard/CLAUDE.md

**Files:**
- Modify: `dashboard/CLAUDE.md`

**Step 1:** Restructure to standard template. Current content is good — reorganize.

**Step 2:** Commit.

```bash
git add dashboard/CLAUDE.md
git commit -m "docs(dashboard): restructure CLAUDE.md to standard template"
```

---

## Phase 10: MCP North Cloud

### Task 25: Polish mcp-north-cloud/README.md

**Files to read:**
- `mcp-north-cloud/README.md` (currently 30KB — needs trimming)
- `mcp-north-cloud/CLAUDE.md`

**Step 1:** Read the README. 30KB is too long. Identify what's redundant with CLAUDE.md. The README should cover: what is MCP, setup, configuration, tool list (table format), integration examples. Move implementation details (handler patterns, JSON-RPC error codes) to CLAUDE.md.

**Step 2:** Restructure README to be ~400 lines max.

**Step 3:** Commit.

```bash
git add mcp-north-cloud/README.md
git commit -m "docs(mcp-north-cloud): trim README from 30KB, reorganize for clarity"
```

---

### Task 26: Rewrite mcp-north-cloud/CLAUDE.md

**Files:**
- Modify: `mcp-north-cloud/CLAUDE.md`

**Step 1:** Restructure to standard template. Current content is excellent — reorganize.

**Step 2:** Move JSON-RPC error codes from README to CLAUDE.md.

**Step 3:** Commit.

```bash
git add mcp-north-cloud/CLAUDE.md
git commit -m "docs(mcp-north-cloud): restructure CLAUDE.md to standard template"
```

---

## Phase 11: Search Frontend

### Task 27: Polish search-frontend docs

**Files:**
- Modify: `search-frontend/README.md`
- Modify: `search-frontend/CLAUDE.md`

**Step 1:** Read both files. Both are good. Focus on:
- Remove "Contributing guidelines" and "License" from README (internal project)
- Verify tech stack (Vite 7, Tailwind 4 — check package.json)
- Restructure CLAUDE.md to standard template

**Step 2:** Commit both.

```bash
git add search-frontend/README.md search-frontend/CLAUDE.md
git commit -m "docs(search-frontend): polish README, restructure CLAUDE.md"
```

---

## Phase 12: nc-http-proxy

### Task 28: Polish nc-http-proxy/README.md

**Files to read:**
- `nc-http-proxy/README.md`
- `nc-http-proxy/main.go`

**Step 1:** Read both. README is already comprehensive. Verify accuracy, add Integration section.

---

### Task 29: Create nc-http-proxy/CLAUDE.md

**Files:**
- Create: `nc-http-proxy/CLAUDE.md`

**Step 1:** Write a developer guide covering:
- Quick reference (Taskfile commands for mode switching)
- Key concepts: replay mode (fixtures), record mode (live + cache), live mode (pass-through)
- Cache key generation (SHA-256 of normalized URL)
- Admin API endpoints
- Common gotchas: fixture not found in replay mode, stale cache, mode switching workflow
- Testing workflow: how to add new fixtures

**Step 2:** Commit both.

```bash
git add nc-http-proxy/README.md nc-http-proxy/CLAUDE.md
git commit -m "docs(nc-http-proxy): add CLAUDE.md, polish README"
```

---

## Phase 13: Pipeline Service

### Task 30: Read pipeline source code

**Files to read:**
- `pipeline/internal/` directory listing
- `pipeline/main.go`
- `pipeline/config.yml.example` (if exists)

**Step 1:** Read to understand what pipeline actually does (observability and event tracking).

---

### Task 31: Create pipeline/README.md and CLAUDE.md

**Files:**
- Create: `pipeline/README.md`
- Create: `pipeline/CLAUDE.md`

**Step 1:** Write README with: what it is, API endpoints, configuration, how it fits in the system.

**Step 2:** Write CLAUDE.md with: developer guide, key concepts, gotchas.

**Step 3:** Commit.

```bash
git add pipeline/README.md pipeline/CLAUDE.md
git commit -m "docs(pipeline): add README.md and CLAUDE.md"
```

---

## Phase 14: Click Tracker

### Task 32: Read click-tracker source code

**Files to read:**
- `click-tracker/internal/` directory listing
- `click-tracker/main.go`
- `click-tracker/cmd/`

**Step 1:** Read to understand what the service does (tracking article link clicks for analytics).

---

### Task 33: Create click-tracker/README.md and CLAUDE.md

**Files:**
- Create: `click-tracker/README.md`
- Create: `click-tracker/CLAUDE.md`

**Step 1:** Write README with: purpose, API endpoints (redirect tracking), configuration, integration.

**Step 2:** Write CLAUDE.md with: developer guide, key concepts, gotchas.

**Step 3:** Commit.

```bash
git add click-tracker/README.md click-tracker/CLAUDE.md
git commit -m "docs(click-tracker): add README.md and CLAUDE.md"
```

---

## Phase 15: ML Sidecars

### Task 34: Read ml-sidecars source code

**Files to read:**
- `ml-sidecars/mining-ml/main.py`
- `ml-sidecars/crime-ml/main.py`
- `ml-sidecars/coforge-ml/main.py`
- `ml-sidecars/entertainment-ml/main.py`
- `ml-sidecars/anishinaabe-ml/main.py`
- `ml-sidecars/mining-ml/requirements.txt` (to confirm FastAPI)

**Step 1:** Read to understand: endpoints (classify, health), request/response format, models used, configuration options.

---

### Task 35: Create ml-sidecars/README.md

**Files:**
- Create: `ml-sidecars/README.md`

**Step 1:** Write a top-level README covering all 5 sidecars:

```markdown
# ML Sidecars

> Python/FastAPI ML classification services that augment the North Cloud classifier.

## Overview
ML sidecars are standalone FastAPI services that provide specialized ML-based content
classification. The classifier service calls them during the classification pipeline.
Each sidecar is optional and controlled by environment variable flags.

## Sidecars

| Sidecar | Port | Env Flag | Purpose |
|---------|------|----------|---------|
| crime-ml | 8076 | CRIME_ENABLED | Crime content classification |
| mining-ml | 8077 | MINING_ENABLED | Mining industry content |
| coforge-ml | 8078 | COFORGE_ENABLED | Coforge-specific content |
| entertainment-ml | 8079 | ENTERTAINMENT_ENABLED | Entertainment content |
| anishinaabe-ml | 8080 | ANISHINAABE_ENABLED | Indigenous/Anishinaabe content |

## API (all sidecars share this interface)

POST /classify
Content-Type: application/json
{ "title": "...", "body": "..." }

GET /health

## Classify Request/Response

[actual request/response format from code]

## Classification Output Fields

[fields added by each sidecar, as seen in publisher CLAUDE.md]

## Running

All sidecars start with the north-cloud Docker stack.
Enable/disable per sidecar via .env flags.

## Training Models

Each sidecar has train_and_export.py for retraining:
cd ml-sidecars/{sidecar-name}
python train_and_export.py

## Architecture

Each sidecar follows the same structure:
ml-sidecars/{sidecar}/
├── main.py             # FastAPI app
├── classifier/         # ML model modules
├── models/             # Serialized model files
├── requirements.txt
└── train_and_export.py

## Integration

The classifier service calls each enabled sidecar via HTTP.
Results are merged into the classified document in Elasticsearch.
See classifier/CLAUDE.md for the hybrid rule+ML decision matrices.
```

**Step 2:** Fill in the actual request/response format from reading the sidecar source code.

**Step 3:** Commit.

```bash
git add ml-sidecars/README.md
git commit -m "docs(ml-sidecars): add README.md covering all 5 Python ML sidecars"
```

---

## Final: Review and Consistency Check

### Task 36: Cross-service consistency check

**Step 1:** Verify that all CLAUDE.md files follow the standard template structure (same section order, same heading levels).

**Step 2:** Verify that all README files have the Integration section pointing to the correct upstream/downstream services.

**Step 3:** Verify root README.md links to all new documentation files.

**Step 4:** Final commit.

```bash
git add -A
git commit -m "docs: complete documentation overhaul - 25 files created/updated"
```

---

## Execution Notes

- Read actual code before writing any doc — no guessing
- When in doubt about an env var name, check `.env.example` or `config.yml.example`
- When in doubt about an API endpoint, check the service's `internal/api/` handlers
- Router layers: verify count in `publisher/internal/router/service.go` before writing
- ML sidecars: read `main.py` for actual POST /classify request/response shape
- Do not add "License" sections (internal project)
- Do not add "Contributing" sections (internal project)
- Keep docs concise — link to deeper docs rather than repeating content
