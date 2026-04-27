# Research: Backlog Triage and Roadmap

**Mission**: `backlog-triage-and-roadmap-01KQ6PSF`
**Date**: 2026-04-27

## Open Clarifications

None. The spec contains zero `[NEEDS CLARIFICATION]` markers. The only open
decisions are methodology choices for executing the triage; those are
documented below.

## Methodology Decisions

### D1 — Snapshot the GitHub state once at the start of execution

**Decision**: Run `gh issue list --state open --json number,title,milestone,labels,body --limit 100` once at the start of execution. Persist the raw JSON to `snapshot.json` next to the spec. All subsequent classification reads only from that file.

**Rationale**: GitHub state can drift mid-run if other automation files or closes issues. Snapshotting once produces a deterministic, re-auditable input. Also satisfies FR-012 (record the snapshot timestamp) and NFR-001 (freshness), and supports NFR-004 (deterministic survivor ordering).

**Alternatives considered**:
- Re-fetch per issue: rejected — non-deterministic, harder to debug if a verdict is challenged.
- Cache in memory only: rejected — loses traceability the moment the run ends.

### D2 — Cross-check each open issue against the last 30 days of `git log`

**Decision**: For every issue under consideration, scan the last 30 days of `git log --oneline --no-merges` for commits whose message references the issue number, related identifiers, or the topic. If a recent merge plausibly obsoletes the issue, mark `verdict = deprecate` and link the commit hash in the justification.

**Rationale**: The repo has had recent merges (validator sector_alignment promotion, ICP seeding fixes, signal-crawler scaffolding) that almost certainly obsolete some open issues. 30 days is wide enough to cover those merges without dragging in unrelated history.

**Alternatives considered**:
- 90 days: rejected — pulls in history irrelevant to the current backlog state, increases noise.
- Since-issue-creation: rejected — per-issue cost is high, and many issues are months old; mostly the same answer as 30 days for what we care about.
- Skip the cross-check: rejected — we'd `keep` issues that are already done.

### D3 — Topological sort with milestone-priority tiebreaker for survivor ordering

**Decision**: Order the survivor list by topological sort over `depends_on` edges. Break ties (independent items at the same level) using milestone priority `Lead Intelligence Integration > Phase 3: Signal Crawlers > Developer Experience`, then by smaller size first within a milestone.

**Rationale**: Dependencies are the hard constraint — violating them produces work the maintainer can't actually start. Within an unblocked tier, the user has signaled Lead Intelligence is the headline thread, so it gets first call. Smaller-first inside a tier surfaces fast wins for momentum. Satisfies FR-009 and NFR-004.

**Alternatives considered**:
- Pure milestone-priority order: rejected — ignores dependencies.
- Pure size-first: rejected — starves the high-value Lead Intelligence work in favor of trivial Developer Experience items.
- Value-weighted scoring: rejected — adds subjectivity and a parameter the maintainer would have to maintain; not worth the complexity for thirteen issues.

### D4 — `bypass-eligible` is advisory and conservative

**Decision**: Annotate each `keep` survivor with `bypass_eligible: yes/no`. Default to `no`. Mark `yes` only when the survivor is a single-file or single-package change with no schema, no API surface, no cross-service touchpoint, and no security-relevant code path.

**Rationale**: The charter (per the testing-tier and review-policy clauses) explicitly allows trivial fixes to bypass the full Spec Kitty loop. But the bypass is the maintainer's call, not the agent's. Conservative defaulting keeps the safety property (full loop unless proven trivial) and surfaces the agent's recommendation without locking it in.

**Alternatives considered**:
- Default to `yes`: rejected — inverts the safety bias.
- Skip the annotation: rejected — forces the maintainer to re-derive triviality on every survivor.

### D5 — Cross-check recommendations against active branches and open PRs

**Decision**: Before naming a recommended downstream mission, the agent runs `git branch -a --list 'claude/*'` and `gh pr list --state open` and refuses to recommend scope that overlaps an active branch or open PR.

**Rationale**: Avoids the failure mode where the triage proposes a mission whose work is already in progress on another branch. Cheap check, high value.

**Alternatives considered**:
- Trust the maintainer to catch overlap: rejected — the whole point of the report is to remove that re-derivation cost.

## References Consulted

- `docs/specs/lead-pipeline.md` — to understand which Lead Intelligence Integration issues are still architecturally load-bearing.
- `docs/superpowers/specs/2026-04-05-signal-crawler-sprint-design.md` — to assess the two Phase 3 issues against the signal-crawler design intent.
- Recent commit history (`ca572c5e`, `1784a144`, `e5d8389c`, `8eedc729`, `dc4dac20`) — candidate sources of obsolescence.
- Charter `testing_requirements`, `review_policy`, `risk_boundaries`, `exception_policy` — drives the bypass-eligibility heuristic and the no-code-change boundary.
