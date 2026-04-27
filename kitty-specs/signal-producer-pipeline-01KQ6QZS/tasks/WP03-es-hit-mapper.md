---
work_package_id: WP03
title: ES-Hit-to-Signal Mapper
dependencies:
- WP01
requirement_refs:
- FR-006
- FR-007
- FR-008
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T010
- T011
- T012
- T013
- T014
agent: "claude:opus-4.7:reviewer:reviewer"
shell_pid: "6680"
history:
- event: created
  at: '2026-04-27T05:55:00Z'
  by: /spec-kitty.tasks
authoritative_surface: signal-producer/internal/mapper/
execution_mode: code_change
mission_id: 01KQ6QZSNBQF3VW515AKM0SQ46
mission_slug: signal-producer-pipeline-01KQ6QZS
owned_files:
- signal-producer/internal/mapper/mapper.go
- signal-producer/internal/mapper/mapper_test.go
tags: []
---

# WP03 — ES-Hit-to-Signal Mapper

## Objective

Convert an Elasticsearch hit into the Waaseyaa `Signal` wire format. Pure data transformation; no I/O, no networking, no logging dependencies. Two content types (`rfp`, `need_signal`); collision-prevention via `external_id` prefix.

## Context

Read first:
- [spec.md](../spec.md) FR-006, FR-007, FR-008.
- [data-model.md](../data-model.md) sections "ESHit" and "Signal".
- [contracts/signals-post.yaml](../contracts/signals-post.yaml) — `Signal` schema is authoritative.

Charter constraints: C-002 (golangci-lint clean — no `interface{}` use `any`, named constants, no shadowing).

## Branch Strategy

Planning base: `main`. Merge target: `main`. Lane workspace from `lanes.json`. Independent leaf WP — runs in parallel with WP02 and WP04 after WP01 lands.

## Subtask Guidance

### T010 — Define `Signal` struct

**Purpose**: Mirror the wire format from `contracts/signals-post.yaml`.

**Steps**:

1. Create `signal-producer/internal/mapper/mapper.go`. Package: `mapper`.
2. Define:
   ```go
   type Signal struct {
       SignalType       string         `json:"signal_type"`
       ExternalID       string         `json:"external_id"`
       Source           string         `json:"source"`
       SourceURL        string         `json:"source_url"`
       Label            string         `json:"label"`
       Strength         int            `json:"strength"`
       OrganizationName string         `json:"organization_name"`
       Sector           string         `json:"sector"`
       Province         string         `json:"province"`
       ExpiresAt        *string        `json:"expires_at,omitempty"`
       Payload          map[string]any `json:"payload"`
   }
   ```
3. Use `any`, not `interface{}` (C-002).
4. Define named constants for the two prefixes:
   ```go
   const (
       externalIDPrefixRFP        = "nc-rfp-"
       externalIDPrefixNeedSignal = "nc-sig-"
       sourceNorthCloud           = "north-cloud"
   )
   ```

**Files**: `signal-producer/internal/mapper/mapper.go`.

**Validation**:

- [ ] Struct fields match the OpenAPI contract exactly.
- [ ] `expires_at` is `*string` with `omitempty`.
- [ ] No `interface{}` anywhere.

### T011 — `MapHit` for `rfp`

**Purpose**: Implement the RFP-side mapping rules from `data-model.md` and #593.

**Steps**:

1. Define the entry point:
   ```go
   func MapHit(hit map[string]any) (Signal, error)
   ```
2. Validate top-level required fields: `_id`, `title`, `quality_score`, `url`, `crawled_at`, `content_type`. Missing → return wrapped error: `fmt.Errorf("mapper: missing required field %q", field)`.
3. Switch on `content_type`:
   - `"rfp"` → call internal `mapRFPHit(hit)`.
   - `"need_signal"` → T012.
   - Other → `fmt.Errorf("mapper: unsupported content_type %q", contentType)`.
4. `mapRFPHit` rules (from `data-model.md`):
   - `signal_type`: literal `"rfp"`.
   - `external_id`: `externalIDPrefixRFP + _id`.
   - `source`: `sourceNorthCloud`.
   - `source_url`: from top-level `url`.
   - `label`: from `title`.
   - `strength`: from `quality_score` (must be int — JSON unmarshal gives `float64` so cast safely).
   - `organization_name`: from `rfp.organization_name`, default `""`.
   - `province`: from `rfp.province`, default `""`.
   - `sector`: first element of `rfp.categories` ([]string), default `""`. (Per #593, "first GSIN category".)
   - `expires_at`: from `rfp.closing_date` if present (pointer set to that value); nil if missing.
   - `payload`: the full hit map.

**Files**: `signal-producer/internal/mapper/mapper.go` (extended).

**Validation**:

- [ ] Maps a complete RFP hit correctly.
- [ ] Missing `rfp.organization_name` produces `OrganizationName: ""`, no error.
- [ ] Missing `rfp.closing_date` produces `ExpiresAt: nil`.
- [ ] Empty `rfp.categories` slice produces `Sector: ""`.

### T012 — `MapHit` for `need_signal`

**Purpose**: Implement the need_signal-side mapping rules.

**Steps**:

1. Internal `mapNeedSignalHit(hit map[string]any) (Signal, error)`:
   - `signal_type`: from `need_signal.signal_type` — required, error if missing.
   - `external_id`: `externalIDPrefixNeedSignal + _id`.
   - `source`: `sourceNorthCloud`.
   - `source_url`: from top-level `url`.
   - `label`: from `title`.
   - `strength`: from `quality_score`.
   - `organization_name`, `province`, `sector`: from `need_signal.organization_name`, `need_signal.province`, `need_signal.sector`. All optional → empty string default.
   - `expires_at`: nil (need_signal has no closing date).
   - `payload`: full hit map.
2. Confirm `external_id` prefix differs from RFP — required for collision prevention (FR-007).

**Files**: `signal-producer/internal/mapper/mapper.go` (extended).

**Validation**:

- [ ] Maps a complete need_signal hit correctly.
- [ ] Missing `need_signal.signal_type` returns a wrapped error.
- [ ] `external_id` starts with `nc-sig-`, not `nc-rfp-`.

### T013 — Tolerance for missing optional fields

**Purpose**: Defensive helper functions so the two `mapHit` paths don't repeat `if v, ok := ...; !ok`.

**Steps**:

1. Add small helpers:
   ```go
   func stringFromPath(hit map[string]any, path ...string) string
   func firstStringInSlice(hit map[string]any, path ...string) string
   func optionalStringFromPath(hit map[string]any, path ...string) *string
   func intFromPath(hit map[string]any, path ...string) int
   ```
2. `path` lets callers traverse `rfp.organization_name` as `("rfp", "organization_name")`. Each helper returns the typed default (empty string, 0, or nil) on any missing/wrong-typed step.
3. Keep helpers in the same file. Functions ≤ 30 lines each (well under funlen 100).
4. Use these helpers in `mapRFPHit` and `mapNeedSignalHit` to avoid repetitive type assertions.

**Files**: `signal-producer/internal/mapper/mapper.go` (extended).

**Validation**:

- [ ] Helpers tolerate every "missing field" path without panicking.
- [ ] No type assertion in the call sites of the two mapHit functions; everything goes through the helpers.

### T014 — Unit tests in `mapper_test.go`

**Purpose**: ≥ 80% coverage on the mapper package per NFR-003. Cover both content types and the missing-field paths.

**Steps**:

1. Create `signal-producer/internal/mapper/mapper_test.go`. Test helpers start with `t.Helper()`.
2. Tests:
   - `TestMapHit_RFP_Complete`: full RFP hit → expected Signal.
   - `TestMapHit_RFP_MissingOptionalFields`: no `rfp.organization_name`/`rfp.province`/`rfp.categories`/`rfp.closing_date` → empty strings, nil expires_at, no error.
   - `TestMapHit_NeedSignal_Complete`: full need_signal hit → expected Signal.
   - `TestMapHit_NeedSignal_MissingSignalType`: returns error.
   - `TestMapHit_MissingTopLevelRequired`: cycle through `_id`, `title`, `quality_score`, `url`, `crawled_at`, `content_type` — each returns a wrapped error mentioning the missing field.
   - `TestMapHit_UnsupportedContentType`: returns error.
   - `TestMapHit_ExternalIDPrefixesAreDistinct`: same `_id`, two different content types → distinct `external_id` values (collision prevention).
3. Use table-driven tests where natural.

**Files**: `signal-producer/internal/mapper/mapper_test.go`.

**Validation**:

- [ ] `task test:signal-producer` passes.
- [ ] Coverage ≥ 80% for `mapper.go`.
- [ ] Distinct-prefix test exists and passes.

## Definition of Done

- [ ] All five subtasks complete with their validation checklists ticked.
- [ ] `task lint:signal-producer` and `task test:signal-producer` green.
- [ ] Coverage ≥ 80% on `mapper.go`.
- [ ] No files modified outside `owned_files`.

## Reviewer Guidance

1. **Wire format match**: Cross-check the `Signal` struct fields against `contracts/signals-post.yaml`. Field names, JSON tags, and types must match exactly.
2. **Pure function**: `MapHit` MUST NOT do I/O, log, call external services, or read globals. Reject if you find any. (`infrastructure/logger` is allowed only via injection — but since this is a pure mapper, no logger is needed at all.)
3. **No `interface{}`**: Search for `interface{}` in the diff. Reject if present (use `any`).
4. **Helpers don't panic on bad shapes**: Run the test suite; the missing-field tests cover this.
5. **Prefix distinction**: One row in the test table proves `nc-rfp-` ≠ `nc-sig-` for the same `_id`. Reject if missing.

## Risks and Mitigations

| Risk                                                                           | Mitigation                                                                                                  |
| ------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------- |
| ES quality_score arrives as `float64` (JSON), but `Signal.Strength` is `int`. | `intFromPath` casts safely; reviewer checks one float64 fixture round-trips correctly.                      |
| `rfp.categories` shape varies (string vs slice).                              | Helper returns empty string on any non-slice or empty-slice case; tested.                                   |
| Future content types appear silently in ES.                                   | Default branch returns wrapped error, not silent skip. Producer logs and counts as `skipped`.              |

## Implementation Command

```bash
spec-kitty agent action implement WP03 --agent <agent-name> --mission signal-producer-pipeline-01KQ6QZS
```

## Activity Log

- 2026-04-27T06:54:29Z – claude:opus-4.7:implementer:implementer – shell_pid=21608 – Started implementation via action command
- 2026-04-27T06:57:24Z – claude:opus-4.7:implementer:implementer – shell_pid=21608 – Mapper ready for review
- 2026-04-27T06:57:54Z – claude:opus-4.7:reviewer:reviewer – shell_pid=6680 – Started review via action command
