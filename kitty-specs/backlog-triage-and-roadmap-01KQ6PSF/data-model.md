# Data Model: Backlog Triage and Roadmap

**Mission**: `backlog-triage-and-roadmap-01KQ6PSF`

There is no persisted schema, no database, no API. The "data model" describes
the record shapes the agent produces in memory and renders into markdown rows
inside the triage report. It exists so the report stays uniform across all
thirteen issues and so a future reader can scan it predictably.

## TriageRecord

One per open issue. Renders as a row in the per-milestone tables and as a
detail block below.

| Field             | Type                                              | Required | Source / Notes                                                                                                  |
| ----------------- | ------------------------------------------------- | -------- | --------------------------------------------------------------------------------------------------------------- |
| `number`          | integer                                           | yes      | From `gh issue list` JSON.                                                                                      |
| `title`           | string                                            | yes      | From `gh issue list` JSON; not editorialized.                                                                   |
| `milestone`       | enum {`lead-intelligence`, `phase-3`, `dx`, `other`} | yes      | Mapped from `milestone.title`. `other` triggers an addendum entry rather than full triage.                      |
| `verdict`         | enum {`keep`, `deprecate`, `merge-into:#NNN`}     | yes      | Per FR-002.                                                                                                     |
| `scope_line`      | string ≤ 25 words                                 | yes      | What the issue actually delivers. Per FR-003.                                                                   |
| `justification`   | string                                            | conditional | Required when `verdict ∈ {deprecate, merge-into}`. Per FR-007 / FR-008.                                       |
| `size`            | enum {`S`, `M`, `L`}                              | conditional | Required when `verdict = keep`. Per FR-004.                                                                   |
| `size_rationale`  | string ≤ 25 words                                 | conditional | Required when `verdict = keep`.                                                                                 |
| `depends_on`      | list of `#NNN`                                    | conditional | Required when `verdict = keep`; may be empty list. Per FR-005.                                                  |
| `blocks`          | list of `#NNN`                                    | conditional | Required when `verdict = keep`; may be empty list. Per FR-005.                                                  |
| `bypass_eligible` | enum {`yes`, `no`}                                | conditional | Required when `verdict = keep`. Conservative default `no` per research D4 / spec C-006.                         |

### Validation rules

1. Exactly one verdict per issue.
2. `merge-into:#NNN` must reference an issue number that is also in the snapshot and has `verdict = keep`. Two-way merges (A→B, B→A) are forbidden.
3. `depends_on` must reference issue numbers in the snapshot. Forward references to closed issues are an error (the dependency is already satisfied; remove it).
4. `size` rationale must mention what makes the size that value (e.g., "single package, no migration" for S; "schema change + cross-service callsite" for L).
5. `bypass_eligible: yes` requires that the issue has no schema change, no API surface change, no cross-service touchpoint, and no security-relevant code path. Otherwise `no`.

## SurvivorRank

Derived from `keep` records. Renders as the prioritized survivor list table.

| Field             | Type                | Required | Source / Notes                                                                |
| ----------------- | ------------------- | -------- | ----------------------------------------------------------------------------- |
| `rank`            | integer ≥ 1         | yes      | 1-indexed, contiguous, no gaps.                                               |
| `issue_number`    | integer             | yes      | Foreign key to `TriageRecord.number` where `verdict = keep`.                  |
| `rationale_line`  | string ≤ 25 words   | yes      | One-line "why this position": dependency-driven, milestone priority, or size. |

### Validation rules

1. Every `keep` record appears exactly once in the survivor list.
2. The order respects the partial order induced by `depends_on` edges (a record with `depends_on: [#X]` ranks after `#X`).
3. The first survivor in the list has an empty `depends_on` (otherwise the topological sort is broken).

## MissionRecommendation

Optional. Renders as the **Recommended Next Missions** section. At least one
required (FR-010); typically one to three.

| Field               | Type              | Required | Source / Notes                                                                                              |
| ------------------- | ----------------- | -------- | ----------------------------------------------------------------------------------------------------------- |
| `friendly_name`     | string            | yes      | Title-case, ≤ 7 words; matches the convention used for this mission.                                        |
| `slug_proposal`     | kebab-case string | yes      | Will be passed to `spec-kitty agent mission create "<slug>"`.                                               |
| `survivor_numbers`  | list of integers  | yes      | One or more issue numbers from the survivor list, in the order they should be implemented inside the mission. |
| `scope_paragraph`   | string ≤ 100 words | yes      | What the mission delivers, why it is grouped together, what is explicitly out of scope.                     |

### Validation rules

1. Each `survivor_numbers` entry must reference a real `keep` record.
2. No survivor appears in more than one `MissionRecommendation`.
3. At least one recommendation must cover `rank = 1` from the survivor list, so there is always a clear "next thing to do."
4. Recommendation must not overlap an active branch or open PR (research D5).

## Markdown Rendering

The records map to the report sections as follows:

- **Per-milestone triage tables** (one per milestone, plus an `other` addendum if needed): one row per `TriageRecord`.
- **Detail blocks**: one block per `TriageRecord` with all conditional fields populated, allowing readers to drill in without scrolling between sections.
- **Prioritized Survivor List**: one row per `SurvivorRank`.
- **Recommended Next Missions**: one block per `MissionRecommendation`.
- **Snapshot metadata**: snapshot timestamp + `snapshot.json` reference at the top of the report.
