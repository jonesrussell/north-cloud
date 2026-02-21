# Documentation Overhaul Design

**Date**: 2026-02-20
**Scope**: Full README and CLAUDE.md audit, restructure, and creation across all monorepo services
**Goal**: Public/open-source ready documentation with accuracy and professional polish

---

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Audience | Public / open-source ready | Self-contained READMEs with full feature docs |
| ML sidecars docs | Single `ml-sidecars/README.md` | All 5 Python sidecars documented as a group |
| CLAUDE.md scope | Full restructure to consistent template | Uniform developer experience across services |
| Root CLAUDE.md | Split into CLAUDE.md + ARCHITECTURE.md | Quick-ref/rules vs. deep system design |
| Approach | Sequential deep-dive (read code, then write) | Accuracy over speed |

---

## Document Architecture

### Root Level
- **`README.md`** — Public overview (pipeline, services, quick start, env vars). Polish existing.
- **`CLAUDE.md`** — AI dev guide condensed to: quick-ref commands, critical linting rules, code conventions, Docker conventions, git workflow. ~300 lines max.
- **`ARCHITECTURE.md`** — New file. Deep architecture: pipeline flow, service interactions, routing layers, ES index model, Redis channels, Go bootstrap pattern, version history.

### Service Level
Each service gets:
- **`README.md`** — Public-facing. Setup, features, API reference, configuration, architecture, development, integration.
- **`CLAUDE.md`** — AI dev guide. Quick reference, architecture, key concepts, gotchas, testing, code patterns.

---

## README Template

```markdown
# {Service Name}

> One-line tagline.

## Overview
What it does and where it fits in the North Cloud pipeline.

## Features
- Feature 1
- Feature 2

## Quick Start
### Docker (Recommended)
### Local Development

## API Reference
| Method | Path | Auth | Description |
|--------|------|------|-------------|

## Configuration
### Environment Variables
| Variable | Default | Description |

## Architecture
Internal package structure + brief description of each.

## Development
### Running Tests
### Linting
### Building

## Integration
How this service connects to upstream/downstream services.
```

---

## CLAUDE.md Template

```markdown
# {Service} — Developer Guide

## Quick Reference
Commands you run every day.

## Architecture
Directory tree + role of each internal package.

## Key Concepts
Service-specific concepts (job states, routing layers, classification pipeline, etc.)

## API Reference
Endpoint list (brief — see README for full docs).

## Configuration
Env vars + config.yml shape.

## Common Gotchas
Numbered list of known traps.

## Testing
How to run tests, mock patterns, coverage targets.

## Code Patterns
Idiomatic examples specific to this service.
```

---

## Full Work Scope

### Create New (9 files)
| File | Notes |
|------|-------|
| `ARCHITECTURE.md` | Split from root CLAUDE.md — pipeline, routing, ES model |
| `auth/README.md` | Service overview, JWT auth flow, API, setup |
| `dashboard/README.md` | Vue 3 management UI, setup, routes, auth |
| `click-tracker/README.md` | Click tracking service overview |
| `click-tracker/CLAUDE.md` | Developer guide |
| `pipeline/README.md` | Pipeline observability service |
| `pipeline/CLAUDE.md` | Developer guide |
| `ml-sidecars/README.md` | All 5 ML sidecars: purpose, API, model, config |
| `nc-http-proxy/CLAUDE.md` | Developer guide for proxy modes |

### Rewrite (15 files)
| File | Issue |
|------|-------|
| `CLAUDE.md` (root) | Split — keep only quick-ref + critical rules |
| `crawler/README.md` | Currently 30 lines — needs full service README |
| `publisher/README.md` | Verify accuracy vs. 6-layer routing |
| `auth/CLAUDE.md` | Restructure to new template |
| `crawler/CLAUDE.md` | Restructure to new template |
| `classifier/CLAUDE.md` | Restructure to new template |
| `source-manager/CLAUDE.md` | Restructure to new template |
| `publisher/CLAUDE.md` | Restructure to new template |
| `index-manager/CLAUDE.md` | Restructure to new template |
| `search/CLAUDE.md` | Restructure to new template |
| `dashboard/CLAUDE.md` | Restructure to new template |
| `mcp-north-cloud/CLAUDE.md` | Restructure to new template |
| `search-frontend/CLAUDE.md` | Restructure to new template |

### Polish Only (7 files)
| File | Issue |
|------|-------|
| `README.md` (root) | Minor accuracy updates |
| `classifier/README.md` | Remove dev-status language, accuracy check |
| `source-manager/README.md` | Accuracy check |
| `index-manager/README.md` | Accuracy check |
| `search/README.md` | Accuracy check |
| `search-frontend/README.md` | Accuracy check |
| `mcp-north-cloud/README.md` | Trim and reorganize (currently 30KB) |
| `nc-http-proxy/README.md` | Accuracy check |

---

## Service Execution Order

Process services in dependency order (upstream first):

1. Root files (README.md, CLAUDE.md → ARCHITECTURE.md)
2. Auth (no upstream deps)
3. Source Manager (feeds Crawler)
4. Crawler (depends on Source Manager)
5. Index Manager (used by Crawler, Classifier)
6. Classifier (depends on Index Manager)
7. Publisher (depends on Classifier output)
8. Search (depends on classified content)
9. Dashboard (orchestrates all services)
10. MCP North Cloud (wraps all services)
11. Search Frontend (depends on Search)
12. nc-http-proxy (dev tool for Crawler)
13. Pipeline (observability layer)
14. Click Tracker (standalone)
15. ml-sidecars (all 5 sidecars in one README)

---

## Quality Criteria

A finished README must:
- [ ] Accurately describe the service's current feature set
- [ ] Include a working Quick Start (Docker + local)
- [ ] Have a complete API endpoint table
- [ ] List all required environment variables
- [ ] Show how it connects to other services
- [ ] Be written in present tense, active voice
- [ ] Not contain development history or "week 1-4" language

A finished CLAUDE.md must:
- [ ] Follow the standard template structure
- [ ] Have Quick Reference commands as the first section
- [ ] List all common gotchas specific to this service
- [ ] Include code examples for the most common patterns
- [ ] Be ≤300 lines (surface-level; link to README for detail)
