# Quickstart: Reading and Acting on the Triage Report

**Mission**: `backlog-triage-and-roadmap-01KQ6PSF`
**Audience**: maintainer (you), or a future agent picking up where you left off.

The triage report is one markdown file at
`kitty-specs/backlog-triage-and-roadmap-01KQ6PSF/triage-report.md`. This guide
explains how to read it and what to do with each section. It is written so a
returning reader can act in under five minutes (Success Criterion 1).

## What the report contains, in order

1. **Snapshot metadata** — when the GitHub state was captured and a pointer to the raw `snapshot.json`. If the timestamp is older than ~24 hours, treat the report as stale (NFR-001).
2. **Per-milestone triage tables** — one table per milestone bucket, every open issue listed with its verdict.
3. **Detail blocks** — one per issue, with the full classification (size, dependencies, justification, bypass-eligibility).
4. **Prioritized Survivor List** — the ordered queue of `keep` issues. This is the "what next" answer.
5. **Recommended Next Missions** — one to three concrete proposals you can hand to `/spec-kitty.specify`.
6. **Deprecation follow-up list** — issue numbers to close on GitHub, with the link to whatever obsoleted them.

## How to act on each verdict

### `keep`

Find the issue's row in the **Prioritized Survivor List**. The first one (`rank: 1`) is the next thing to do. If `bypass_eligible: yes`, you may implement directly under conventional commits + CI gates without spawning a Spec Kitty mission. Otherwise, look at the **Recommended Next Missions** section to find which proposed mission groups it.

### `deprecate`

The detail block carries a one-line justification (typically a commit hash or PR link that obsoleted it). Open the GitHub issue, paste the justification as a closing comment, and close. The report aggregates these into a single follow-up list at the bottom for batch handling.

### `merge-into:#NNN`

The scope of this issue has been folded into another open issue (`#NNN`). Open this issue on GitHub, comment "Merged into #NNN — closing as duplicate," and close. The absorbing issue's detail block in the report describes what scope was added.

## Running a recommended mission

For each `MissionRecommendation` block, the report gives you the slug and a scope paragraph. Run:

```bash
spec-kitty agent mission create "<slug-from-recommendation>" --json
```

Then `/spec-kitty.specify` with the scope paragraph as your invocation text. The recommendation already lists which survivor issue numbers are in scope; reference them in the spec.

## When to rerun this triage

- The backlog has changed materially since the snapshot timestamp (new issues filed, several closed without going through the recommended missions).
- A recommended mission's scope diverged enough during its own specify phase that the survivor list ordering no longer reflects reality.
- More than ~30 days have elapsed since the snapshot.

A rerun is a fresh `/spec-kitty.specify backlog-triage-and-roadmap`. The new mission gets its own ULID; the old report stays in the repo as history.

## Validation before the maintainer accepts

The implement phase will self-review the report against the spec's FRs / NFRs / Cs. Maintainer acceptance gates are:

- Every open issue from the snapshot has exactly one verdict.
- Every `deprecate` carries a justification link.
- Every `merge-into` references a real `keep` issue number.
- The survivor list respects dependency order.
- At least one recommendation covers `rank = 1`.

If any of those fail, reject the implement phase; the agent regenerates and re-reviews.
