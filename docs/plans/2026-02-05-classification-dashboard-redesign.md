# Classification Dashboard Redesign

**Date**: 2026-02-05
**Status**: Design approved
**Scope**: Dashboard classification improvements for operators and ML developers, with support for crime-ml and mining-ml services

---

## Context

Crime-ml and mining-ml have been deployed. The dashboard needs to surface their outputs, give operators visibility into ML health and hybrid decisions, and give ML developers tools for monitoring model performance and drift.

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| New API endpoint home | Classifier service | Only component with access to rules, ML outputs, hybrid merge logic, model versions, and review state |
| Model drift storage | Postgres in classifier DB | Low-cardinality hourly rollups; simple SQL queries; no new infrastructure; classifier owns the data |
| Drift table structure | Split crime/mining tables | Avoids sparse columns; each domain has distinct fields |
| Review queue storage | Postgres in classifier DB | Stateful workflow (pending/reviewed/dismissed) fits relational model; ES stays source of truth for classification data |
| ML developer toggle | Client-side localStorage | Zero backend changes; data isn't sensitive; any user can opt in |
| ML developer tools location | Dedicated `/intelligence/ml` route + inline histograms (toggle-gated) | Clean separation without forcing context-switching |

---

## Architecture Overview

### New Backend (Classifier Service)

**API endpoint groups:**
- `GET /api/v1/metrics/ml-health` — ML service reachability, model versions, latency
- `GET /api/v1/metrics/confidence-distributions` — Histogram data for relevance/stage/commodity confidence
- `GET /api/v1/metrics/disagreements` — Rule vs ML disagreement examples with filtering
- `GET /api/v1/review/queue` — Paginated review queue with status/type filters
- `PUT /api/v1/review/queue/:id` — Update review status, add notes
- `GET /api/v1/review/queue/stats` — Review counts by status and type

**New Postgres tables:**
- `review_queue` — Stateful review workflow
- `crime_classification_metrics` — Hourly crime classification rollups
- `mining_classification_metrics` — Hourly mining classification rollups

### New Backend (Index Manager Service)

- `GET /api/v1/aggregations/mining` — Mining field aggregations (mirrors existing crime aggregation pattern)

### New Dashboard Routes

| Route | View | Audience |
|-------|------|----------|
| `/intelligence/mining` | MiningBreakdownView | Operators |
| `/intelligence/review` | Review Queue | Operators |
| `/intelligence/ml` | Model Drift Indicators | ML developers |
| `/intelligence/ml/disagreements` | Disagreement Explorer | ML developers |

### Enhanced Existing Views

- **Document detail** — Hybrid Decision Audit panel, ML Input Inspector
- **Intelligence section** — Classifier Health widget, Pipeline Mode Indicator
- **CrimeBreakdownView / MiningBreakdownView** — Inline confidence histograms (toggle-gated)

---

## Phase 1: Operator Essentials

### 1.1 Classifier Health Widget

Compact card in the Intelligence section header, visible on all Intelligence pages.

**Displays:**
- **crime-ml**: reachable/unreachable indicator, model version (e.g., `2025-02-01-crime-v1`), last classify latency in ms
- **mining-ml**: same format
- **Pipeline mode**: crime `hybrid` / `rules-only` / `disabled`, mining same

**Backend**: `GET /api/v1/metrics/ml-health` — classifier pings both ML services' `/health` endpoints, returns status + cached model versions. Dashboard polls every 30s.

**Response shape:**
```json
{
  "crime_ml": {
    "reachable": true,
    "model_version": "2025-02-01-crime-v1",
    "last_latency_ms": 45,
    "last_checked_at": "2026-02-05T12:00:00Z"
  },
  "mining_ml": {
    "reachable": true,
    "model_version": "2025-02-01-mining-v1",
    "last_latency_ms": 38,
    "last_checked_at": "2026-02-05T12:00:00Z"
  },
  "pipeline_mode": {
    "crime": "hybrid",
    "mining": "hybrid"
  }
}
```

### 1.2 MiningBreakdownView (`/intelligence/mining`)

Mirrors CrimeBreakdownView structure exactly.

**Panels:**
- Mining percentage of total classified documents
- Relevance breakdown: core_mining / peripheral_mining / not_mining (bar chart)
- Mining stage breakdown: exploration / development / production / unspecified (bar chart)
- Commodity breakdown: gold, copper, lithium, nickel, uranium, iron_ore, rare_earths, other (horizontal bar chart with counts)
- Location breakdown: local_canada / national_canada / international / not_specified

**Backend**: `GET /api/v1/aggregations/mining` on index-manager. Queries `*_classified_content` indexes, aggregates on `mining.*` fields. Follows existing crime aggregation pattern.

### 1.3 Hybrid Decision Audit Panel (Document Detail)

New collapsible panel on DocumentDetailView showing the classification decision trail. Rendered for both crime and mining sections when present.

**Displays:**
- Rule result: relevance + confidence (e.g., `core_mining @ 0.85`)
- ML result: relevance + confidence (e.g., `not_mining @ 0.72`)
- Merge outcome: final relevance + final confidence (e.g., `core_mining @ 0.70`)
- Decision reason: human-readable string (e.g., "Rule core + ML disagree -> review required")
- Review required: yes/no badge

**Data source**: Existing classified content fields — no new API needed, just UI rendering.

### 1.4 Pipeline Mode Indicator

Small status badge group in the Intelligence section showing:
- `Crime: hybrid` / `rules-only` / `disabled`
- `Mining: hybrid` / `rules-only` / `disabled`

Sourced from the `/api/v1/metrics/ml-health` response.

---

## Phase 2: Review Workflow

### 2.1 Review Queue Table

```sql
CREATE TABLE review_queue (
    id BIGSERIAL PRIMARY KEY,
    document_id TEXT NOT NULL,
    index_name TEXT NOT NULL,
    source_name TEXT NOT NULL,
    title TEXT,
    url TEXT,

    -- Classification context
    review_type TEXT NOT NULL,          -- 'crime' or 'mining'
    rule_relevance TEXT,                -- e.g., 'core_street_crime'
    rule_confidence REAL,
    ml_relevance TEXT,                  -- e.g., 'not_crime'
    ml_confidence REAL,
    merge_outcome TEXT,                 -- e.g., 'core_street_crime'
    decision_reason TEXT,               -- human-readable explanation

    -- Workflow state
    status TEXT NOT NULL DEFAULT 'pending',  -- pending / reviewed / dismissed
    reviewer_notes TEXT,
    reviewed_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ DEFAULT now(),

    UNIQUE(document_id, review_type)
);

CREATE INDEX idx_review_queue_status ON review_queue(status);
CREATE INDEX idx_review_queue_type ON review_queue(review_type);
CREATE INDEX idx_review_queue_created ON review_queue(created_at DESC);
```

Classifier inserts a row whenever it sets `review_required=true` during hybrid merge. The `UNIQUE(document_id, review_type)` constraint prevents duplicates on reclassification.

### 2.2 Review Queue API

- `GET /api/v1/review/queue?status=pending&type=crime&page=1&page_size=20` — Paginated, filterable list. Returns items with all classification context inline.
- `PUT /api/v1/review/queue/:id` — Body: `{"status": "reviewed", "reviewer_notes": "..."}`. Sets `reviewed_at` timestamp.
- `GET /api/v1/review/queue/stats` — Returns counts: `{pending: 12, reviewed: 83, dismissed: 7, by_type: {crime: 65, mining: 37}}`.

### 2.3 Review Queue View (`/intelligence/review`)

**Layout:**
- Header stats row: pending count (prominent), reviewed today, dismissed today
- Filter bar: status dropdown (pending/reviewed/dismissed/all), type toggle (crime/mining/all), date range
- Table columns: title (links to document detail), source, review type, rule result, ML result, merge outcome, decision reason, status badge, created timestamp
- Row actions: "Mark Reviewed" and "Dismiss" buttons with optional notes modal
- Empty state: "No items pending review" with check icon

### 2.4 ML Input Inspector (Document Detail)

New panel on DocumentDetailView showing what the ML model received:

- Source text used: the truncated body sent to ML (500 char limit)
- Full body preview: expandable raw text
- Truncation indicator: "Body truncated from 2,847 to 500 characters" if applicable

Data comes from existing classified content fields.

---

## Phase 3: ML Developer Tools

### 3.1 Classification Metrics Tables

```sql
CREATE TABLE crime_classification_metrics (
    id BIGSERIAL PRIMARY KEY,
    hour TIMESTAMPTZ NOT NULL,
    model_version TEXT NOT NULL,
    total_classified INT NOT NULL DEFAULT 0,

    -- Relevance distribution
    relevance_core_pct REAL,
    relevance_peripheral_pct REAL,
    relevance_not_crime_pct REAL,

    -- Confidence stats
    avg_relevance_confidence REAL,
    min_relevance_confidence REAL,
    max_relevance_confidence REAL,
    avg_location_confidence REAL,

    -- Crime type distribution
    violent_crime_pct REAL,
    property_crime_pct REAL,
    drug_crime_pct REAL,
    gang_violence_pct REAL,
    organized_crime_pct REAL,
    criminal_justice_pct REAL,

    -- Disagreement counts
    rule_ml_agree_count INT DEFAULT 0,
    rule_ml_disagree_count INT DEFAULT 0,
    review_required_count INT DEFAULT 0,

    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(hour, model_version)
);

CREATE TABLE mining_classification_metrics (
    id BIGSERIAL PRIMARY KEY,
    hour TIMESTAMPTZ NOT NULL,
    model_version TEXT NOT NULL,
    total_classified INT NOT NULL DEFAULT 0,

    -- Relevance distribution
    relevance_core_pct REAL,
    relevance_peripheral_pct REAL,
    relevance_not_mining_pct REAL,

    -- Stage distribution
    stage_exploration_pct REAL,
    stage_development_pct REAL,
    stage_production_pct REAL,
    stage_unspecified_pct REAL,

    -- Commodity distribution
    commodity_gold_pct REAL,
    commodity_copper_pct REAL,
    commodity_lithium_pct REAL,
    commodity_nickel_pct REAL,
    commodity_uranium_pct REAL,
    commodity_iron_ore_pct REAL,
    commodity_rare_earths_pct REAL,
    commodity_other_pct REAL,

    -- Confidence stats
    avg_relevance_confidence REAL,
    avg_stage_confidence REAL,
    avg_location_confidence REAL,

    -- Disagreement counts
    rule_ml_agree_count INT DEFAULT 0,
    rule_ml_disagree_count INT DEFAULT 0,
    review_required_count INT DEFAULT 0,

    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(hour, model_version)
);
```

### 3.2 Hourly Rollup Collector

Piggybacks on classifier's existing background processor loop. Every hour:

1. Query in-memory counters accumulated during classification (relevance outcomes, confidence values, disagreement counts)
2. Compute percentages and averages for the completed hour
3. Insert one row per model_version per table
4. Reset counters

No Elasticsearch queries needed — classifier has all data at classification time. Counters are lightweight in-memory maps, flushed hourly.

### 3.3 Confidence Distribution API

`GET /api/v1/metrics/confidence-distributions?type=crime&days=7`

```json
{
  "type": "crime",
  "period_days": 7,
  "relevance_confidence": {
    "buckets": [
      {"range": "0.0-0.1", "count": 3},
      {"range": "0.1-0.2", "count": 7},
      {"range": "0.9-1.0", "count": 142}
    ]
  },
  "model_versions": ["2025-02-01-crime-v1"],
  "total_samples": 1284
}
```

For mining, also includes `stage_confidence` and `commodity_scores` histograms.

### 3.4 Model Drift Indicators (`/intelligence/ml`)

Main view at `/intelligence/ml`. Three panels:

- **Version timeline**: Line chart showing model_version transitions over time. Version changes rendered as vertical markers.
- **Relevance drift**: Area chart of `relevance_core_pct` / `peripheral_pct` / `not_pct` over 30 days. Tabbed for crime vs mining.
- **Commodity drift** (mining only): Stacked area chart of commodity percentages over 30 days.

All charts query metrics tables with simple SQL.

### 3.5 Rule vs ML Disagreement Explorer (`/intelligence/ml/disagreements`)

`GET /api/v1/metrics/disagreements?type=crime&disagreement=rule_core_ml_not&page=1&page_size=20`

**Layout:**
- Filter bar: type (crime/mining), disagreement pattern dropdown, date range
- Results table: title, source, rule result, ML result, merge outcome, confidence delta, link to document detail
- Export button: CSV download for offline analysis / retraining datasets
- Summary stats: total disagreements, top patterns, disagreement rate trend

Backend queries `review_queue` table (captures rule/ML/merge context) joined with disagreement counts from metrics tables.

### 3.6 Inline Confidence Histograms (Toggle-gated)

When localStorage `mlDeveloperMode` toggle is enabled, CrimeBreakdownView and MiningBreakdownView each get a collapsible section showing confidence distribution histograms. Uses `/api/v1/metrics/confidence-distributions` endpoint. Hidden by default.

### 3.7 Classification Timeline View

Available to all users in Intelligence section:

- Per-source volume: Stacked bar chart showing classification volume by source over time
- Domain breakdown: Line chart of crime vs mining vs general topic proportions
- Time range selector: 7d / 30d / 90d

Data from metrics tables (`total_classified` per hour, summed by day).

### 3.8 ML Developer Toggle

localStorage key: `mlDeveloperMode` (boolean, default `false`).

Toggle location: Settings icon or toggle switch in the Intelligence section header, near the Classifier Health widget.

When enabled:
- Confidence histogram sections appear on CrimeBreakdownView and MiningBreakdownView
- No other behavioral changes (ML developer pages at `/intelligence/ml` are always accessible via navigation)

---

## Implementation Notes

- MiningBreakdownView reuses CrimeBreakdownView patterns and component structure
- Mining aggregation endpoint in index-manager follows existing crime aggregation pattern
- Review queue inserts happen in classifier's hybrid merge logic (crime and mining classifiers)
- Hourly rollup collector uses in-memory counters, not ES queries
- All new classifier API endpoints require JWT authentication (existing middleware)
- Dashboard components use existing UI patterns (Card, Badge, StatCard, PageHeader)
- No new npm dependencies expected — charts can use existing charting approach or lightweight inline rendering

## Phasing Summary

| Phase | Scope | Dependencies |
|-------|-------|-------------|
| Phase 1: Operator Essentials | Health widget, MiningBreakdownView, Hybrid Decision Audit, Pipeline Mode | None — ships first |
| Phase 2: Review Workflow | review_queue table, Review Queue API + view, ML Input Inspector | Phase 1 (operators need visibility before review workflow) |
| Phase 3: ML Developer Tools | Metrics tables, rollup collector, confidence distributions, drift indicators, disagreement explorer, timeline, toggle | Phase 2 (review_queue used by disagreement explorer) |
