# D2: Indigenous Category Taxonomy Expansion

**Status**: Implemented
**Date**: 2026-03-11
**Depends on**: D0 (Global Indigenous Content Platform), D1 (Region Taxonomy Finalization)

## Overview

Expand the indigenous content category taxonomy from the original 6 Canada-centric categories to 10 globally applicable categories. This milestone defines the canonical category set, adds placeholder keyword arrays for future multilingual pattern population (D2.1), and ensures backward compatibility.

## Canonical Categories (10)

| Slug | Definition | Example content |
|------|-----------|-----------------|
| `culture` | Ceremonies, art, music, dance, traditional practices, cultural preservation | Powwow, corroboree, haka, dreamtime stories |
| `language` | Language revitalization, education, documentation, endangered languages | Te Reo Māori revival, Anishinaabemowin immersion |
| `land_rights` | Territory disputes, land claims, demarcation, resource rights | Terra indígena demarcation, treaty land entitlements |
| `environment` | Climate, water rights, pipeline opposition, deforestation, conservation | Standing Rock, Amazon deforestation, water protectors |
| `sovereignty` | Self-determination, governance, treaties, political autonomy | Band council resolutions, tribal sovereignty rulings |
| `education` | Schools, residential school legacy, indigenous education programs | TRC recommendations, indigenous curriculum reform |
| `health` | Indigenous health disparities, traditional medicine, mental health | Boil water advisories, traditional healing programs |
| `justice` | MMIWG, incarceration, policing, legal rights, human rights | Missing and murdered inquiry, overrepresentation |
| `history` | Colonial history, decolonization, historical events, reconciliation | Residential school discoveries, treaty signing dates |
| `community` | Elders, youth, family, community events, social programs | Community gatherings, elder consultations, youth programs |

## Backward Compatibility

The original 6 categories (`anishinaabe`, `culture`, `language`, `governance`, `land_rights`, `education`) map to the new taxonomy:

| Old category | New category | Notes |
|-------------|-------------|-------|
| `anishinaabe` | (removed) | Nation-specific; use `culture` + region instead |
| `culture` | `culture` | Unchanged |
| `language` | `language` | Unchanged |
| `governance` | `sovereignty` | Renamed for global applicability |
| `land_rights` | `land_rights` | Unchanged |
| `education` | `education` | Unchanged |

New categories added: `environment`, `health`, `justice`, `history`, `community`.

Existing classified content with `governance` will continue to route correctly — the publisher converts categories to slugs dynamically and does not validate against a fixed list.

## Classifier Mapping Rules

### Python ML Sidecar (`relevance.py`)

Each category has a keyword list in `CATEGORY_KEYWORDS`. Keywords are checked via substring match against lowercased text. The sidecar emits up to `MAX_CATEGORIES = 5` matched categories.

Placeholder keyword arrays are defined for each category. Full multilingual keyword population is deferred to D2.1.

### Go Classifier (`indigenous_rules.go`)

The Go classifier does not perform category extraction — it only determines relevance (core/peripheral/not). Category assignment is the ML sidecar's responsibility. The Go rules file defines category constants for documentation and cross-referencing.

## Multilingual Keyword Sets (D2.1)

Each category will have keywords in 7 languages: English, Spanish, French, Portuguese, Nordic, Te Reo Māori, Japanese. D2 defines the category structure; D2.1 populates the keyword arrays with domain-expert-reviewed terms.

## Publisher Routing

The publisher's `IndigenousDomain.Routes()` already handles categories generically:

```go
slug := strings.ToLower(strings.ReplaceAll(cat, " ", "-"))
channels = append(channels, "indigenous:category:"+slug)
```

No publisher code changes are needed — any category string from the classifier is automatically routed to `indigenous:category:{slug}`.

## Future Extensions

- **Hierarchical categories**: e.g., `culture/art`, `culture/ceremony` — not needed until content volume justifies it
- **People-level categories**: e.g., `anishinaabe`, `maori`, `ainu` — consider as tags rather than categories to avoid combinatorial explosion
- **Cross-category content**: A single article can match multiple categories (up to `MAX_CATEGORIES = 5`); no additional logic needed
