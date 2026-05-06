---
work_package_id: WP13
title: Severity Inference
dependencies:
- WP05
- WP06
- WP07
requirement_refs:
- FR-002
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T053
- T054
- T055
- T056
phase: B
agent: "claude:sonnet:implementer:implementer"
shell_pid: "270050"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: alert-crawler/internal/severity/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- alert-crawler/internal/severity/**
priority: P2
tags: []
---

# WP13 — Severity Inference

## Objective

Configurable heuristic mapping table that infers `severity` from observed substances in a `HarmReductionHazard`. Highest-severity substance wins. Operator-tunable in `config.yml` via `SeverityConfig.Table`.

## Context

- Spec §3 FR-002 (severity field), Edge cases
- Plan §TC-012 (configurable heuristic mapping table)
- Research Q-4 (severity inference authority)

## Branch Strategy

Standard.

## Subtasks

### T053 — Create `internal/severity/table.go`

**Purpose**: Type-safe representation of the substance → severity table.

**Steps**:
1. Create `alert-crawler/internal/severity/table.go`:
   ```go
   package severity

   import (
       "strings"

       "github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
   )

   // Table maps lowercase substance names to severity floors.
   type Table map[string]domain.Severity

   func NewTable(raw map[string]domain.Severity) Table {
       t := make(Table, len(raw))
       for k, v := range raw {
           t[strings.ToLower(strings.TrimSpace(k))] = v
       }
       return t
   }

   func (t Table) Lookup(substance string) (domain.Severity, bool) {
       v, ok := t[strings.ToLower(strings.TrimSpace(substance))]
       return v, ok
   }
   ```
2. Severity ordering: `info < low < medium < high < critical`. Add an `int` rank to support comparison:
   ```go
   func severityRank(s domain.Severity) int {
       switch s {
       case domain.SeverityInfo: return 0
       case domain.SeverityLow: return 1
       case domain.SeverityMedium: return 2
       case domain.SeverityHigh: return 3
       case domain.SeverityCritical: return 4
       default: return -1
       }
   }
   ```

**Files**:
- `alert-crawler/internal/severity/table.go` (new, ~60 lines).

### T054 — Create `internal/severity/infer.go`

**Purpose**: The `Infer(Hazard, Table) Severity` function.

**Steps**:
1. Create `alert-crawler/internal/severity/infer.go`:
   ```go
   package severity

   import "github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"

   const FloorSeverity = domain.SeverityMedium

   // Infer returns the severity level for a hazard based on the configured
   // substance table. Highest-severity substance wins. If no substance matches,
   // returns FloorSeverity.
   func Infer(hazard domain.Hazard, table Table) domain.Severity {
       result := FloorSeverity
       resultRank := severityRank(result)

       if hazard.HarmReduction == nil {
           return FloorSeverity
       }

       // Iterate over both the substance list and the composition entries.
       collect := []string{}
       collect = append(collect, hazard.HarmReduction.Substances...)
       for _, c := range hazard.HarmReduction.Composition {
           collect = append(collect, c.Name)
       }

       for _, name := range collect {
           if sev, ok := table.Lookup(name); ok {
               if r := severityRank(sev); r > resultRank {
                   result = sev
                   resultRank = r
               }
           }
       }
       return result
   }
   ```

**Files**:
- `alert-crawler/internal/severity/infer.go` (new, ~50 lines).

### T055 — Default config table

**Purpose**: Conservative starting point in `SetDefaults` (already added in WP07/T029, but verify and document here).

**Steps**:
1. Verify `WP07` `defaultSeverityTable()` includes:
   - `carfentanil` → `critical`
   - `nitazenes` → `high`
   - `medetomidine` → `high`
   - `xylazine` → `high`
   - `fentanyl` → `high`
   - `benzodiazepine` → `high`
2. If WP07 didn't include all of these, file a follow-up in this WP's reviewer notes for the WP07 implementer to add them.
3. Document in `alert-crawler/CLAUDE.md` how to extend the table via env or override:
   ```
   # config.yml override:
   severity:
     table:
       my-novel-substance: critical
   ```
   Or via env: `SEVERITY_TABLE='{"my-novel-substance":"critical"}'` (depending on what the env tag binds).

**Files**:
- `alert-crawler/CLAUDE.md` (modify gotcha/notes section).

### T056 — Unit tests

**Purpose**: Verify highest-severity-wins semantics and edge cases.

**Steps**:
1. Create `alert-crawler/internal/severity/infer_test.go`:
   - **TestInfer_HighestWins**: hazard with `[fentanyl (high), carfentanil (critical)]` returns `critical`.
   - **TestInfer_UnknownSubstance**: hazard with `[unknown-substance]` returns `medium` (FloorSeverity).
   - **TestInfer_EmptyHazard**: nil HarmReduction returns `medium`.
   - **TestInfer_CompositionAlsoChecked**: substances list empty but composition has `carfentanil`; returns `critical`.
   - **TestInfer_CaseInsensitive**: `[Fentanyl]` matches a table entry of `fentanyl`.
   - **TestInfer_TrimsWhitespace**: `[" fentanyl "]` matches `fentanyl`.
2. `t.Helper()`. Coverage ≥80%.

**Files**:
- `alert-crawler/internal/severity/infer_test.go` (new, ~140 lines).
- `alert-crawler/internal/severity/table_test.go` (new, ~60 lines).

**Validation**:
- All tests pass.
- Coverage ≥80%.

## Definition of Done

- Table type is configurable via WP07's config.
- Infer function returns highest-severity substance wins.
- Defaults are conservative (errs toward higher severity for opioid+benzo combinations).
- Coverage ≥80%.

## Risks

- **PR-003**: heuristic table may drift from issuing-organization practice. Mitigation: configurable; document review cadence in CLAUDE.md.
- **Floor severity**: choosing `medium` as the default guards against a too-low default for unknown substances. If reviewers prefer `low` or `info`, surface for discussion.

## Reviewer Guidance

- Verify the highest-severity-wins logic.
- Verify case-insensitive lookup and whitespace trimming.
- Verify the floor severity is `medium`.
- Confirm defaults align with current Manitoba Harm Reduction Network published guidance (cross-reference T039's golden RSS fixture).

## Implementation Command

```bash
spec-kitty agent action implement WP13 --agent <name>
```

Depends on WP05, WP06, WP07.

## Activity Log

- 2026-05-06T23:24:00Z – claude:sonnet:implementer:implementer – shell_pid=270050 – Started implementation via action command
