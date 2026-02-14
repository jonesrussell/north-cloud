# Topic-Agnostic Location Routing

**Date**: 2026-02-14
**Status**: Approved

## Problem

`GenerateLocationChannels()` in `publisher/internal/router/crime.go` only generates location channels for crime articles. Mining and entertainment articles with location data get no geographic routing. This is a vestige of the project's crime-only origins.

## Design

### New file: `location.go`

A topic-agnostic `GenerateLocationChannels(article *Article) []string` that:

1. Checks if the article has valid location data (skips unknown/empty country)
2. Determines which domain classifiers are active on this article:
   - Crime: `CrimeRelevance` not `not_crime` / empty
   - Mining: `Mining.Relevance` not `not_mining` / empty
   - Entertainment: `Entertainment.Relevance` not `not_entertainment` / empty
3. For each active prefix, generates geographic channels:
   - `{prefix}:local:{city}` (Canadian city-level)
   - `{prefix}:province:{code}` (Canadian province-level)
   - `{prefix}:canada` (Canadian national)
   - `{prefix}:international` (non-Canadian)

### Channel format

Same as existing crime channels — no breaking change for crime consumers:
- `crime:local:toronto`, `crime:province:on`, `crime:canada`, `crime:international`

New channels for other domains:
- `mining:local:sudbury`, `mining:province:on`, `mining:canada`, `mining:international`
- `entertainment:local:toronto`, `entertainment:canada`, etc.

### Files changed

| File | Action |
|------|--------|
| `publisher/internal/router/location.go` | Create — topic-agnostic location routing |
| `publisher/internal/router/location_test.go` | Create — tests for all topic combinations |
| `publisher/internal/router/crime.go` | Remove `GenerateLocationChannels` and location constants |
| `publisher/internal/router/crime_test.go` | Move location tests to `location_test.go`, update pipeline test |
| `publisher/CLAUDE.md` | Update Layer 4 description |

### What stays the same

- `service.go` Layer 4 call: `GenerateLocationChannels(article)` — same signature
- Mining's own `mining:canada`/`mining:international` from `appendMiningLocationChannel` — uses mining-specific `Mining.Location` field, not content-based location. Additive, not duplicative.
- Crime channel format — no breaking change
