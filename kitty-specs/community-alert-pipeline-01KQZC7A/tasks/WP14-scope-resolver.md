---
work_package_id: WP14
title: Scope Resolver
dependencies:
- WP05
- WP06
- WP07
requirement_refs:
- C-003
- FR-009
- FR-010
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T057
- T058
- T059
- T060
phase: B
agent: "claude:sonnet:implementer:implementer"
shell_pid: "273598"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: alert-crawler/internal/scope/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- alert-crawler/internal/scope/**
priority: P2
tags: []
---

# WP14 — Scope Resolver

## Objective

Resolve `scope` tokens for an Alert by combining source-default tokens with content-derived tokens (location-name → city slug). Imports `taxonomy.ParentRegion` for hierarchy walks. Logs (does NOT fail) when an unknown taxonomy token is encountered (Edge-04).

## Context

- Spec §3 FR-009, FR-010, §5 C-003
- Plan §Component Design (Scope resolver), §TC-007, §TC-015
- Research R-004 (taxonomy package extension; ParentRegion to be added in WP03)

## Branch Strategy

Standard. Functionally depends on WP03's `ParentRegion`; operates via `replace` directive during parallel dev.

## Subtasks

### T057 — Create `internal/scope/resolver.go`

**Purpose**: Resolver type that wraps the indigenous-taxonomy package.

**Steps**:
1. Create `alert-crawler/internal/scope/resolver.go`:
   ```go
   package scope

   import (
       "strings"

       "github.com/jonesrussell/indigenous-taxonomy/generated/go/taxonomy"

       "github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
   )

   type Resolver struct {
       defaultByCity map[string]taxonomy.Region
   }

   func New() *Resolver {
       return &Resolver{
           defaultByCity: map[string]taxonomy.Region{
               "winnipeg":  taxonomy.CanadaManitobaWinnipeg,
               "toronto":   taxonomy.CanadaOntarioToronto,
               "ottawa":    taxonomy.CanadaOntarioOttawa,
               "vancouver": taxonomy.CanadaBritishColumbiaVancouver,
               "calgary":   taxonomy.CanadaAlbertaCalgary,
               "saskatoon": taxonomy.CanadaSaskatchewanSaskatoon,
           },
       }
   }

   // Resolve returns the final scope token list for an alert, combining
   // source defaults with location-derived tokens.
   func (r *Resolver) Resolve(source domain.AlertSource, locationHint string) []string { /* ... */ }
   ```
2. Note: constants like `taxonomy.CanadaManitobaWinnipeg` will exist after WP03 regenerates `regions.go`. During parallel dev, the `replace` directive points at the local checkout; commits in WP02/WP03 must land before WP14's tests can pass.

**Files**:
- `alert-crawler/internal/scope/resolver.go` (new, ~120 lines).

### T058 — Default-scope expansion per source

**Purpose**: Each source carries `DefaultScope []string` in config (e.g., `[treaty:1, canada:manitoba]`). Resolver expands these into a deduplicated list.

**Steps**:
1. In `Resolve`:
   ```go
   func (r *Resolver) Resolve(source domain.AlertSource, locationHint string) []string {
       seen := map[string]struct{}{}
       result := []string{}
       add := func(token string) {
           if _, dup := seen[token]; dup {
               return
           }
           seen[token] = struct{}{}
           result = append(result, token)
       }

       // 1. Source defaults
       for _, t := range source.DefaultScope {
           add(t)
       }

       // 2. Content-derived: location-name → city slug
       if hint := strings.ToLower(strings.TrimSpace(locationHint)); hint != "" {
           if city, ok := r.defaultByCity[hint]; ok {
               add(string(city))
               // Walk up the hierarchy to add parent tokens.
               current := city
               for {
                   parent, ok := taxonomy.ParentRegion(current)
                   if !ok {
                       break
                   }
                   add(string(parent))
                   current = parent
               }
           }
       }

       return result
   }
   ```
2. The walk-up ensures consumers configured for any ancestor (Treaty 1, manitoba, canada) still receive the alert via FR-010.

**Files**:
- `alert-crawler/internal/scope/resolver.go` (already, ~60 lines added).

### T059 — Location-name → city slug inference

**Purpose**: Map free-text "Winnipeg" or "Toronto" found in alert titles/descriptions to canonical city slugs.

**Steps**:
1. Already partially in T057's `defaultByCity` map.
2. The location hint is provided by the parser (WP09's `ExtractLocation`). The runner (WP15) passes it to `Resolve`.
3. Edge cases:
   - **Unknown city**: not in the map; resolver does NOT add a token (and emits a metric `alert_crawler.scope.unknown_location_total`).
   - **Multiple locations** in description: parser surfaces only the primary; resolver does not multiplex.
   - **Location is not a city** (e.g., "Manitoba" province-level): resolver should still match if added to a future `defaultByProvince` map; for v1, only city-level mapping.

**Files**:
- `alert-crawler/internal/scope/resolver.go` (already).

### T060 — Unit tests

**Purpose**: Hierarchy resolution, missing token handling, deduplication.

**Steps**:
1. Create `alert-crawler/internal/scope/resolver_test.go`:
   - **TestResolve_DefaultsOnly**: source with `[treaty:1, canada:manitoba]` and empty locationHint returns `["treaty:1", "canada:manitoba"]`.
   - **TestResolve_LocationAddsCity**: source with default `[canada:manitoba]` and hint `"Winnipeg"` returns `["canada:manitoba", "canada:manitoba:winnipeg", "canada"]` (city + walk-up).
   - **TestResolve_DeduplicatesParents**: source default `[canada:manitoba, canada]` and hint `"Winnipeg"` returns `["canada:manitoba", "canada", "canada:manitoba:winnipeg"]` (walk-up dedupes).
   - **TestResolve_UnknownLocationNoOp**: hint `"Atlantis"` returns the source defaults only.
   - **TestResolve_LowerCaseAndTrim**: hint `"  WINNIPEG  "` matches `winnipeg`.
   - **TestResolve_EmptySource**: defaults empty, no hint → returns empty.
2. `t.Helper()`. Coverage ≥80%.

**Files**:
- `alert-crawler/internal/scope/resolver_test.go` (new, ~150 lines).

**Validation**:
- All tests pass once WP02 and WP03 have committed and the regenerated `regions.go` is on disk (via the `replace` directive).
- Coverage ≥80%.

## Definition of Done

- Resolver combines source defaults + location hint with hierarchy walk.
- Unknown location is a metric, not an error.
- Deduplication via map.
- Coverage ≥80%.

## Risks

- **RR-005**: alert-crawler is the first NC service to import `indigenous-taxonomy`. WP05's `replace` directive bridges the gap. Verify the imports compile before WP14's other code paths come online.
- **City name ambiguity**: "Vancouver" (BC) and "Vancouver" (WA) — out of v1 scope; the source default constrains country to Canada.

## Reviewer Guidance

- Verify the walk-up correctly stops at top-level (`canada` has no parent).
- Verify deduplication behavior.
- Verify the resolver does NOT panic on unknown taxonomy constants (graceful degradation).
- Verify the `ParentRegion` is imported from the package (not duplicated locally).

## Implementation Command

```bash
spec-kitty agent action implement WP14 --agent <name>
```

Depends on WP05, WP06, WP07. Functionally depends on WP02 and WP03 in the sibling repo (via `replace` directive).

## Activity Log

- 2026-05-06T23:29:08Z – claude:sonnet:implementer:implementer – shell_pid=273598 – Started implementation via action command
- 2026-05-06T23:35:43Z – claude:sonnet:implementer:implementer – shell_pid=273598 – Scope resolver via taxonomy.ParentRegion
