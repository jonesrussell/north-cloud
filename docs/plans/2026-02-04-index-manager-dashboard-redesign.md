# Index-Manager Enhancement + Dashboard Redesign

**Date:** 2026-02-04
**Scope:** Option B (Standard) — Expose classifier intelligence in index-manager, redesign dashboard for operator workflows
**Status:** Design Complete

---

## Overview

The classifier now produces rich crime sub-categories and location data, but the index-manager doesn't properly expose these fields. The dashboard is organized by service boundaries instead of operator workflows. This design addresses both gaps.

**Goals:**
1. Surface crime/location intelligence through structured API fields and aggregations
2. Redesign dashboard navigation around operator workflows
3. Build intelligence views that leverage the new data layer

---

## Part 1: Index-Manager Changes

### 1.1 Elasticsearch Mappings

Update `classified_content` mapping to include nested crime and location objects.

**Crime object:**
```
crime:
  sub_label: keyword          # "violent_crime", "property_crime", etc.
  primary_crime_type: keyword # top crime type for aggregations
  relevance: keyword          # "core_street_crime", "peripheral_crime", "not_crime"
  crime_types: keyword[]      # ["violent_crime", "drug_crime"]
  final_confidence: float
  homepage_eligible: boolean
  review_required: boolean
  model_version: keyword
```

**Location object:**
```
location:
  city: keyword
  province: keyword
  country: keyword
  specificity: keyword        # "city", "province", "country", "unknown"
  confidence: float
```

**Removed:** `is_crime_related: boolean` (replaced by `crime.relevance != "not_crime"`)

**Migration:** New mapping applies to new indexes. Existing indexes continue working via dynamic mapping.

---

### 1.2 API Document Type

Update `Document` struct to expose crime and location as first-class fields.

```go
type Document struct {
    // Existing fields (unchanged)
    ID            string         `json:"id"`
    Title         string         `json:"title"`
    URL           string         `json:"url"`
    SourceName    string         `json:"source_name"`
    PublishedDate time.Time      `json:"published_date"`
    CrawledAt     time.Time      `json:"crawled_at"`
    ContentType   string         `json:"content_type"`
    QualityScore  int            `json:"quality_score"`
    Topics        []string       `json:"topics"`
    Body          string         `json:"body"`
    RawText       string         `json:"raw_text,omitempty"`
    RawHTML       string         `json:"raw_html,omitempty"`

    // New structured fields
    Crime    *CrimeInfo    `json:"crime,omitempty"`
    Location *LocationInfo `json:"location,omitempty"`

    // Backward compatibility
    IsCrimeRelated bool `json:"is_crime_related"` // computed from Crime.Relevance

    // Unstructured spillover
    Meta map[string]any `json:"meta,omitempty"`
}

type CrimeInfo struct {
    SubLabel         string   `json:"sub_label"`
    PrimaryCrimeType string   `json:"primary_crime_type"`
    Relevance        string   `json:"relevance"`
    CrimeTypes       []string `json:"crime_types"`
    Confidence       float64  `json:"confidence"`
    HomepageEligible bool     `json:"homepage_eligible"`
    ReviewRequired   bool     `json:"review_required"`
    ModelVersion     string   `json:"model_version,omitempty"`
}

type LocationInfo struct {
    City        string  `json:"city"`
    Province    string  `json:"province"`
    Country     string  `json:"country"`
    Specificity string  `json:"specificity"`
    Confidence  float64 `json:"confidence"`
}
```

---

### 1.3 Document Filters

Extend `DocumentFilters` to support crime and location filtering.

```go
type DocumentFilters struct {
    // Existing filters (unchanged)
    MinQualityScore *int
    MaxQualityScore *int
    Topics          []string
    FromDate        *time.Time
    ToDate          *time.Time
    FromCrawledAt   *time.Time
    ToCrawledAt     *time.Time

    // Crime filters (new)
    CrimeRelevance   []string  // ["core_street_crime", "peripheral_crime"]
    CrimeSubLabels   []string  // ["violent_crime", "property_crime"]
    CrimeTypes       []string  // filter if ANY match
    HomepageEligible *bool
    ReviewRequired   *bool

    // Location filters (new)
    Cities      []string  // ["Toronto", "Vancouver"]
    Provinces   []string  // ["ON", "BC"]
    Countries   []string  // ["Canada"]
    Specificity []string  // ["city", "province"]

    // Source filter (new)
    Sources []string  // ["cbc", "toronto_sun"]
}
```

**Query parameter mapping:**
```
GET /api/v1/documents?crime_relevance=core_street_crime,peripheral_crime
                     &crime_sub_labels=violent_crime
                     &cities=Toronto,Ottawa
                     &provinces=ON
                     &sources=cbc
                     &min_quality=60
```

**ES query translation:**
- Array filters use `terms` queries (OR logic within array)
- Multiple filter types combine with `bool.filter` (AND logic across types)

**Backward compatibility:** Accept `is_crime_related=true` as shorthand for `crime_relevance=core_street_crime,peripheral_crime`

---

### 1.4 Aggregation Endpoints

Three new endpoints to power intelligence views.

#### `GET /api/v1/aggregations/crime`

```json
{
  "by_sub_label": {
    "violent_crime": 1247,
    "property_crime": 892,
    "drug_crime": 456,
    "organized_crime": 123,
    "criminal_justice": 678
  },
  "by_relevance": {
    "core_street_crime": 2891,
    "peripheral_crime": 505,
    "not_crime": 12450
  },
  "by_crime_type": {
    "assault": 523,
    "theft": 412,
    "robbery": 298
  },
  "total_crime_related": 3396,
  "total_documents": 15846
}
```

#### `GET /api/v1/aggregations/location`

```json
{
  "by_country": {
    "Canada": 14200,
    "United States": 1646
  },
  "by_province": {
    "ON": 8450,
    "BC": 2100,
    "AB": 1800
  },
  "by_city": {
    "Toronto": 4200,
    "Vancouver": 1100,
    "Calgary": 950
  },
  "by_specificity": {
    "city": 9800,
    "province": 3200,
    "country": 2100,
    "unknown": 746
  }
}
```

#### `GET /api/v1/aggregations/overview`

```json
{
  "total_documents": 15846,
  "total_crime_related": 3396,
  "top_cities": ["Toronto", "Vancouver", "Calgary"],
  "top_crime_types": ["assault", "theft", "robbery"],
  "quality_distribution": {
    "high": 4200,
    "medium": 8100,
    "low": 3546
  }
}
```

**All endpoints accept DocumentFilters** for scoped aggregations (e.g., "crime distribution in Ontario this week").

---

## Part 2: Dashboard Changes

### 2.1 Menu Redesign

Switch from service-boundary organization to operator-workflow organization.

```
┌─────────────────────────────────────┐
│  NORTH CLOUD                        │
├─────────────────────────────────────┤
│  ▸ Operations                       │
│      Pipeline Monitor               │
│      Recent Articles                │
│      Review Queue                   │
│                                     │
│  ▸ Intelligence                     │
│      Crime Breakdown                │
│      Location Breakdown             │
│      Index Explorer                 │
│                                     │
│  ▸ Content Intake                   │
│      Crawler Jobs                   │
│      Discovered Links               │
│      Rules                          │
│                                     │
│  ▸ Sources                          │
│      All Sources                    │
│      Cities                         │
│      Reputation                     │
│                                     │
│  ▸ Distribution                     │
│      Channels                       │
│      Routes                         │
│      Delivery Logs                  │
│                                     │
│  ▸ System                           │
│      Health                         │
│      Auth                           │
│      Cache                          │
└─────────────────────────────────────┘
```

---

### 2.2 View Inventory

#### New Views

| View | Section | Purpose | Data Source | Complexity |
|------|---------|---------|-------------|------------|
| Crime Breakdown | Intelligence | Crime distribution by sub-label, relevance, type | `/aggregations/crime` | Medium |
| Location Breakdown | Intelligence | Geographic distribution by city/province | `/aggregations/location` | Medium |
| Review Queue | Operations | Articles flagged `review_required=true` | `/documents?review_required=true` | Low |
| Routes | Distribution | Route management (replaces redirect hack) | Publisher API | Low |

#### Renamed/Enhanced Views

| Current | New | Changes |
|---------|-----|---------|
| Pipeline Monitor | Operations → Pipeline Monitor | Add overview aggregations |
| Articles | Operations → Recent Articles | Rename only |
| Indexes | Intelligence → Index Explorer | Rename, add crime/location filters |
| Channels | Distribution → Channels | Move only |
| Delivery Logs | Distribution → Delivery Logs | Move from External Feeds |

#### Removed Views

| View | Reason |
|------|--------|
| Redis Streams | Low value, CLI access sufficient |
| Classifier Stats | Replaced by Crime/Location Breakdown |

---

## Implementation Sequence

### Phase 1: Index-Manager (Foundation)

1. **ES Mappings** — Update `classified_content.go` with crime/location nested objects
2. **Domain Types** — Add `CrimeInfo`, `LocationInfo` structs to `document.go`
3. **Document Service** — Update `mapToDocument()` to deserialize crime/location
4. **Filters** — Extend `DocumentFilters` and query builder
5. **Aggregation Endpoints** — Implement `/crime`, `/location`, `/overview`

### Phase 2: Dashboard Structure

6. **Navigation Config** — Update `navigation.ts` with new menu structure
7. **Router** — Add new routes, update redirects
8. **Sidebar** — Update section rendering

### Phase 3: New Views

9. **Review Queue** — Filtered document list
10. **Routes View** — Basic CRUD table
11. **Crime Breakdown** — Charts + filters + aggregation calls
12. **Location Breakdown** — Charts + filters + aggregation calls

### Phase 4: Enhanced Views

13. **Pipeline Monitor** — Integrate overview aggregations
14. **Index Explorer** — Add crime/location filter controls

---

## Phase 2: Future Enhancements

- Source Health view (ingestion volume, quality trends, error rates per source)

---

## Files to Modify

### Index-Manager
- `/index-manager/internal/elasticsearch/mappings/classified_content.go`
- `/index-manager/internal/domain/document.go`
- `/index-manager/internal/service/document_service.go`
- `/index-manager/internal/elasticsearch/query_builder.go`
- `/index-manager/internal/api/handlers.go`
- `/index-manager/internal/api/routes.go` (new aggregation routes)

### Dashboard
- `/dashboard/src/config/navigation.ts`
- `/dashboard/src/router/index.ts`
- `/dashboard/src/views/operations/ReviewQueueView.vue` (new)
- `/dashboard/src/views/intelligence/CrimeBreakdownView.vue` (new)
- `/dashboard/src/views/intelligence/LocationBreakdownView.vue` (new)
- `/dashboard/src/views/distribution/RoutesView.vue` (new)
- `/dashboard/src/views/operations/PipelineMonitorView.vue` (enhance)
- `/dashboard/src/views/intelligence/IndexExplorerView.vue` (rename + enhance)

---

## Success Criteria

- [ ] Crime/location fields visible in document API responses
- [ ] Filters work for crime sub-labels, cities, provinces
- [ ] Aggregation endpoints return correct counts
- [ ] Dashboard menu reflects new structure
- [ ] Crime Breakdown shows distribution charts
- [ ] Location Breakdown shows geographic distribution
- [ ] Review Queue displays flagged articles
- [ ] Routes view provides full CRUD
- [ ] Pipeline Monitor shows overview stats
