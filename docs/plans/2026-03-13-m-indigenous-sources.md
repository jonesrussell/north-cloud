# M-Indigenous-Sources: Global Indigenous Source Onboarding

**Status**: Implementing
**Date**: 2026-03-13
**Depends on**: D0 (Global Indigenous Design), D1 (Region Taxonomy), D2 (Category Taxonomy), M-Indigenous-Classifier

## Goal

Onboard 150–250 global indigenous media outlets as NorthCloud sources, with correct region tagging, language metadata, and render-mode defaults. This is the data-seeding companion to the classifier expansion — sources produce content that flows through the multilingual indigenous pipeline.

## Region Taxonomy (from D1)

7 canonical regions defined in `infrastructure/indigenous/region.go`:

| Slug | Peoples | Example sources |
|------|---------|-----------------|
| `canada` | First Nations, Métis, Inuit | APTN, Windspeaker, Nation Talk |
| `us` | Native American, Alaska Native, Native Hawaiian | Indian Country Today, Navajo Times |
| `latin_america` | Maya, Quechua, Mapuche, Guaraní | Servindi, AIPIN, Mapuexpress |
| `oceania` | Aboriginal Australian, Torres Strait Islander, Māori | NITV, Te Ao Māori News, Koori Mail |
| `europe` | Sámi, Basque, Roma | Ságat, YLE Sápmi |
| `asia` | Ainu, Adivasi, indigenous Taiwanese | Adivasi Resurgence, Taiwan Indigenous TV |
| `africa` | San, Maasai, Amazigh | IWGIA Africa, Amazigh World News |

Normalization: `NormalizeRegionSlug()` handles mixed case, spaces, and hyphens → underscore canonical form.

## Category Taxonomy (from D2)

10 global categories used by the classifier and publisher:

`culture`, `language`, `land_rights`, `environment`, `sovereignty`, `education`, `health`, `justice`, `history`, `community`

Content classified into these categories routes to `indigenous:category:{slug}` Redis channels.

## Source Selection Criteria

Sources are selected based on three tiers:

1. **Indigenous-owned**: Media organizations owned and operated by indigenous peoples or nations (e.g., APTN, Indian Country Today, NITV)
2. **Indigenous-governed**: Organizations with indigenous editorial boards or governance structures (e.g., cultural foundations, tribal media offices)
3. **Indigenous-serving**: Non-indigenous organizations with dedicated indigenous coverage desks (e.g., CBC Indigenous, The Guardian's First Nations coverage)

Priority: indigenous-owned > indigenous-governed > indigenous-serving.

## Crawlability Assessment

Each source is assessed for crawlability before onboarding:

| Factor | Static (`render_mode: static`) | Dynamic (`render_mode: dynamic`) |
|--------|-------------------------------|----------------------------------|
| Server-rendered HTML | Yes | N/A |
| JavaScript-rendered content | No | Yes (Playwright) |
| RSS/Atom feed available | Preferred for `ingestion_mode: feed` | Not required |
| Rate limit sensitivity | Default 10 req/min | Default 5 req/min (heavier) |
| Max depth | 2 (standard) | 1 (minimize render load) |

### Render-Mode Defaults

- **Static** (default): Standard HTTP fetch. Used for WordPress, Hugo, Drupal, and other server-rendered sites.
- **Dynamic**: Playwright render worker. Used for React/Vue/Angular SPAs, sites with heavy JS content loading. Requires `CRAWLER_RENDER_WORKER_URL` to be configured.

Most indigenous media outlets use WordPress or similar CMS platforms → default to `static`. Dynamic rendering is reserved for sites confirmed to require JS rendering.

## Source Data Format

Sources are stored in `scripts/global-indigenous-sources.json`:

```json
[
  {
    "name": "APTN News",
    "homepage": "https://www.aptnnews.ca",
    "rss": "https://www.aptnnews.ca/feed/",
    "region": "canada",
    "country": "CA",
    "language": "en",
    "render_mode": "static"
  }
]
```

Fields:
- `name` (required): Human-readable source name, unique across all sources
- `homepage` (required): Base URL for the crawler
- `rss` (optional): RSS/Atom feed URL for feed-based ingestion
- `region` (required): One of the 7 canonical region slugs
- `country` (required): ISO 3166-1 alpha-2 country code
- `language` (required): Primary content language (ISO 639-1)
- `render_mode` (required): `static` or `dynamic`

## Import Mechanism

New API endpoint on the source-manager: `POST /api/v1/sources/import-indigenous`

- Accepts the JSON file contents as the request body (array of source objects)
- Validates each source (name, URL, region, render_mode)
- Converts to `models.Source` with appropriate defaults:
  - `Enabled: true`
  - `RateLimit: "10s"` (static) or `"12s"` (dynamic)
  - `MaxDepth: 2` (static) or `1` (dynamic)
  - `IngestionMode: "feed"` if RSS is provided, `"standard"` otherwise
  - `IndigenousRegion`: set from source's `region` field
  - `RenderMode`: set from source's `render_mode` field
  - `FeedURL`: set from source's `rss` field (if provided)
- Upserts via existing `UpsertSourcesTx` (keyed on `name`)
- Returns created/updated counts

## Pipeline Flow

```
global-indigenous-sources.json
    → POST /api/v1/sources/import-indigenous
    → source-manager DB (with indigenous_region, render_mode)
    → crawler picks up sources (respects render_mode: static vs dynamic)
    → raw_content indexed to ES (meta.indigenous_region passthrough)
    → classifier processes (multilingual v3 patterns, confidence scoring)
    → classified_content to ES (indigenous.relevance, categories, region, confidence)
    → publisher routes (confidence >= 0.35 gate)
        → content:indigenous (catch-all)
        → indigenous:category:{slug} (per category)
        → indigenous:region:{slug} (per region)
```

## Testing Strategy

- **Source-manager**: Unit tests for JSON parsing, validation, and source conversion
- **Seed data**: `TestGlobalIndigenousSourcesSeedFile` validates `scripts/global-indigenous-sources.json` parses, each source passes importer validation, and source names remain unique
- **Crawler**: Existing render_mode tests confirm static/dynamic routing (no new code needed)
- **Publisher**: Existing region routing tests cover all 7 regions (verified by `TestAllowedRegionsCount`)
- **Integration**: Manual verification via MCP tools after deployment

## 2026-04-25 Feed Coverage Update (#631)

Issue #631 identified several indigenous sources that relied on crawl-only ingestion because they had no configured feed URL. CBC Indigenous, Windspeaker, and First Nations Drum remain blocked on proxy routing because production fetches were network-blocked or forbidden. A focused feed discovery pass found a live Cronkite News Indigenous category RSS endpoint, so `scripts/global-indigenous-sources.json` now includes `Cronkite News Indigenous` with `rss` set to `https://cronkitenews.azpbs.org/category/indigenous/feed/`. Importing the seed file will onboard it as a feed-polled indigenous source with the standard 60-minute poll interval.

## Future Extensions

### Auto-Discovery
- Leverage the existing `AllowSourceDiscovery` field to discover new indigenous sources from outlinks
- Source Identity Resolver can match discovered URLs to existing sources via `IdentityKey`

### People-Level Tagging
- Add `indigenous_nations` array field to source metadata (e.g., ["anishinaabe", "cree", "mohawk"])
- Enables nation-specific content feeds beyond the region/category taxonomy

### Language Auto-Detection
- The classifier's `language_detected` field (from M-Indigenous-Classifier) already tracks content language
- Future: aggregate detected languages per source to validate the `language` field in the source JSON

### Feed Health Monitoring
- RSS feeds can go stale or return errors
- The existing `feed_disabled_at` / `feed_disable_reason` mechanism handles this automatically
- Dashboard can surface sources with disabled feeds for editorial attention
