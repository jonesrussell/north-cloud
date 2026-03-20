# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- M0 and M1 milestone design documents
- GitHub milestones (M0-M4), service labels, and governance structure
- **Global Indigenous Content Platform (D0)** — non-breaking expansion of indigenous pipeline to global coverage
  - Source-Manager: `indigenous_region` column + migration (7 regions: canada, us, latin_america, oceania, europe, asia, africa)
  - Crawler: region passthrough from source config to ES `meta.indigenous_region`
  - ML Sidecar: multilingual pattern expansion (7 languages, 19 core patterns, 10 categories), model version `2026-03-08-indigenous-v2`
  - Go Classifier: mirrored multilingual regex patterns (19 core + 5 peripheral), region passthrough to `IndigenousResult.Region`
  - Publisher: region routing to `indigenous:region:{slug}` channels
- **Indigenous Region Taxonomy Finalization (D1)** — shared region validation and normalization
  - `infrastructure/indigenous` package: canonical 7-region taxonomy with `IsValidRegion` and `NormalizeRegionSlug`
  - Source-Manager: validates and normalizes `indigenous_region` on create/update
  - Crawler: normalizes region slug before writing to `meta.indigenous_region`
  - Publisher: uses shared `NormalizeRegionSlug` for region channel routing (handles mixed-case, hyphens, spaces)
- **Indigenous Category Taxonomy Expansion (D2)** — expand from 6 to 10 global categories
  - ML Sidecar: structured `CATEGORY_KEYWORDS` with per-language placeholder comments, `CATEGORY_COUNT` constant
  - Go Classifier: category constants and `IndigenousCategories` slice for cross-referencing
  - Publisher: tests for all 10 category routing channels
  - Design doc: `docs/plans/2026-03-11-indigenous-category-taxonomy.md`
- **M-Indigenous-Classifier: Multilingual Indigenous Classifier v3** — full multilingual keyword expansion and confidence scoring
  - ML Sidecar: populated all 10 category keyword arrays across 7 languages, confidence scoring (0.60–0.95), `language_detected` field, model version `2026-03-12-indigenous-v3`
  - Go Classifier: mirrored multilingual category keywords in `indigenousCategoryKeywords` map, pattern-hit-based confidence scoring, `countPatternHits`/`countMatchedCategories` helpers
  - Publisher: confidence threshold (>= 0.35) gates indigenous routing, prevents low-confidence content from cluttering category feeds
  - Tests: 56 Python tests (7 languages, 10 categories, mixed-language, false positives), expanded Go test matrix with confidence scoring and keyword coverage
  - Design doc: `docs/plans/2026-03-12-m-indigenous-classifier.md`
- **M-Indigenous-Backfill: Global Indigenous Re-Crawl** — admin endpoint to trigger staggered re-crawl jobs for indigenous sources
  - Crawler: `POST /api/v1/backfill/indigenous` endpoint with `region`, `limit`, `dry_run` query params
  - Crawler: `BackfillIndigenousHandler` following `SyncEnabledSourcesHandler` pattern with staggered job dispatch
  - Source-Manager: `GET /api/v1/sources/indigenous` endpoint filtering sources with `indigenous_region IS NOT NULL`
  - Publisher: `indigenous_backfill_total`, `indigenous_backfill_success`, `indigenous_backfill_failed` Redis metrics counters
  - Design doc: `docs/plans/2026-03-14-m-indigenous-backfill.md`
- **M-Indigenous-Sources: Global Indigenous Source Onboarding** — seed 186 global indigenous media outlets across 7 regions
  - Source-Manager: new `POST /api/v1/sources/import-indigenous` endpoint for JSON-based bulk import with region validation
  - Source data: `scripts/global-indigenous-sources.json` with 186 outlets (44 Canada, 35 US, 26 Latin America, 32 Oceania, 17 Europe, 16 Asia, 16 Africa)
  - Import logic: validates region, render_mode; sets rate limits (10s static / 12s dynamic), max depth (2 static / 1 dynamic), feed ingestion mode
  - Publisher: all-regions routing test verifying `indigenous:region:{slug}` for all 7 canonical regions
  - Design doc: `docs/plans/2026-03-13-m-indigenous-sources.md`

## [0.5.0] - 2026-03-08

### Added
- AI Observer service for classifier drift detection
- Grafana dashboard for AI Insights
- Prometheus metrics and pipeline dashboard for publisher
- Contract test job in CI pipeline
- v1 contract schemas for Redis messages, search API, and channels
- MCP fetch_url tool with schema extraction and JS renderer
- Playwright renderer added to CI build pipeline

### Fixed
- AI Observer: read source_name instead of non-existent domain field
- AI Observer: strip markdown fences from LLM JSON response
- AI Observer: use flattened ES mapping for details field
- Deploy: force-recreate Grafana on infrastructure changes
- Deploy: route Redis through compose network
- Security: bind Redis and Postgres-crawler to 127.0.0.1 only

## [0.4.0] - 2026-02-16

### Added
- Indigenous ML sidecar and Layer 7 Indigenous routing in publisher
- Mining ML sidecar and Layer 5 mining routing in publisher
- Entertainment ML sidecar and Layer 6 entertainment routing
- Coforge ML sidecar and Layer 8 coforge routing
- Source name sanitization extracted into infrastructure/naming package
- MCP audit logging, rate limiting, health check, error sanitization

### Fixed
- Classifier: skip head and aside in ExtractTextFromHTML
- RFP Ingestor: add explicit .keyword sub-field to content_type mapping
- RFP Ingestor: align ES mapping with search service expectations
- Search: parse topics[] array query param format
- Search: add human-readable label to facet buckets

## [0.3.0] - 2026-01-31

### Added
- Crawler: support unlimited crawl depth via max_depth: -1
- Classifier: event/recipe/job/obituary keyword heuristics + Schema.org Event detection
- Source Manager: Excel import improvements + internal tests
- Crawler: Colly features implementation
- Crawler: URL frontier for deduplication
- RFP Ingestor service (CanadaBuys CSV feed)
- Social Publisher service (Redis subscriber with priority queue)
- Click Tracker service

### Fixed
- Crawler: prevent fetcher from overwriting enriched raw content docs
- Classifier: fix ES index naming derivation + bulk response error parsing

## [0.2.0] - 2026-01-07

### Added
- Crime sub-category classification (violent, property, drug, organized, justice)
- Pipeline event service
- Database-backed publisher routing with 8 layers

### Changed
- Publisher modernization: database-backed Redis Pub/Sub routing hub
- Dashboard authentication: JWT-based auth with route guards

## [0.1.0] - 2025-12-23

### Added
- Raw content pipeline (raw → classify → publish)
- Crawler with interval-based job scheduler
- Classifier with hybrid rules+ML architecture
- Publisher with topic-based routing
- Source Manager with CSS selector configuration
- Search service with multi-index wildcard queries
- Dashboard (Vue.js 3)
- Auth service (JWT tokens)
- Index Manager for ES lifecycle
- Infrastructure shared packages (config, logger, ES client, Redis, JWT)
