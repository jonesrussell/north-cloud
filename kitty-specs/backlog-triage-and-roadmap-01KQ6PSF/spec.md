# Specification: Backlog Triage and Roadmap

**Mission**: backlog-triage-and-roadmap-01KQ6PSF
**Mission Type**: software-dev (planning sub-mode — no production code change)
**Status**: Draft
**Created**: 2026-04-27
**Target Branch**: main

---

## Overview

The North Cloud repository is held in a frozen-except-backlog posture: no new feature work begins until the open issue backlog has been deliberately reviewed, pruned, and ordered. Today there are thirteen open issues spread across three milestones (`Lead Intelligence Integration`, `Phase 3: Signal Crawlers`, `Developer Experience`). Some are likely still load-bearing; others are stale, duplicate, or no longer in scope after recent merges (validator promotion, ICP seeding, signal-crawler scaffolding).

This mission produces a single deliverable: a **triage report** that classifies each open issue, identifies dependencies, sizes the survivors, and proposes the next set of Spec Kitty missions to run. It does not modify production code, infrastructure, or tests. Its output is the input for every subsequent mission in this charter.

## User Scenarios

### Primary

The solo maintainer reads the triage report, accepts or revises the verdicts, and uses the prioritized survivor list to decide which Spec Kitty mission to run next. The maintainer can answer “what is the smallest valuable next mission?” without re-reading thirteen GitHub issues from scratch.

### Secondary

A future agent (acting as reviewer or planner) uses the same report as authoritative context for sequencing decisions, dependency awareness, and de-duplication, instead of re-deriving the backlog state from GitHub each session.

### Edge Cases

- An open issue is already obsoleted by a recent merge but the GitHub issue is still open. The triage marks it `deprecate` with a one-line justification linking to the merge.
- Two issues describe overlapping work (for example, deployment + scheduling for the same service). The triage marks one `merge-into:#NNN` and folds its scope into the survivor.
- An issue’s scope is unclear from the title alone. The triage reads the issue body and any linked PR/issue thread before classifying, and records a one-line scope so a future reader does not need to.
- New issues are filed mid-mission. The report is treated as a snapshot at the time of generation; a follow-up mission may rerun the triage if the backlog changes materially.

## Functional Requirements

| ID      | Requirement                                                                                                                                                                                                                                        | Status   |
| ------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| FR-001  | The triage report MUST list every issue currently open at the time of generation across the milestones `Lead Intelligence Integration (north-cloud)`, `Phase 3: Signal Crawlers`, and `Developer Experience`, identified by issue number and title. | Required |
| FR-002  | For each listed issue, the report MUST record exactly one verdict from the set {`keep`, `deprecate`, `merge-into:#NNN`}.                                                                                                                            | Required |
| FR-003  | For each issue, the report MUST record a one-line scope statement (≤ 25 words) describing what the issue actually delivers, not just its title.                                                                                                     | Required |
| FR-004  | For each `keep` issue, the report MUST record a rough size estimate from the set {S, M, L} where S = ≤ 0.5 day, M = 0.5–2 days, L = > 2 days, and a one-line justification of the size class.                                                       | Required |
| FR-005  | For each `keep` issue, the report MUST list dependencies on other issues by number, distinguishing `blocks:` (this issue blocks the listed issue) from `depends-on:` (this issue is blocked by the listed issue).                                  | Required |
| FR-006  | For each `keep` issue, the report MUST record the milestone bucket it belongs to.                                                                                                                                                                  | Required |
| FR-007  | For each `deprecate` verdict, the report MUST record a one-line justification (typically a link to the merge, PR, or rationale that obsoleted the issue).                                                                                          | Required |
| FR-008  | For each `merge-into:#NNN` verdict, the report MUST identify the absorbing issue and record a one-line note describing what scope is being folded in.                                                                                              | Required |
| FR-009  | The report MUST contain a **Prioritized Survivor List** ordering all `keep` issues by execution sequence, respecting recorded dependencies, with a one-line rationale for the chosen ordering.                                                     | Required |
| FR-010  | The report MUST include a **Recommended Next Missions** section proposing one or more Spec Kitty missions covering the top of the survivor list, with a friendly name and a one-paragraph scope per proposed mission.                              | Required |
| FR-011  | The report MUST be a single markdown file committed to the repository at `kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/triage-report.md` on the mission branch.                                                                                  | Required |
| FR-012  | The report MUST record the GitHub data snapshot timestamp (UTC, ISO 8601) used to derive the issue list, so readers can detect drift if the backlog changes after the report is written.                                                            | Required |
| FR-013  | If a `deprecate` verdict is proposed, the report MUST include a follow-up action item to actually close the GitHub issue (separate from this mission) so the live backlog converges with the triage outcome.                                       | Required |

## Non-Functional Requirements

| ID       | Requirement                                                                                                                                                                                                            | Threshold                                                                              | Status   |
| -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- | -------- |
| NFR-001  | Report freshness — the GitHub snapshot used to populate the report must be no older than the report’s commit timestamp.                                                                                                | Snapshot timestamp ≤ commit timestamp − 24 hours.                                      | Required |
| NFR-002  | Coverage — the report must classify 100% of issues open at snapshot time across the three named milestones.                                                                                                            | 13 of 13 (or N of N if backlog changes by snapshot time).                              | Required |
| NFR-003  | Readability — a maintainer unfamiliar with the report layout can locate any issue’s verdict in under thirty seconds via a single scan of the report.                                                                  | Verdict locatable in ≤ 30 seconds; report ≤ 600 lines total.                           | Required |
| NFR-004  | Survivor list determinism — two runs of the same triage logic against the same snapshot produce the same survivor ordering.                                                                                            | Identical ordering on repeated runs against pinned snapshot data.                     | Required |

## Constraints

| ID    | Constraint                                                                                                                                                                                  | Status   |
| ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| C-001 | This mission MUST NOT modify any source code, configuration, infrastructure, or test under the production service trees (`crawler/`, `classifier/`, `publisher/`, etc.).                     | Required |
| C-002 | This mission MUST NOT close, edit, or comment on GitHub issues. Issue closure is a downstream action carried out separately, post-merge.                                                    | Required |
| C-003 | The report MUST be derived from the live GitHub state at snapshot time (via `gh issue list`), not from a hand-curated guess at the backlog.                                                | Required |
| C-004 | The mission MUST honor the charter’s scope boundary: NC owns the content pipeline only. Issues that turn out to belong to Minoo, Waaseyaa, or consuming editorial apps are flagged `deprecate` with a scope-violation note. | Required |
| C-005 | All commits MUST follow conventional commit format and the `claude/{description}-{session-id}` branch convention; no force-pushes to `main`; no `--no-verify`.                              | Required |
| C-006 | The triage MUST respect the charter’s “trivial fixes may bypass the full Spec Kitty loop” clause: each `keep` issue is annotated as `bypass-eligible: yes/no`, where `yes` means the maintainer may implement directly under conventional commit + CI gates without spawning a Spec Kitty mission. | Required |

## Success Criteria

1. The maintainer can decide the next mission to run in under five minutes after opening the triage report, without re-reading the thirteen issues.
2. Every open issue at snapshot time has exactly one verdict and (if kept) a survivor-list rank.
3. The Recommended Next Missions section names at least one mission whose scope is small enough to ship in one Spec Kitty cycle (specify → plan → tasks → implement → review).
4. After the maintainer accepts the report, deprecating the obsolete issues on GitHub takes less than ten minutes (i.e., each `deprecate` row carries enough context to close the issue without re-investigation).
5. Re-running the triage against the same snapshot produces the same survivor list.

## Key Entities

- **Issue**: A GitHub issue identified by `(repo, number)` with a title, body, milestone, and label set, captured at snapshot time.
- **Milestone**: One of the three named buckets (`Lead Intelligence Integration (north-cloud)`, `Phase 3: Signal Crawlers`, `Developer Experience`).
- **Verdict**: One of `keep`, `deprecate`, `merge-into:#NNN`, applied per issue.
- **Survivor**: An issue with verdict `keep`, augmented with size, dependencies, milestone, bypass-eligibility, and survivor-list rank.
- **Mission Recommendation**: A proposed downstream Spec Kitty mission grouping one or more survivors under a shared scope.
- **Triage Report**: The single markdown deliverable that contains all of the above.

## Assumptions

- The `gh` CLI is available and authenticated for the maintainer’s account; the agent uses it to capture the snapshot.
- The three milestone names listed by the user (`Lead Intelligence Integration (north-cloud)`, `Phase 3: Signal Crawlers`, `Developer Experience`) are the only milestones in scope. Any open issue without one of these milestones is out of scope for this triage and noted as such in an addendum.
- The current count of thirteen open issues is correct at the time of mission creation. The actual snapshot at execution may differ slightly; the report uses the snapshot count as authoritative.
- The maintainer accepts that some `keep` issues may turn out to be redundant once a downstream mission spec is drafted; reclassification by a later mission is acceptable.
- “Trivial bypass” is a maintainer judgment call. The triage proposes which issues qualify; the maintainer ratifies.

## Out of Scope

- Closing or commenting on GitHub issues.
- Drafting full specs for the recommended downstream missions (each gets its own `/spec-kitty.specify`).
- Re-prioritizing closed milestones or issues that are not currently open.
- Changes to the charter, repo layout, or CI configuration.
- Any production code or infrastructure modification.

## Risks and Mitigations

| Risk                                                                            | Mitigation                                                                                                                                |
| ------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| Triage classifies a still-relevant issue as `deprecate` due to a stale read.    | Each `deprecate` row carries a justification link; maintainer review acts as the final gate before any GitHub close action.               |
| Survivor list bakes in a sequence the maintainer disagrees with.                | The list is advisory; reordering is a maintainer judgment call and does not require re-running the mission.                              |
| Backlog changes between snapshot and merge.                                    | Snapshot timestamp is recorded in the report (FR-012); a future micro-triage can amend the survivor list if drift is material.            |
| Recommendation proposes a mission that overlaps an in-flight or merged change. | The agent cross-checks recommended scope against recent commit history (last 30 days of `git log`) before naming a recommendation.       |

---

## Notes for Downstream Phases

- **/spec-kitty.plan** for this mission produces a concrete plan for *generating the triage report* (data collection steps, classification heuristics, write/commit). It does not plan the survivors’ implementation.
- **/spec-kitty.tasks** breaks the planning steps into work packages: snapshot, per-milestone triage pass, survivor sequencing, recommendation drafting, report write/commit.
- **/spec-kitty.implement** executes the triage and writes `triage-report.md`. No production-code worktree should be needed; the report lives under `kitty-specs/`.
- **/spec-kitty.review** validates the report against this spec’s FRs, NFRs, and constraints before merge.
