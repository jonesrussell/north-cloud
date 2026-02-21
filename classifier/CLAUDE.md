# Classifier — Developer Guide

## Quick Reference

```bash
# Development
task dev              # Start with hot reload
task test             # Run tests
task lint             # Run linter
task migrate:up       # Run migrations

# API (port 8071)
curl http://localhost:8071/health
curl http://localhost:8071/api/v1/classify \
  -X POST \
  -H "Authorization: Bearer <token>" \
  -d '{"title":"...", "raw_text":"..."}'
```

## Architecture

```
classifier/
├── cmd/
│   └── migrate/        # Database migration runner
├── internal/
│   ├── api/            # HTTP handlers (Gin), routes, server setup
│   ├── anishinaabemlclient/  # Anishinaabe ML sidecar HTTP client
│   ├── bootstrap/      # Service initialisation phases
│   ├── classifier/     # Core classification logic
│   │   ├── classifier.go         # Orchestrator
│   │   ├── content_type.go       # Article vs page detection
│   │   ├── quality.go            # Quality scoring (0-100)
│   │   ├── topic.go              # Rule-based topic detection
│   │   ├── source_reputation.go  # Source trust scoring
│   │   ├── crime.go              # Hybrid crime classifier
│   │   ├── mining.go             # Hybrid mining classifier
│   │   ├── coforge.go            # Hybrid coforge classifier
│   │   ├── entertainment.go      # Hybrid entertainment classifier
│   │   ├── anishinaabe.go        # Hybrid anishinaabe classifier
│   │   └── location.go           # Location classifier
│   ├── coforgemlclient/    # Coforge ML sidecar HTTP client
│   ├── config/             # Configuration struct and loader
│   ├── data/               # Static data assets
│   ├── database/           # PostgreSQL repositories (rules, reputation)
│   ├── domain/             # RawContent, ClassifiedContent, Rule models
│   ├── elasticsearch/      # ES client and index mappings
│   ├── entertainmentmlclient/ # Entertainment ML sidecar HTTP client
│   ├── mlclient/           # Shared ML client utilities
│   ├── mlhealth/           # ML sidecar health check helper
│   ├── mltransport/        # HTTP transport for ML sidecars
│   ├── processor/          # Background polling and batch processing
│   ├── storage/            # Elasticsearch read/write
│   └── telemetry/          # OpenTelemetry/Prometheus provider
├── migrations/             # SQL files (001-011)
└── tests/integration/      # Integration tests
```

## Key Concepts

### 4-Step Classification Pipeline

```
Elasticsearch: {source}_raw_content (classification_status=pending)
    ↓
Poller fetches batch → Classifier.Classify() → BuildClassifiedContent()
    ↓
Elasticsearch: {source}_classified_content
```

**Step 1 — Content Type** (`content_type.go`): Determines `content_type` (article, page, video, image, job) and `content_subtype` (press_release, blog_post, event, advisory, report, blotter, company_announcement). URL patterns and content heuristics drive this step.

**Step 2 — Quality Score** (`quality.go`): Produces an integer 0-100 from four equally-weighted components (word count, metadata completeness, content richness, readability). Items scoring below the spam threshold (30) are flagged but still classified.

**Step 3 — Topic Detection** (`topic.go`): Loads keyword rules from the `classification_rules` PostgreSQL table. Rules are evaluated in priority-descending order; up to `MaxTopics` (default 5) topics may match. Rules are cached in memory at startup.

**Step 4 — Source Reputation** (`source_reputation.go`): Looks up the source's historical reputation score (0-100 from PostgreSQL) and updates it after each classification based on the quality score and spam flag.

### Optional Classifiers (Steps 5-9)

After the 4-step pipeline, `classifyOptionalForPublishable()` gates five hybrid classifiers on content type and subtype:

| Content type/subtype | Classifiers that run |
|----------------------|----------------------|
| non-article | none |
| article/event | location only |
| article/blotter | crime only |
| article/report | none |
| article (other subtypes) | crime, mining, coforge, entertainment, anishinaabe, location |

Each hybrid classifier is nil when disabled — the corresponding field is omitted from the classified document output.

### Quality Score Details

| Factor | Max points | Notes |
|--------|------------|-------|
| Word count | 25 | Scaled: <100=10, 100-200=15, 200-300=20, 300+=25 |
| Metadata completeness | 25 | Title, published date, author, description |
| Content richness | 25 | Paragraph structure, headings |
| Readability | 25 | Sentence variety, avg length |

**Thresholds**:
- `quality_score >= 70`: High quality
- `quality_score 40-69`: Medium quality
- `quality_score < 40`: Low quality / spam candidate
- `quality_score < 30`: Spam threshold (flagged; source reputation penalised)

### Topic Classification Rules

Stored in PostgreSQL `classification_rules` table:

```sql
-- Example crime sub-category rules (from migration 007)
INSERT INTO classification_rules (topic, keywords, priority, enabled) VALUES
('violent_crime',   '["murder","assault","shooting","homicide"]', 100, true),
('property_crime',  '["theft","burglary","vandalism","arson"]',   90,  true),
('drug_crime',      '["drug","narcotic","trafficking"]',          90,  true);
```

**5 Crime Sub-Categories** (migration 007):
- `violent_crime` — murder, assault, shooting, homicide, domestic violence, kidnapping
- `property_crime` — theft, burglary, robbery, vandalism, arson, shoplifting
- `drug_crime` — drugs, narcotics, trafficking, drug busts, overdoses
- `organized_crime` — gang, cartel, racketeering, money laundering, human trafficking
- `criminal_justice` — court, sentencing, trial, arrest, conviction

**Mining topic rule** (migration 011): Uses narrow, mining-specific keywords only. Ambiguous terms (gold, silver, resource, grade, deposit) were removed to prevent false positives — the mining-ml hybrid classifier handles nuanced relevance filtering.

### Hybrid Classifiers

Each of the five domain classifiers follows the same pattern:

1. Evaluate keyword rules (high precision, fast)
2. Call the ML sidecar via HTTP (higher recall)
3. Merge via a decision matrix → final relevance class + confidence
4. Non-blocking: ML failure falls back to rules-only; logs a warning

#### Crime Hybrid Classifier

**Enabled via**: `CRIME_ENABLED=true`, `CRIME_ML_SERVICE_URL=http://crime-ml:8076`

**Relevance classes**:
- `core_street_crime` — Homepage eligible (murders, shootings, assaults with arrest)
- `peripheral_crime` — Category pages only (impaired driving, international, policy)
- `not_crime` — Excluded

**Decision matrix**:

| Rules | ML | Result | Confidence |
|-------|----|--------|------------|
| core | core | core | High (avg) |
| core | not_crime | core + review | Medium |
| core | unreachable | core | Rule conf |
| — | core (>0.9) | peripheral + review | ML conf * 0.8 |
| other | other | Rule result | Rule conf |

**Output fields** in `ClassifiedContent`:
```json
{
  "crime": {
    "street_crime_relevance": "core_street_crime",
    "crime_types": ["violent_crime"],
    "location_specificity": "local_canada",
    "final_confidence": 0.92,
    "homepage_eligible": true,
    "category_pages": ["violent-crime", "crime"],
    "review_required": false
  }
}
```

#### Mining Hybrid Classifier

**Enabled via**: `MINING_ENABLED=true`, `MINING_ML_SERVICE_URL=http://mining-ml:8077`

**Relevance classes**: `core_mining`, `peripheral_mining`, `not_mining`

**Mining stage**: `exploration`, `development`, `production`, `unspecified`

**Commodities** (multi-label): `gold`, `copper`, `lithium`, `nickel`, `uranium`, `iron_ore`, `rare_earths`, `other`

**Decision matrix**:

| Rules | ML | Result | ReviewRequired |
|-------|----|--------|----------------|
| core | core | core_mining, high conf | false |
| core | not_mining | core_mining | true |
| core | unreachable | core_mining, rule conf | false |
| peripheral | core, high conf | core_mining | optional |
| not_mining | core, high conf | peripheral_mining | true |
| not_mining | core, low conf | peripheral_mining | true |

**Output fields**:
```json
{
  "mining": {
    "relevance": "core_mining",
    "mining_stage": "exploration",
    "commodities": ["gold", "copper"],
    "location": "local_canada",
    "final_confidence": 0.92,
    "review_required": false,
    "model_version": "2025-02-01-mining-v1"
  }
}
```

#### Entertainment Hybrid Classifier

**Enabled via**: `ENTERTAINMENT_ENABLED=true`, `ENTERTAINMENT_ML_SERVICE_URL=http://entertainment-ml:8079`

**Relevance classes**: `core_entertainment`, `peripheral_entertainment`, `not_entertainment`

Classifies entertainment industry news, celebrity content, film, TV, and music.

#### Coforge Hybrid Classifier

**Enabled via**: `COFORGE_ENABLED=true`, `COFORGE_ML_SERVICE_URL=http://coforge-ml:8078`

**Relevance classes**: `core_coforge`, `peripheral_coforge`, `not_coforge`

Classifies Coforge-relevant industry and technology content.

#### Anishinaabe Hybrid Classifier

**Enabled via**: `ANISHINAABE_ENABLED=true`, `ANISHINAABE_ML_SERVICE_URL=http://anishinaabe-ml:8080`

**Relevance classes**: `core_anishinaabe`, `peripheral_anishinaabe`, `not_anishinaabe`

**Categories** (multi-label): `culture`, `language`, `governance`, `land_rights`, `education`

Classifies Anishinaabe/Indigenous content for routing to specialised consumers.

**Output fields**:
```json
{
  "anishinaabe": {
    "relevance": "core_anishinaabe",
    "categories": ["culture", "language"],
    "final_confidence": 0.88,
    "review_required": false,
    "model_version": "2026-02-16-anishinaabe-v1"
  }
}
```

## API Reference

All `/api/v1/*` routes require `Authorization: Bearer <token>`.

**Health** (public):
- `GET /health` — Liveness
- `GET /ready` — Readiness
- `GET /health/memory` — Memory stats
- `GET /metrics` — Prometheus metrics (when telemetry provider configured)

**Classification**:
- `POST /api/v1/classify` — Classify a single article
- `POST /api/v1/classify/batch` — Classify multiple articles
- `POST /api/v1/classify/reclassify/:content_id` — Re-classify an existing document
- `GET /api/v1/classify/:content_id` — Get classification result

**Rules**:
- `GET /api/v1/rules` — List classification rules
- `POST /api/v1/rules` — Create rule
- `PUT /api/v1/rules/:id` — Update rule
- `DELETE /api/v1/rules/:id` — Delete rule

**Source Reputation**:
- `GET /api/v1/sources` — List sources
- `GET /api/v1/sources/:name` — Get source details
- `PUT /api/v1/sources/:name` — Update source
- `GET /api/v1/sources/:name/stats` — Source statistics

**Statistics**:
- `GET /api/v1/stats` — Overall stats
- `GET /api/v1/stats/topics` — Topic distribution
- `GET /api/v1/stats/sources` — Source reputation distribution

**Metrics**:
- `GET /api/v1/metrics/ml-health` — Sidecar health (reachability, latency, pipeline mode for all 5 ML sidecars)

## Configuration

```yaml
# config.yml (key fields)
service:
  port: 8071                      # CLASSIFIER_PORT
  concurrency: 10                 # CLASSIFIER_CONCURRENCY
  batch_size: 100
  poll_interval: 30s              # CLASSIFIER_POLL_INTERVAL
  min_quality_score: 0            # CLASSIFIER_MIN_QUALITY_SCORE

classification:
  crime:
    enabled: false                # CRIME_ENABLED
    ml_service_url: ""            # CRIME_ML_SERVICE_URL
  mining:
    enabled: false                # MINING_ENABLED
    ml_service_url: ""            # MINING_ML_SERVICE_URL
  coforge:
    enabled: false                # COFORGE_ENABLED
    ml_service_url: ""            # COFORGE_ML_SERVICE_URL
  entertainment:
    enabled: false                # ENTERTAINMENT_ENABLED
    ml_service_url: ""            # ENTERTAINMENT_ML_SERVICE_URL
  anishinaabe:
    enabled: false                # ANISHINAABE_ENABLED
    ml_service_url: ""            # ANISHINAABE_ML_SERVICE_URL
```

## Common Gotchas

1. **Must populate `Body` and `Source` aliases**: The publisher expects these fields on `ClassifiedContent`. They are set in `BuildClassifiedContent()`:
   ```go
   Body:   raw.RawText,  // Alias for RawText
   Source: raw.URL,      // Alias for URL
   ```
   If these are missing the publisher will skip the document silently.

2. **Spam threshold is 30**: Items with `quality_score < 30` are flagged as spam and the source's reputation score is penalised. The document is still classified and written to `{source}_classified_content`.

3. **Rules are cached in memory**: Changes to `classification_rules` in PostgreSQL take effect on the next service restart. There is no live reload endpoint for rules.

4. **Source reputation updates on every classification**: Quality scores continuously feed back into each source's reputation score via `UpdateAfterClassification()`.

5. **Poller interval is configurable**: Default is 30 seconds. Set via `CLASSIFIER_POLL_INTERVAL` or `service.poll_interval` in `config.yml`.

6. **`Classify()` is near the `funlen` limit**: Optional classifiers (crime, mining, coforge, entertainment, anishinaabe, location) are extracted into `runOptionalClassifiers()`. When adding new classification steps, add them there — not to `Classify()` directly.

7. **Mining false positives**: The mining topic keyword rule (migration 011) is intentionally narrow. Ambiguous commodity terms (gold, silver, resource, grade) are excluded. The mining-ml hybrid classifier (Layer 5 publisher routing) handles nuanced relevance filtering. Do not add broad terms back to the rule.

8. **ML sidecar absent from classified content**: When a hybrid classifier is disabled (`CRIME_ENABLED=false`, etc.), the corresponding domain field (`crime`, `mining`, etc.) is nil and omitted from the Elasticsearch document. Publisher routes that filter on `mining.relevance` will see no results until the sidecar is enabled.

9. **Content subtype gates optional classifiers**: Pages, listings, and non-article types skip all optional classifiers. `event` subtypes run location only; `blotter` subtypes run crime only; `report` subtypes skip all optional classifiers. Standard articles run all enabled classifiers.

## Testing

```bash
# Run all tests
task test

# Run with coverage
task test:cover

# Run specific package
go test ./internal/classifier/...

# Run integration tests (requires all infrastructure running)
go test ./tests/integration/...
```

Integration tests require PostgreSQL, Elasticsearch, and Redis running. Use
`task docker:dev:up` to start the infrastructure before running integration tests.

## Code Patterns

### Adding a new hybrid classifier

1. Create `internal/classifier/{domain}.go` with a `{Domain}Classifier` struct and `Classify(ctx, raw) (*domain.{Domain}Result, error)` method.
2. Create `internal/classifier/{domain}_rules.go` with the keyword rule function.
3. Add `{Domain}Result` struct to `internal/domain/classification.go`.
4. Add `{Domain}Config` to `internal/config/config.go` with `Enabled` and `MLServiceURL` env tags.
5. Wire the classifier in `internal/bootstrap/classifier.go`.
6. Add the result field to `domain.ClassificationResult` and `domain.ClassifiedContent`.
7. Call the classifier in `runOptionalClassifiers()` in `classifier.go`.
8. Add output field to `BuildClassifiedContent()`.

### ES index naming

Raw content: `{source_name}_raw_content`
Classified content: `{source_name}_classified_content`

The suffixes are configurable via `ELASTICSEARCH_RAW_SUFFIX` and `ELASTICSEARCH_CLASSIFIED_SUFFIX` (defaults: `_raw_content`, `_classified_content`).
