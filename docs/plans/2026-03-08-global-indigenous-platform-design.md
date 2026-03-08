# Global Indigenous Content Platform — Implementation (D0)

**Date:** 2026-03-08
**Status:** Implemented
**Branch:** `claude/global-indigenous-platform-d0ff45dc`
**Design Doc:** `docs/plans/2026-03-08-global-indigenous-design.md`

---

## Summary

Expands NorthCloud's indigenous content pipeline from Canada-only to global coverage across 7 regions, 10 content categories, and 7+ languages. This is a classifier-first expansion — source onboarding is a separate milestone.

**Scope:** 571 insertions, 36 deletions across 20 files in 5 services.

---

## Implementation Units

### Unit 1: Source-Manager Column + Crawler Passthrough

**Files:**
- `source-manager/migrations/008_add_indigenous_region.up.sql` (new)
- `source-manager/migrations/008_add_indigenous_region.down.sql` (new)
- `source-manager/internal/models/source.go`
- `source-manager/internal/repository/source.go`
- `crawler/internal/sources/apiclient/types.go`
- `crawler/internal/sources/types/types.go`
- `crawler/internal/sources/apiclient/converter.go`
- `crawler/internal/content/rawcontent/service.go`

**Changes:**
- Added `indigenous_region TEXT` nullable column to `sources` table
- Updated all 7 SQL operations in repository (Create, GetByID, GetByIdentityKey, ListPaginated, List, Update, UpsertSource) and `scanSourceRow`
- Added `IndigenousRegion` to `APISource`, `SourceConfig`, and converter
- Crawler passes `indigenous_region` through to `meta.indigenous_region` in Elasticsearch raw_content documents

### Unit 2: ML Sidecar Multilingual Expansion

**Files:**
- `ml-sidecars/indigenous-ml/classifier/relevance.py`
- `ml-sidecars/indigenous-ml/main.py`
- `ml-sidecars/indigenous-ml/tests/__init__.py` (new)
- `ml-sidecars/indigenous-ml/tests/test_relevance.py` (new)

**Changes:**
- Replaced 6 Canada-centric categories with 10 global categories
- Expanded CORE_PATTERNS from 6 to 19 patterns across 7 languages
- Expanded PERIPHERAL_PATTERNS from 3 to 5
- Bumped `MODEL_VERSION` to `2026-03-08-indigenous-v2`
- Added 28 tests across 8 test classes

### Unit 3: Go Classifier Global Patterns

**Files:**
- `classifier/internal/classifier/indigenous_rules.go`
- `classifier/internal/classifier/indigenous_rules_test.go` (new)

**Changes:**
- Expanded `indigenousCorePatterns` from 6 to 19 compiled regexes (parity with Python)
- Expanded `indigenousPeripheralPatterns` from 3 to 5
- Added 10 test functions covering all language groups

### Unit 4: Classifier Region Passthrough

**Files:**
- `classifier/internal/domain/classification.go`
- `classifier/internal/classifier/indigenous.go`
- `classifier/internal/classifier/indigenous_test.go`

**Changes:**
- Added `Region string` to `IndigenousResult` struct
- Reads `raw.Meta["indigenous_region"]` and sets on classification result
- Region flows into ES `classified_content` documents via existing `BuildClassifiedContent` path
- Added 2 tests for region passthrough (present and empty)

### Unit 5: Publisher Region Routing

**Files:**
- `publisher/internal/router/content_item.go`
- `publisher/internal/router/indigenous.go`
- `publisher/internal/router/indigenous_test.go`

**Changes:**
- Added `Region string` to `IndigenousData` struct
- Routes to `indigenous:region:{region}` channels when region is non-empty
- Added 2 tests for region routing (with region and without)

---

## Taxonomies

### Region Taxonomy (7 regions)

| Tag | Coverage |
|-----|----------|
| `canada` | First Nations, Metis, Inuit |
| `us` | Native American, Alaska Native, Native Hawaiian |
| `latin_america` | Maya, Quechua, Mapuche, Guarani, Wayuu, Amazonian peoples |
| `oceania` | Aboriginal Australian, Torres Strait Islander, Maori, Pacific Islander |
| `europe` | Sami, Basque, Roma |
| `asia` | Ainu, Adivasi, Tibetan, Hmong, indigenous Taiwanese |
| `africa` | San, Maasai, Pygmy/Batwa, Amazigh/Berber, Ogiek |

### Content Category Taxonomy (10 categories)

`culture`, `language`, `land_rights`, `environment`, `sovereignty`, `education`, `health`, `justice`, `history`, `community`

---

## End-to-End Data Flow

```
Source-Manager DB                Crawler                          Classifier                    Publisher
─────────────────               ───────                          ──────────                    ─────────
sources.indigenous_region  →  APISource.IndigenousRegion  →  raw.Meta["indigenous_region"]  →  IndigenousData.Region
        (TEXT)                   (*string)                    (map[string]any)                  (string)
                                     │                              │                              │
                                     ▼                              ▼                              ▼
                              SourceConfig.IndigenousRegion   IndigenousResult.Region      indigenous:region:{slug}
                                   (string)                       (string)                   (Redis channel)
```

**Classification flow:**
1. Crawler fetches page, extracts content, writes `meta.indigenous_region` to ES `raw_content`
2. Classifier reads raw_content, runs hybrid rule+ML classification (Go patterns + Python sidecar)
3. Go classifier reads `meta.indigenous_region`, passes through to `IndigenousResult.Region`
4. Result written to ES `classified_content` with region field
5. Publisher reads classified_content, routes to `content:indigenous` + `indigenous:category:{cat}` + `indigenous:region:{region}`

---

## Testing Strategy

### Go Services
- `cd SERVICE && GOWORK=off go test ./...`
- `cd SERVICE && GOWORK=off golangci-lint run`
- All 4 Go services pass lint and tests

### Python ML Sidecar
- `cd ml-sidecars/indigenous-ml && python3 -m pytest tests/`
- 28 tests across 8 classes covering all 7 language groups

### Pattern Parity
Go (`indigenous_rules.go`) and Python (`relevance.py`) maintain identical pattern sets:
- 19 core patterns (same languages, same terms)
- 5 peripheral patterns (same terms)
- Same `MAX_BODY_CHARS = 500` truncation

---

## Model Versioning

| Version | Date | Changes |
|---------|------|---------|
| `2026-02-27-indigenous-v1` | 2026-02-27 | Initial: 6 Canada-centric categories, English-only |
| `2026-03-08-indigenous-v2` | 2026-03-08 | Global: 10 categories, 7 languages, 19 core patterns |

---

## Future Work

- **People-level tagging**: Add `indigenous_people` field (e.g., `maori`, `sami`, `ainu`) for finer-grained routing
- **Auto-detection**: Detect indigenous content from non-indigenous sources (mainstream news covering indigenous topics)
- **Region auto-inference**: Infer region from content patterns when source doesn't have explicit `indigenous_region`
- **Source onboarding**: Load sources for all 7 regions (see design doc for source research)
- **Grafana dashboard**: Indigenous content metrics by region, category, language
- **Content quality monitoring**: Extraction success rates per region, per source
