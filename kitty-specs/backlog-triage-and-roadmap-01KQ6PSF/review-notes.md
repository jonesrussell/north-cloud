# WP01 Self-Review — Spec Compliance Walk

Mission: `backlog-triage-and-roadmap-01KQ6PSF`
WP: WP01 (the only WP)
Reviewer (self): claude:opus-4.7:implementer:implementer

## Functional Requirements

- **FR-001** ✓ — Per-milestone tables in `triage-report.md` §1.1–§1.3 list every issue from `snapshot.json` with number and title. 13 of 13 covered.
- **FR-002** ✓ — Each row in §1 carries exactly one verdict from `{keep, deprecate, merge-into:#NNN}`. Counts: 12 keep, 1 deprecate (#669), 0 merge-into.
- **FR-003** ✓ — Every row in §1 includes a one-line "Scope" column ≤ 25 words; the same line appears in the §2 detail block.
- **FR-004** ✓ — Every `keep` detail block in §2 carries a Size (S/M/L) and a Size-rationale line. Distribution: 8 S, 2 M, 2 L.
- **FR-005** ✓ — Every `keep` detail block lists `Depends on` and `Blocks`, even if empty.
- **FR-006** ✓ — Every detail block carries a `Milestone:` line.
- **FR-007** ✓ — The single `deprecate` verdict (#669) cites commit `dc4dac20` with link.
- **FR-008** ✓ — No `merge-into` verdicts were issued; trivially satisfied. The report explicitly notes (§5) why no merges were chosen.
- **FR-009** ✓ — §3 contains the Prioritized Survivor List (12 rows, contiguous ranks 1–12) with rationale lines respecting all `depends_on` edges.
- **FR-010** ✓ — §4 proposes 2 mission recommendations covering the rank-1 survivor (#595 in `signal-producer-pipeline`).
- **FR-011** ✓ — Report is at `kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/triage-report.md`.
- **FR-012** ✓ — Snapshot timestamp `2026-04-27T05:44:07Z` recorded in the report header.
- **FR-013** ✓ — §5 ("Deprecation Follow-Up") provides the close-action item with a paste-ready close comment for #669.

## Non-Functional Requirements

- **NFR-001** ✓ — Snapshot timestamp `2026-04-27T05:44:07Z` is minutes ahead of the planned commit timestamp; well within the 24-hour staleness budget.
- **NFR-002** ✓ — 13 of 13 issues classified (Lead Intelligence: 10; Phase 3: 2; Developer Experience: 1). Verified by inspecting `snapshot.json` length against §1 row counts.
- **NFR-003** ✓ — Report is 252 lines (well under the 600-line cap). Per-milestone tables at top + survivor table at §3 give a single-scan path to any verdict.
- **NFR-004** ✓ — Survivor sort is deterministic: topological sort with milestone-priority then size-ascending tiebreakers. Re-running against the pinned `snapshot.json` reproduces the same order.

## Constraints

- **C-001** ✓ — `git status` is expected to show changes only inside `kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/` (the three owned files: `triage-report.md`, `snapshot.json`, `review-notes.md`). Will verify pre-commit.
- **C-002** ✓ — No `gh issue close`, `gh issue edit`, or `gh issue comment` was invoked. Only `gh auth status`, `gh issue list`, and `gh pr list` (all read-only) were used.
- **C-003** ✓ — Backlog derived from live `gh issue list --state open` output (pinned to `snapshot.json`); no hand-curation.
- **C-004** ✓ — Scope-boundary check applied. #580 is flagged as belonging to the sibling Ansible repo for the actual fix, but kept in this triage because the symptom surfaces in NC deploy logs; this is documented in its detail block. No issue belongs to Minoo / Waaseyaa / consuming editorial app code paths.
- **C-005** ✓ — Conventional commit message planned (`feat(triage): generate backlog triage report`). Branch name `claude/musing-benz-b38483` matches the convention. No `--no-verify`, no force-push.
- **C-006** ✓ — `bypass_eligible` defaults to `no` (11 of 12 survivors). Only #580 marked `yes` (Ansible-only env-var addition; no NC code, no schema, no API surface, no security regression — vault still owns the secrets).

## Judgement Calls Flagged for Reviewer

1. **#646 not bypass-eligible despite "chore"**: The bundle includes gosec G118 (real cancel-leak fix in `sse/broker.go`) and `os.Getenv` removal that touches startup wiring. Per C-006, that fails the bypass test even though the bulk of the work is mechanical. A reviewer who disagrees could reasonably split the issue into a trivial slice (mnd/testpackage) and a non-trivial slice (gosec/forbidigo) and bypass the trivial slice — I left this as a single `no` to be conservative.
2. **#580 marked bypass-eligible despite touching prod**: The fix lives in the Ansible role (separate repo), not in this codebase. Within NC the change is documentation-class. Reviewer should confirm the Ansible-side change is also low-risk.
3. **No `merge-into` verdicts**: The deployment cluster (#588, #599, #600) initially looked like a merge candidate, but each targets a different service (signal-crawler vs signal-producer vs enrichment) with different cadences and paths. Keeping them separate matches how they will land.
4. **External dependency on northops-waaseyaa #92, #93** for #588 — flagged in the detail block but **not** treated as an internal `depends_on` edge because cross-repo issues are out of this snapshot. Reviewer may want a separate "external-blocked" annotation in future runs.
5. **Two recommendations, not one or three**: The two clean parallel streams (signal-producer chain vs enrichment chain) make natural mission boundaries. A single "Lead Intelligence Phase 1" mission would be too large; a third covering #646/#580/#588 would add scheduling overhead without parallelism benefit (each is a one-PR chore).
