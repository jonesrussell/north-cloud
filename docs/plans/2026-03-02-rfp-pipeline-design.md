# RFP Indexing Pipeline Design

**Date**: 2026-03-02
**Status**: Approved
**Scope**: Full content pipeline for indexing RFPs across North America (Canada first)

## Overview

Add `rfp` as a first-class content type to the North Cloud pipeline. RFPs are crawled from government procurement portals and aggregator platforms, classified with structured metadata extraction (deadline, budget, sector, geography), and published to Redis channels for consumption by MyMe dashboard via both Redis pub/sub and the search API.

## Consumer

MyMe dashboard app — consumes via:
- **Redis pub/sub**: Real-time notifications of new RFPs
- **Search API**: Browsing, filtering, faceted search over the full RFP catalog

## Data Model — RFPResult

```go
type RFPResult struct {
    ExtractionMethod string   `json:"extraction_method"` // "schema_org" | "structured_page" | "heuristic"

    // Core identity
    Title            string   `json:"title,omitempty"`
    ReferenceNumber  string   `json:"reference_number,omitempty"`  // Solicitation/tender number
    OrganizationName string   `json:"organization_name,omitempty"` // Issuing body
    Description      string   `json:"description,omitempty"`

    // Dates
    PublishedDate    string   `json:"published_date,omitempty"`  // ISO 8601
    ClosingDate      string   `json:"closing_date,omitempty"`    // Submission deadline
    AmendmentDate    string   `json:"amendment_date,omitempty"`

    // Budget/value
    BudgetMin        *float64 `json:"budget_min,omitempty"`
    BudgetMax        *float64 `json:"budget_max,omitempty"`
    BudgetCurrency   string   `json:"budget_currency,omitempty"` // "CAD", "USD"

    // Classification
    ProcurementType  string   `json:"procurement_type,omitempty"` // "goods" | "services" | "construction" | "mixed"
    NAICSCodes       []string `json:"naics_codes,omitempty"`
    Categories       []string `json:"categories,omitempty"`       // "IT", "construction", "healthcare"

    // Geography
    Province         string   `json:"province,omitempty"` // Province/state code
    City             string   `json:"city,omitempty"`
    Country          string   `json:"country,omitempty"`  // "CA", "US"

    // Eligibility
    Eligibility      string   `json:"eligibility,omitempty"` // Open/restricted/indigenous set-aside

    // Metadata
    SourceURL        string   `json:"source_url,omitempty"`
    ContactName      string   `json:"contact_name,omitempty"`
    ContactEmail     string   `json:"contact_email,omitempty"`
}
```

## Source Strategy

Target ~20-25 sources, onboarded in tiers after pipeline code is built:

**Tier 1** (clean HTML, highest value):
- buyandsell.gc.ca — federal procurement
- MERX (merx.com) — major aggregator
- 3-4 provincial portals with clean HTML (BC Bid, Ontario, Alberta)

**Tier 2** (remaining provinces + major cities):
- Remaining provincial portals (Quebec SEAO, Saskatchewan, Manitoba, Nova Scotia, NB, NL, PEI)
- Major municipal portals (Toronto, Vancouver, Montreal, Calgary, Ottawa, Edmonton)

**Tier 3** (aggregators + edge cases):
- Biddingo, Bonfire
- Sources requiring special handling (JS-heavy SPAs)

Source onboarding (writing CSS selectors, testing) is a separate effort after pipeline code ships.

## Classification Pipeline

### Step 1 — Content Type Detection

New `content_type_rfp_heuristic.go`:
- High confidence keywords: "request for proposal", "request for tender", "call for tenders", "solicitation notice", "invitation to tender"
- Medium confidence: "procurement", "bid submission", "closing date for submissions", "proposal deadline"
- Threshold: 2+ keyword matches = `rfp` content type at 0.80 confidence

Crawler `content_detector.go` URL patterns: `/rfp/`, `/tenders/`, `/procurement/`, `/solicitations/`, `/bids/`, `/opportunities/`

### Step 2 — Topic Detection

Add `rfp` topic to `classification_rules` table. Sector-specific topics (construction, IT, healthcare) detected naturally by multi-topic system (up to 5 topics per document).

### Step 3 — Quality Scoring

No changes. Existing quality factors apply well to procurement docs.

### Step 4 — RFP Extraction

New `rfp_extractor.go`:
- **Tier 1**: Schema.org JSON-LD (rare for gov sites but checked)
- **Tier 2**: Structured page parsing — labeled fields ("Closing Date:", "Reference Number:", "Estimated Value:")
- **Tier 3**: Heuristic text parsing — regex for dates, dollar amounts, NAICS codes

Gated by `RFP_ENABLED=true`. No ML sidecar needed initially — procurement language is distinctive enough for rules-based extraction.

## Publisher Routing — Layer 11

New `domain_rfp.go`:

| Channel | Filter logic |
|---------|-------------|
| `content:rfps` | All docs with `rfp` content type |
| `rfp:country:{code}` | `rfp.country` field ("ca", "us") |
| `rfp:province:{code}` | `rfp.province` field ("on", "bc", "ab") |
| `rfp:sector:{slug}` | `rfp.categories` array ("it", "construction") |
| `rfp:type:{slug}` | `rfp.procurement_type` ("goods", "services") |

Add `rfp` to `layer1SkipTopics` to prevent generic topic routing. No quality score gate — all RFPs published regardless of quality score.

## Elasticsearch Mapping

Nested object added to `classified_content`:

```
rfp: {
  type: object
  properties:
    extraction_method:  keyword
    title:              text (standard analyzer)
    reference_number:   keyword
    organization_name:  keyword
    description:        text (standard analyzer)
    published_date:     date
    closing_date:       date
    amendment_date:     date
    budget_min:         float
    budget_max:         float
    budget_currency:    keyword
    procurement_type:   keyword
    naics_codes:        keyword (array)
    categories:         keyword (array)
    province:           keyword
    city:               keyword
    country:            keyword
    eligibility:        keyword
    source_url:         keyword
    contact_name:       keyword
    contact_email:      keyword
}
```

Key MyMe queries: filter by province + sector + budget range, range on closing_date for "expiring soon", faceted aggregations on province/categories/procurement_type.

## Files Changed

### New files (6)

| File | Purpose |
|------|---------|
| `classifier/internal/classifier/rfp_extractor.go` | Schema.org + structured page + heuristic extraction |
| `classifier/internal/classifier/rfp_extractor_test.go` | Extractor tests |
| `classifier/internal/classifier/content_type_rfp_heuristic.go` | Keyword-based content type detection |
| `classifier/internal/classifier/content_type_rfp_heuristic_test.go` | Heuristic tests |
| `publisher/internal/router/domain_rfp.go` | Layer 11 routing |
| `publisher/internal/router/domain_rfp_test.go` | Routing tests |

### Modified files (9)

| File | Change |
|------|--------|
| `classifier/internal/domain/classification.go` | Add `RFPResult` struct, `ContentTypeRFP` constant, `RFP` field on `ClassificationResult` and `ClassifiedContent` |
| `classifier/internal/config/config.go` | Add `RFPConfig` with `Enabled` env var |
| `classifier/internal/bootstrap/classifier.go` | Conditionally create `RFPExtractor` |
| `classifier/internal/classifier/classifier.go` | Add `rfpExtractor` field, wire extraction call, add `runRFPExtraction()` |
| `classifier/internal/elasticsearch/mappings/classified_content.go` | Add RFP nested object mapping |
| `crawler/internal/crawler/content_detector.go` | Add RFP URL patterns |
| `publisher/internal/router/service.go` | Add `RFP` field to `ContentItem`, register Layer 11 |
| `publisher/internal/router/domain_topic.go` | Add `rfp` to `layer1SkipTopics` |
| `docker-compose.base.yml` | Add `RFP_ENABLED` env var to classifier |

### No new services, containers, or databases.

## Environment Variables

- `RFP_ENABLED=true` — gates the RFP extractor in classifier service
