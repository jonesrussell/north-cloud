---
work_package_id: WP01
title: Generate Triage Report
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-005
- FR-006
- FR-007
- FR-008
- FR-009
- FR-010
- FR-011
- FR-012
- FR-013
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T001
- T002
- T003
- T004
- T005
- T006
- T007
agent: "claude:opus-4.7:implementer:implementer"
shell_pid: "31084"
history:
- event: created
  at: '2026-04-27T05:38:52Z'
  by: /spec-kitty.tasks
authoritative_surface: kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/
execution_mode: planning_artifact
mission_id: 01KQ6PSF33WBN4QX3V0STTC8VT
mission_slug: backlog-triage-and-roadmap-01KQ6PSF
owned_files:
- kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/triage-report.md
- kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/snapshot.json
- kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/review-notes.md
tags: []
---

# WP01 — Generate Triage Report

## Objective

Produce `kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/triage-report.md` that classifies every open issue across the three named milestones, sequences the survivors by dependency, and recommends the next Spec Kitty missions to run. Also produce the deterministic GitHub snapshot (`snapshot.json`) and a private review note (`review-notes.md`) capturing the FR/NFR/C compliance walk.

This work package is the entire mission. There is no second WP. Splitting was rejected at planning time because the work is strictly sequential and produces a single output artifact; multiple WPs would add worktree overhead with no parallelism benefit.

## Context

Read these before starting:

- [spec.md](../spec.md) — authoritative requirement contract (FR-001 to FR-013, NFR-001 to NFR-004, C-001 to C-006).
- [plan.md](../plan.md) — execution approach and Charter Check.
- [research.md](../research.md) — methodology decisions (snapshot pinning, 30-day git cross-check, topo sort with milestone tiebreaker, conservative bypass default, branch/PR overlap check).
- [data-model.md](../data-model.md) — exact shape of `TriageRecord`, `SurvivorRank`, `MissionRecommendation`.
- [quickstart.md](../quickstart.md) — operator-facing reading guide; the report you produce must match the structure described here.

Charter constraints to honor:

- No production code change anywhere outside `kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/` (C-001).
- No GitHub writes — do not close, edit, or comment on any issue (C-002).
- Conventional commit message; no `--no-verify`; no force-push to `main`; honor lefthook hooks (C-005).
- NC scope boundary: any open issue that turns out to belong to Minoo, Waaseyaa, or consuming editorial apps gets `verdict = deprecate` with a scope-violation note (C-004).
- `bypass_eligible` defaults to `no`; only mark `yes` for single-package, no-schema, no-cross-service, no-security-relevant changes (C-006, research D4).

## Branch Strategy

Planning base: `main`. Final merge target: `main`. The execution worktree for this WP is allocated by `lanes.json` after `finalize-tasks` runs; do not create a worktree manually. All commits land on the lane branch and merge into `main` via the standard implement → review → merge flow.

## Subtask Guidance

### T001 — Pre-flight Checks and Snapshot

**Purpose**: Establish a deterministic, re-auditable input for the rest of the WP. Fail fast if the environment is wrong.

**Steps**:

1. Run `gh auth status`. If unauthenticated, stop with a clear error message and exit non-zero. Do not attempt to proceed without `gh`.
2. Capture the recent merge context in memory: `git log --oneline --no-merges --since="30 days ago"`. You will reference this in T002.
3. Capture the active-branch context in memory: `git branch -a --list 'claude/*'`. You will reference this in T005.
4. Capture the open-PR context: `gh pr list --state open --json number,title,headRefName,baseRefName --limit 50`. You will reference this in T005.
5. Pin the GitHub snapshot. Run:

   ```bash
   gh issue list --state open --json number,title,milestone,labels,body,closedByPullRequestsReferences --limit 100 > kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/snapshot.json
   ```

6. Record the snapshot timestamp. Use the UTC ISO-8601 timestamp captured at step 5; you will quote this in the report (FR-012, NFR-001).
7. Sanity-check coverage. The snapshot should contain ~13 open issues across `Lead Intelligence Integration (north-cloud)`, `Phase 3: Signal Crawlers`, `Developer Experience`. If the count is materially different (≤ 5 or ≥ 25), flag in the review notes and proceed; do not silently truncate.

**Files written**: `kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/snapshot.json`.

**Validation**:

- [ ] `gh auth status` exits 0.
- [ ] `snapshot.json` is valid JSON and contains a non-empty list.
- [ ] At least one issue belongs to each of the three named milestones (else flag in review notes).

### T002 — Per-Issue Read and Classify

**Purpose**: Convert each issue into a `TriageRecord` (per `data-model.md`) with a one-line scope and a verdict.

**Steps**:

1. Iterate over every issue in `snapshot.json`. Skip none.
2. For each issue, read the body. If the title alone is unambiguous (rare), the scope line can be derived from the title; otherwise read the body and any `closedByPullRequestsReferences` for additional signal.
3. Decide the verdict using these heuristics in order:
   - **`deprecate`** if any of: (a) a recent commit (last 30 days, captured in T001) plausibly obsoletes the issue; (b) the issue's scope falls outside NC's pipeline boundary into Minoo / Waaseyaa / editorial apps; (c) the issue describes work already merged. Justification line MUST cite the commit hash or PR number that obsoleted it.
   - **`merge-into:#NNN`** if another open issue already covers the same scope (often: deployment + scheduling for the same service, or two issues describing the same enricher). Justify with a one-line note explaining what scope is being folded into the absorbing issue. Validate: the absorbing issue must also be in the snapshot and ultimately get `verdict = keep`.
   - **`keep`** otherwise. Default verdict; conservative.
4. Write the one-line scope statement (≤ 25 words). It must describe what the issue *delivers*, not just restate its title.
5. If the issue body is too thin to classify confidently, default to `keep` with a justification of "needs maintainer scope review" — do not silently `deprecate` low-information issues.

**Files written**: in-memory `TriageRecord` list; nothing flushed yet.

**Validation**:

- [ ] Every issue from the snapshot has exactly one verdict.
- [ ] Every `deprecate` carries a justification line with at least one link (commit hash, PR, or issue).
- [ ] Every `merge-into:#NNN` references an issue number present in the snapshot, and there are no two-way merges (A→B with B→A).

### T003 — Annotate `keep` Records

**Purpose**: Add the size, dependency, and bypass-eligibility fields required by FR-004, FR-005, and C-006 for every survivor.

**Steps**:

1. For each `keep` record, assign `size ∈ {S, M, L}`:
   - **S** = ≤ 0.5 day. Single package, no schema, no migration, no cross-service touchpoint.
   - **M** = 0.5–2 days. Multiple files, possibly within one service; may include simple migration or config addition.
   - **L** = > 2 days. Cross-service, schema/ES mapping change, new ML sidecar, or new deployable.
2. Write a ≤ 25-word `size_rationale` citing the structural reason (e.g., "single package, in-memory only" → S; "ES mapping + reindex + classifier handler" → L).
3. For each `keep`, populate `depends_on` and `blocks`. Read the issue body for explicit references ("blocked by #595", "after #592 lands"). Cross-check: every number in `depends_on` must reference an issue present in the snapshot; closed-issue references are an error and should be removed (the dependency is already satisfied).
4. Set `bypass_eligible`. Default `no`. Mark `yes` only when ALL of: single package, no schema or ES mapping change, no API surface change, no cross-service touchpoint, no security-relevant code path. When in doubt, `no`.

**Files written**: in-memory `TriageRecord` list (now complete).

**Validation**:

- [ ] Every `keep` record has size, size_rationale, depends_on (possibly empty), blocks (possibly empty), and bypass_eligible.
- [ ] No `depends_on` entry references a closed issue.
- [ ] At least one record has `bypass_eligible: yes` if any survivor is genuinely trivial; otherwise the explicit conservative default is fine.

### T004 — Sequence Survivors

**Purpose**: Produce the prioritized `SurvivorRank` list (FR-009).

**Steps**:

1. Build a directed graph: nodes = `keep` records, edges = `depends_on` (A → B if A.depends_on contains B).
2. Topologically sort. If a cycle is detected, treat it as a defect: drop the weakest edge (the one with the least textual evidence in the issue bodies), record the cycle in review notes, and continue.
3. Within each topological tier (independent items), tiebreak by:
   - **Milestone priority**: `Lead Intelligence Integration (north-cloud)` > `Phase 3: Signal Crawlers` > `Developer Experience`.
   - **Then by size ascending**: S before M before L.
4. Assign `rank` 1-indexed and contiguous.
5. For each `SurvivorRank`, write a ≤ 25-word `rationale_line` explaining the position: dependency-driven ("blocks #594, must ship first"), milestone priority ("Lead Intelligence headline thread"), or size ("trivial cleanup, fast win").

**Files written**: in-memory `SurvivorRank` list.

**Validation**:

- [ ] Every `keep` record appears exactly once in the survivor list.
- [ ] The ordering respects all `depends_on` edges (a survivor with `depends_on: [#X]` ranks after `#X`).
- [ ] Rank 1 has empty `depends_on`.
- [ ] Rationale lines are concrete (cite a number, milestone, or size — not vague).

### T005 — Draft Recommended Next Missions

**Purpose**: Translate the top of the survivor list into 1–3 concrete `MissionRecommendation`s ready to feed into `/spec-kitty.specify` (FR-010).

**Steps**:

1. Read the active-branch list from T001 (`claude/*`) and the open-PR list. For each candidate recommendation, verify its scope does not overlap any active branch or open PR. If it does, exclude that survivor from the recommendation (note it as "in flight" in the report).
2. Group survivors that share a single architectural surface or one cohesive deliverable into a single recommendation. For example, "signal producer binary + checkpoint persistence + Waaseyaa client" likely belong together; "deploy signal-crawler with cron schedule" and "ops: signal producer deployment" might belong together as a deployment mission.
3. For each recommendation:
   - `friendly_name` — title-case, ≤ 7 words.
   - `slug_proposal` — kebab-case, no leading numeric prefix.
   - `survivor_numbers` — list of issue numbers in implementation order.
   - `scope_paragraph` — ≤ 100 words: what the mission delivers, why these survivors group together, what is explicitly out of scope.
4. At least one recommendation must cover the rank-1 survivor (FR-010 + data-model validation).
5. Verify no survivor appears in more than one recommendation.

**Files written**: in-memory `MissionRecommendation` list.

**Validation**:

- [ ] At least one recommendation; at most three.
- [ ] Rank-1 survivor is covered.
- [ ] No survivor appears in more than one recommendation.
- [ ] No recommendation overlaps an active `claude/*` branch or open PR.

### T006 — Compose `triage-report.md`

**Purpose**: Render the in-memory records into the final markdown deliverable (FR-001 to FR-013, NFR-003).

**Steps**:

1. Open with snapshot metadata: snapshot timestamp (UTC ISO-8601), pointer to `snapshot.json`, total issue count, milestone counts.
2. Per-milestone triage tables: one table per milestone bucket. Columns: `#`, `Title`, `Verdict`, `Scope (one line)`. Mark scope-out issues separately if any.
3. Detail blocks: one block per issue, `## #NNN — Title`, with all conditional fields populated. Use bullet lists for `depends_on` and `blocks` to keep it scannable.
4. Prioritized Survivor List: a single table covering all `keep` records in rank order. Columns: `Rank`, `#`, `Title`, `Size`, `Bypass`, `Why this position`.
5. Recommended Next Missions: one `### <Friendly Name>` block per recommendation with the slug proposal, the survivor numbers (in execution order), and the scope paragraph.
6. Deprecation follow-up list: at the bottom, an actionable list of issue numbers to close, each with the closing rationale and link the maintainer can paste into the GitHub close comment (FR-013).
7. Keep total length ≤ 600 lines (NFR-003). If overflowing, prefer compressing detail blocks (one paragraph each) over dropping content.

**Files written**: `kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/triage-report.md`.

**Validation**:

- [ ] All snapshot issues appear in a per-milestone table or in a scope-out addendum.
- [ ] Every issue has a detail block.
- [ ] Survivor table is contiguous in rank, dependency-respecting.
- [ ] At least one recommendation; rank-1 covered.
- [ ] Report is ≤ 600 lines.

### T007 — Self-Review and Commit

**Purpose**: Verify spec compliance line by line, then commit (NFR-002, FR coverage; satisfies Spec Kitty review-policy expectation that the agent self-reviews against the spec before the implement-review loop).

**Steps**:

1. Open `spec.md`. For each FR (FR-001 to FR-013): write a one-line "✓ — covered by section X" or "✗ — gap: ..." entry into `review-notes.md`. If any `✗`, return to the relevant subtask and fix; do not proceed until all `✓`.
2. Repeat for every NFR (NFR-001 to NFR-004) with measurable evidence (e.g., NFR-003: "report is 432 lines, within ≤ 600 cap"; NFR-002: "13 of 13 issues classified").
3. Repeat for every constraint (C-001 to C-006). C-001 is satisfied iff `git status` shows changes only inside `kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/`. C-002 is satisfied iff no `gh issue close`, `gh issue edit`, or `gh issue comment` command was run during this WP.
4. Stage the report, snapshot, and review notes:

   ```bash
   git add kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/triage-report.md \
           kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/snapshot.json \
           kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/review-notes.md
   ```

5. Commit with a conventional commit message:

   ```
   feat(triage): generate backlog triage report

   Snapshots N open issues across three milestones. Classifies each
   (keep/deprecate/merge), sequences survivors by dependency, recommends
   M next missions. Planning-only — no production code change.
   ```

6. Do **not** push, force-push, or run `--no-verify`. Lefthook may run; markdown-only changes should pass without effort.

**Files written**: `kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/review-notes.md`.

**Validation**:

- [ ] `review-notes.md` shows ✓ for every FR, NFR, and C in `spec.md`.
- [ ] `git status` shows changes only inside the mission directory (C-001).
- [ ] Commit message is conventional.
- [ ] No `gh issue close|edit|comment` was run (C-002).

## Definition of Done

- [ ] All seven subtasks above complete with their validation checklists ticked.
- [ ] `triage-report.md` exists, is ≤ 600 lines, and satisfies every FR/NFR/C in `spec.md`.
- [ ] `snapshot.json` is committed alongside the report.
- [ ] `review-notes.md` documents the spec compliance walk.
- [ ] `git status` is clean inside the mission directory after commit.
- [ ] No commits outside `kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/`.

## Reviewer Guidance

When reviewing this WP, focus on:

1. **Coverage**: Does the report classify every issue from the snapshot? Cross-check `snapshot.json` length against the per-milestone tables.
2. **Verdict justifications**: Does every `deprecate` cite a real commit or PR? Does every `merge-into:#NNN` reference an issue that ends up in the survivor list?
3. **Survivor ordering**: Does the rank order respect every `depends_on` edge? Spot-check three random survivors.
4. **Recommendations are actionable**: Could a maintainer hand the rank-1 recommendation to `/spec-kitty.specify` without rewriting it?
5. **Scope discipline**: Did the agent stay inside the mission folder? `git diff --stat HEAD~1 HEAD` should show files only under `kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/`.
6. **No GitHub writes**: Search for `gh issue close|edit|comment` in the WP transcript or shell history.

## Risks and Mitigations

| Risk                                                                 | Mitigation                                                                                                                  |
| -------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| Issue body too thin to classify confidently.                         | Default to `keep` + `bypass_eligible: no` with "needs maintainer scope review" note. Do not silently `deprecate`.           |
| `gh` not authenticated.                                              | T001 fails fast with a clear error.                                                                                         |
| Recent merge contradicts issue body.                                 | Record both signals in justification line; let maintainer arbitrate during implement-review.                                |
| Survivor sequencing the maintainer disagrees with.                   | Survivor list is advisory; reordering does not invalidate the report.                                                       |
| Recommendation overlaps active branch or PR.                         | T005 cross-checks `git branch -a --list 'claude/*'` and `gh pr list --state open` and excludes the conflicting survivor.    |

## Implementation Command

```bash
spec-kitty agent action implement WP01 --agent <agent-name>
```

Run from the project root. The lane and worktree are allocated by `lanes.json` after `finalize-tasks` completes.

## Activity Log

- 2026-04-27T05:43:08Z – claude:opus-4.7:implementer:implementer – shell_pid=31084 – Started implementation via action command
