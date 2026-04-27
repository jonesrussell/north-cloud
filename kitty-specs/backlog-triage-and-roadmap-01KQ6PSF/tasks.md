# Tasks: Backlog Triage and Roadmap

**Mission**: `backlog-triage-and-roadmap-01KQ6PSF`
**Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)
**Branch contract**: planning base = `main`, merge target = `main`.

This mission produces a single markdown report. The work is sequential
(snapshot → classify → annotate → sequence → recommend → write → review) and
fits inside one focused work package. Splitting it into multiple WPs would add
worktree overhead with no parallelism benefit, so we keep it as **WP01** only.

## Subtask Index

| ID    | Description                                                                                  | WP    | Parallel |
| ----- | -------------------------------------------------------------------------------------------- | ----- | -------- |
| T001  | Pre-flight checks and pin a GitHub snapshot                                                  | WP01  |          |
| T002  | Per-issue read and classify (verdict, scope, justification)                                  | WP01  |          |
| T003  | Annotate `keep` records with size, dependencies, and bypass-eligibility                      | WP01  |          |
| T004  | Sequence survivors via topological sort with milestone and size tiebreakers                  | WP01  |          |
| T005  | Draft Recommended Next Missions and cross-check active branches and PRs                      | WP01  |          |
| T006  | Compose `triage-report.md` from the records                                                  | WP01  |          |
| T007  | Self-review against spec FRs/NFRs/Cs and commit                                              | WP01  |          |

No subtask is `[P]` parallel — each step depends on the previous step's output.

## Work Packages

### WP01 — Generate Triage Report

- **Goal**: Produce `kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/triage-report.md` matching every requirement in `spec.md`, plus the supporting `snapshot.json`.
- **Priority**: P0 (this is the entire mission).
- **Independent test**: A maintainer reading only `triage-report.md` can decide the next mission to run within five minutes (Success Criterion 1) and every open issue at snapshot time has exactly one verdict (Success Criterion 2).
- **Estimated prompt size**: ~450 lines.
- **Execution mode**: `planning_artifact` (no source code change).
- **Authoritative surface**: `kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/`.

#### Included subtasks

- [ ] T001 Pre-flight checks and pin a GitHub snapshot (WP01)
- [ ] T002 Per-issue read and classify (WP01)
- [ ] T003 Annotate `keep` records with size, dependencies, and bypass-eligibility (WP01)
- [ ] T004 Sequence survivors with topological sort + tiebreakers (WP01)
- [ ] T005 Draft Recommended Next Missions and cross-check active work (WP01)
- [ ] T006 Compose `triage-report.md` (WP01)
- [ ] T007 Self-review against spec FRs/NFRs/Cs and commit (WP01)

#### Implementation sketch

1. **Pre-flight** (T001): Verify `gh auth status`, capture `git log --oneline --no-merges --since="30 days ago"` to local memory, run `gh issue list --state open --json number,title,milestone,labels,body --limit 100`, and write the raw JSON to `snapshot.json` with the UTC timestamp recorded.
2. **Classify** (T002): For each of the 13 issues, read body and any `closedByPullRequestsReferences`. Decide verdict (`keep` / `deprecate` / `merge-into:#NNN`). Record one-line scope. For deprecate/merge, attach justification with commit hash or issue link.
3. **Annotate** (T003): For each `keep`, attach `size ∈ {S, M, L}` with rationale, `depends_on`, `blocks`, and conservative `bypass_eligible`.
4. **Sequence** (T004): Topological sort over `depends_on` edges. Tiebreak by milestone priority (Lead Intelligence > Phase 3 > Developer Experience), then by size ascending. Validate: rank 1 has empty `depends_on`.
5. **Recommend** (T005): Cross-check `git branch -a --list 'claude/*'` and `gh pr list --state open` to avoid recommending in-flight scope. Group the top of the survivor list into 1–3 `MissionRecommendation`s. Each must cover at least the rank-1 survivor.
6. **Compose** (T006): Render the markdown report per `data-model.md` layout: snapshot metadata, per-milestone tables, detail blocks, prioritized survivor list, recommendations, deprecation follow-up list.
7. **Self-review and commit** (T007): Walk every FR / NFR / C in the spec; document compliance line-by-line in a private review note. If any fails, return to the relevant subtask. On pass, conventional-commit the report + snapshot.

#### Parallel opportunities

None. The pipeline is strictly sequential; each subtask consumes the previous.

#### Dependencies

None. WP01 is the only WP.

#### Risks (covered in plan.md premortem)

- Issue body too thin to classify confidently → mark `keep` + `bypass_eligible: no` with a "needs maintainer scope review" note. Do not silently `deprecate`.
- `gh` not authenticated → fail fast in T001 with a clear error.
- Recent merge contradicted by issue body → record both signals; let maintainer arbitrate.
- Survivor sequencing the maintainer disagrees with → list is advisory.

## Requirement Coverage

WP01 covers every requirement in the spec because the mission is a single
sequential report-generation effort:

- **FRs**: FR-001 through FR-013 — all addressed by WP01.
- **NFRs**: NFR-001 through NFR-004 — all addressed by WP01.
- **Constraints**: C-001 through C-006 — enforced throughout WP01.

## MVP Scope

WP01 is the MVP. There is no smaller deliverable that produces a usable triage
report.

## Next Command

After `finalize-tasks` writes `lanes.json`, the mission is ready for
`/spec-kitty.implement` (or for the implement-review skill to drive
end-to-end).
