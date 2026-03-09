# D1: Indigenous Region Taxonomy Finalization

**Date:** 2026-03-10
**Status:** Implemented
**Depends on:** D0 (Global Indigenous Content Platform)
**Closes:** #210

---

## Canonical Region Taxonomy

7 regions covering all indigenous peoples globally. Slugs are lowercase, underscore-separated, and validated at write time.

| Slug | Coverage |
|------|----------|
| `canada` | First Nations, Metis, Inuit |
| `us` | Native American, Alaska Native, Native Hawaiian |
| `latin_america` | Maya, Quechua, Mapuche, Guarani, Wayuu, Amazonian peoples |
| `oceania` | Aboriginal Australian, Torres Strait Islander, Maori, Pacific Islander |
| `europe` | Sami, Basque (indigenous context), Roma (indigenous context) |
| `asia` | Ainu, Adivasi, Tibetan, Hmong, indigenous Taiwanese |
| `africa` | San, Maasai, Pygmy/Batwa, Amazigh/Berber, Ogiek |

---

## Normalization Rules

All region values are normalized before storage and routing:

1. Trim whitespace
2. Lowercase
3. Replace spaces and hyphens with underscores
4. Validate against the canonical slug set

Invalid region values are rejected at the source-manager API level (HTTP 400).

The normalization function `NormalizeRegionSlug` lives in `infrastructure/indigenous/region.go` so all services share the same logic.

---

## Source-Manager Mapping Rules

- `indigenous_region` is an optional nullable TEXT column on the `sources` table
- Only valid region slugs are accepted (validated on Create and Update)
- Empty/null means the source is not indigenous or region is unknown
- Sources can be queried by region for backfill and reporting

---

## Publisher Routing Rules

The `IndigenousDomain` router normalizes the region slug before channel generation:

```
indigenous:region:{normalized_slug}
```

Examples:
- `indigenous:region:canada`
- `indigenous:region:oceania`
- `indigenous:region:latin_america`

Mixed-case or space-separated inputs (e.g., `"Latin America"`, `"OCEANIA"`) are normalized to the correct slug before routing.

---

## Backward Compatibility

- Existing Canadian sources continue to work unchanged
- The `content:indigenous` catch-all channel is unaffected
- `indigenous:category:{slug}` channels are unaffected
- Empty region values produce no region channel (same as before D0)

---

## Future Extensions

- **People-level tags**: `indigenous_people` field (e.g., `maori`, `sami`, `ainu`) for finer-grained routing. Would produce `indigenous:people:{slug}` channels.
- **Subregions**: If needed, regions can be split (e.g., `oceania` → `australia`, `aotearoa`, `pacific_islands`). The slug set is extensible.
- **Region auto-inference**: Infer region from classifier patterns when source lacks explicit `indigenous_region`.

---

## Implementation

### Shared Package: `infrastructure/indigenous/`

- `region.go`: `AllowedRegions` set, `NormalizeRegionSlug(string) (string, error)`, `IsValidRegion(string) bool`
- `region_test.go`: tests for normalization and validation

### Source-Manager

- Validates `indigenous_region` on Create and Update using `IsValidRegion`
- Rejects invalid values with HTTP 400

### Crawler

- Normalizes region from source config before writing to `meta.indigenous_region`
- Already passes through via `getSourceConfig` → `convertToRawContent`

### Publisher

- `IndigenousDomain.Routes()` normalizes region slug before generating channel name
- Handles mixed-case and whitespace gracefully
