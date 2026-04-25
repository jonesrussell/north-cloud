# Elasticsearch mapping divergence audit (Phase 1B)

**Date:** 2026-04-22  
**Scope:** `classifier/internal/elasticsearch/mappings/*` vs `index-manager/internal/elasticsearch/mappings/*` before consolidation into `infrastructure/esmapping`.

This file is **maintained manually** (not produced by a generator). It records disagreements that the shared SSoT resolved.

## raw_content — top-level fields

| Field | Classifier (pre-SSoT) | Index-manager (pre-SSoT) | Resolution |
| --- | --- | --- | --- |
| `author` | absent | `text` | **Keep index-manager** |
| `article_section` | absent | `keyword` | **Keep index-manager** |
| `json_ld_data` | absent | `object` + properties | **Keep index-manager** |
| `meta` subfields | Rich set incl. `page_type`, `indigenous_region`, text+keyword `detected_content_type`, `article_opinion` **keyword**, `og_image_*` **keyword** (classifier raw) | `article_opinion` **boolean**, `detected_content_type` **keyword** only, `og_image_*` **integer**, no `page_type` / `indigenous_region` | **Union**: add classifier-only keys; **article_opinion → keyword** (accepts string heuristics from crawler); **og_image_width/height → integer** (index-manager); **detected_content_type → text+keyword** (facets use `.keyword` per platform guidance); add `page_type`, `indigenous_region` |
| `crawled_at` / `published_date` / `classified_at` date format | literal same as ESDateFormat | literal same | **Centralize** on `esmapping.ESDateFormat` |

## classified_content — classification block

| Field / area | Classifier | Index-manager | Resolution |
| --- | --- | --- | --- |
| `content_type` | `keyword` | `keyword` | **text + `.keyword` subfield** + `english_content` analyzer on main field (new indices; aggregations use `.keyword`) |
| `type_confidence`, `type_method`, `processing_time_ms` | On `ClassificationResult` JSON | absent in mapping | **Add** to SSoT |
| `low_quality`, `body`, `source` | On `ClassifiedContent` | absent | **Add** (`body`/`source` publisher aliases) |
| `entertainment`, `rfp`, `need_signal` | Nested objects from classifier domain | absent at top level (recipe/job present) | **Add** nested definitions aligned to `domain` structs |
| `crime` | `street_crime_relevance`, `location_specificity`, `category_pages`, no `primary_crime_type` / `relevance` / `model_version` in *mapping* | `primary_crime_type`, `relevance`, `model_version`, no `street_crime_relevance` | **Union** of property names so strict mapping accepts classifier documents and leaves room for legacy/query fields |
| `mining` | `drill_results` nested, `extraction_method`, observability fields | absent | **Union** — add classifier fields |
| `indigenous` | `region` + observability | no `region` | **Add** `region` + observability |
| `coforge` / `recipe` / `job` | Domain includes observability (`decision_path`, …) | absent in mapping | **Add** optional observability fields |
| Index settings `analysis` | absent (classifier struct mapping) | `english_content` analyzer | **Keep index-manager** for classified indices |
| `title` / `raw_text` analyzers | `standard` | overridden to `english_content` on classified index | **Keep index-manager behaviour** |
| Default replicas in tests | classifier wrappers used `(1,1)` | contracts `(1,0)` | **Unchanged per service** — SSoT functions take `shards, replicas` args |

## Post-migration status

After migration, **one** implementation lives in `infrastructure/esmapping`. Classifier and index-manager `mappings` packages are thin re-exports / wrappers only.

## Out of scope (unchanged by this audit)

- `rfp-ingestor/internal/elasticsearch/mapping.go`
- `data/communities/es-mapping.json`
- Query DSL builders (`query_builder.go`)
