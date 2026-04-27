# Implementation Plan: Backlog Triage and Roadmap

**Branch**: `main` (planning base) | **Date**: 2026-04-27 | **Spec**: [spec.md](spec.md)
**Mission**: `backlog-triage-and-roadmap-01KQ6PSF` (mid8: `01KQ6PSF`)

## Summary

Generate a single markdown triage report classifying every open issue across three named milestones, sequence the survivors by dependency, and recommend the next Spec Kitty missions to run. The implementation is a four-step pipeline executed by an agent: GitHub snapshot → per-issue read + classify → survivor sequencing → report write + commit. There is no production code change, no API surface, and no schema migration. The only artifact written outside the mission folder is the report itself, which lives under `kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/triage-report.md`.

## Technical Context

| Aspect              | Value                                                                                                                                                                                                                              |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Deliverable type    | A single markdown file (`triage-report.md`) committed to the mission branch.                                                                                                                                                       |
| Data source         | Live GitHub state via `gh issue list --state open --json number,title,milestone,labels,body --limit 100`, snapshotted once at the start of execution and pinned for determinism.                                                   |
| Snapshot artifact   | Raw `gh` JSON saved to `kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/snapshot.json` at the start of execution; the report references its timestamp (FR-012, NFR-001).                                                            |
| Issue body access   | `gh issue view <number> --json number,title,body,milestone,labels,closedByPullRequestsReferences` for any issue whose title alone is insufficient to classify (most of them).                                                       |
| Classification logic | Per-issue heuristic: (a) cross-check title/body against last 30 days of `git log`; (b) if obsoleted, mark `deprecate`; (c) if scope overlaps another open issue, mark `merge-into:#NNN`; (d) otherwise `keep` with size + deps.    |
| Sequencing logic    | Topological sort over `depends-on:` edges among `keep` issues; ties broken by milestone priority (Lead Intelligence > Phase 3 > Developer Experience as the user-stated focus) and by smaller size first.                          |
| Charter constraints | No production code change (C-001), no GH writes (C-002), conventional commits, no `--no-verify`, no force-push to `main`, no cross-service imports relevant (we only touch `kitty-specs/`).                                       |
| Quality gates       | golangci-lint / `task test` / `task drift:check` / layers / ports — all N/A because no code under service trees changes. Pre-commit lefthook hooks still run; they should be no-ops for markdown-only commits.                     |
| Test approach       | Validation is checklist-based: a self-review pass against the spec's FRs/NFRs/Cs before committing the report. No unit tests; no integration tests (per spec — planning-only).                                                     |

## Charter Check

| Charter dimension     | Status                                                                                                                                                                                          |
| --------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Languages/Frameworks  | ✅ N/A — markdown only.                                                                                                                                                                         |
| Testing requirements  | ✅ N/A — no code; spec acceptance is a self-review checklist.                                                                                                                                   |
| Quality gates         | ✅ All Go gates trivially pass (no Go change). Lefthook still executes; no expected violations.                                                                                                  |
| Review policy         | ✅ Solo maintainer + agent reviewer model applies to the report itself: agent self-reviews against spec before merge; maintainer arbitrates on rejection.                                       |
| Performance targets   | ✅ N/A.                                                                                                                                                                                         |
| Deployment            | ✅ N/A — nothing deploys.                                                                                                                                                                       |
| Risk boundaries       | ✅ Mission stays inside `kitty-specs/`. No service code touched. NC scope boundary (Minoo / Waaseyaa / consuming apps) is *enforced by* the triage itself (C-004 of the spec).                  |
| Bypass-eligibility    | ⚠ This mission is itself non-trivial (it sequences all downstream work) and therefore correctly runs the full Spec Kitty loop. The triage will identify which *downstream* survivors qualify for bypass. |

No charter conflicts. No directives violated. Charter Check **passes**.

## Phase 0 — Outline & Research

The spec contains zero `[NEEDS CLARIFICATION]` markers. There are no open architectural unknowns. Research output (`research.md`) captures methodology decisions only:

- **Decision: snapshot once, pin to disk.** Reading GitHub state at multiple points during the run would let the backlog drift mid-classification. Snapshotting once and writing the raw JSON to `snapshot.json` makes the run deterministic and re-auditable. Alternatives considered: (a) re-fetching per issue (rejected: non-deterministic), (b) caching in memory only (rejected: loses traceability after the run).
- **Decision: cross-check against last 30 days of `git log` for the `deprecate` heuristic.** Many of the open issues predate recent merges (validator promotion, ICP fixes, signal-crawler scaffolding); 30 days covers all of those. Alternatives: 90 days (rejected: noise from older work that isn't relevant to current backlog state); since-issue-creation (rejected: per-issue cost, harder to audit).
- **Decision: topological sort with milestone-priority tiebreaker.** Dependencies are the hard constraint. Within an unblocked tier, prefer smaller items first to surface fast wins. Alternatives: pure milestone-priority order (rejected: ignores dependencies), pure size-first (rejected: starves the high-value Lead Intelligence work).
- **Decision: `bypass-eligible` annotation is advisory only.** The agent proposes; the maintainer ratifies. Encoding it in the report (per C-006) avoids re-deriving it later.

Output: `research.md`.

## Phase 1 — Design & Contracts

### Data model

Three logical record types live inside the report. There is no persistent schema and no DB; the records are markdown rows. Captured in `data-model.md`:

- **TriageRecord** — one per open issue. Fields: `number`, `title`, `milestone`, `verdict ∈ {keep, deprecate, merge-into:#NNN}`, `scope_line`, `justification` (for deprecate/merge), and (when `verdict = keep`) `size ∈ {S, M, L}`, `size_rationale`, `depends_on: [#NNN, ...]`, `blocks: [#NNN, ...]`, `bypass_eligible ∈ {yes, no}`.
- **SurvivorRank** — derived from `keep` records: `rank` (1-indexed), `issue_number`, `rationale_line`.
- **MissionRecommendation** — proposed downstream missions: `friendly_name`, `slug_proposal`, `survivor_numbers: [#NNN, ...]`, `scope_paragraph`.

### Contracts

No API surface. No `contracts/` directory. The report's *content contract* is the spec's FR table; nothing to add here.

### Quickstart

A short operator-facing guide is captured in `quickstart.md`: how to read the report, how to act on `deprecate` rows, how to convert a `MissionRecommendation` into a `/spec-kitty.specify` invocation.

## Execution Approach

Implementation runs as a single sequential pass. There is no parallelism, no lanes, and no branching beyond the planning branch itself. The work packages produced by `/spec-kitty.tasks` will reflect this:

1. **Snapshot** — capture `gh issue list` JSON to `snapshot.json` and record the UTC timestamp.
2. **Per-issue read** — for each of the 13 issues, fetch the issue body and any linked-PR signals, distill into a one-line scope.
3. **Classify** — apply the verdict heuristic; record justifications.
4. **Size and depend** — for each `keep`, attach S/M/L plus dependency edges.
5. **Sequence** — topological sort + tiebreakers → ordered survivor list.
6. **Recommend** — group the top of the survivor list into 1–3 proposed downstream missions.
7. **Write and review** — write `triage-report.md`, self-review against spec FRs/NFRs/Cs, then commit.
8. **Report back** — produce a summary message with verdict counts, top survivors, and recommended next mission.

## Risks (Premortem)

| Risk                                                                                                                          | Likelihood | Impact | Mitigation                                                                                                                                  |
| ----------------------------------------------------------------------------------------------------------------------------- | ---------- | ------ | -------------------------------------------------------------------------------------------------------------------------------------------- |
| Issue body lacks enough info to classify confidently.                                                                         | Medium     | Med    | Mark with verdict `keep` + `bypass_eligible: no` and a one-line note "needs maintainer scope review"; do **not** silently `deprecate`.       |
| `gh` not authenticated in execution environment.                                                                              | Low        | High   | First WP runs `gh auth status`; fails fast with a clear error if not authenticated.                                                          |
| Recent merge obsoletes an issue but the issue body contradicts that.                                                          | Medium     | Med    | The 30-day `git log` cross-check is mandatory; agent records both signals in the justification line and lets the maintainer arbitrate.       |
| Survivor sequencing produces a result the maintainer disagrees with.                                                          | Medium     | Low    | The list is advisory (per spec). Maintainer reordering is cheap and does not invalidate the report.                                          |
| The triage takes long enough that GitHub state drifts.                                                                        | Low        | Low    | Snapshot pinning (research decision #1) prevents this from corrupting the run.                                                               |
| The agent recommends a downstream mission that overlaps an in-flight branch.                                                  | Low        | Med    | Recommendation step explicitly checks `git branch -a` for active `claude/*` branches and `gh pr list --state open` before naming missions.   |

## Stop Point

Phase 1 complete. Artifacts: `plan.md`, `research.md`, `data-model.md`, `quickstart.md`. No `contracts/`. No bulk-edit map (this is not a bulk edit).

Next command: `/spec-kitty.tasks`. Branch contract for that command: planning base = `main`, merge target = `main`.
