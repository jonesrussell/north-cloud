# Continuous Spec Observability — Design Spec

**Date**: 2026-03-18
**Status**: Approved
**Scope**: Wiring `tools/drift-detector.sh` into all three enforcement surfaces (Taskfile, lefthook, CI) as a hard gate.

---

## Problem

The repo has a working spec drift detector (`tools/drift-detector.sh`) and a PR template checkbox asking contributors to update specs — but no enforcement. The drift-detector is orphaned: not referenced in any workflow, Taskfile task, or git hook.

The codified context (`CLAUDE.md`, `docs/specs/`) is the root of trust for multi-repo orchestration, cross-project audits, and the drift governor. If specs drift from implementation, everything downstream becomes probabilistic.

## Decision

**Hard gate, no escape hatch.** Every PR that touches service code must have current specs. The drift-detector exits non-zero when any spec is stale — that exit code blocks merges at every enforcement layer.

Rationale: Solo founder environment. No political cost to hard gates. Soft warnings accumulate into spec debt. Escape hatches erode invariants.

## Design

### 1. Taskfile — `drift:check` task

New top-level task wrapping the existing script:

```yaml
drift:check:
  desc: "Check for spec drift (service code newer than its spec)"
  cmds:
    - tools/drift-detector.sh {{.CLI_ARGS | default "5"}}
```

Wired into all CI task variants as the **first** step (fail fast — no point linting or testing if architecture is stale):

```yaml
ci:
  cmds:
    - task: drift:check
    - task: lint
    - task: test
    - task: vuln

ci:changed:
  cmds:
    - task: drift:check
    - task: lint:changed
    - task: test:changed
    - task: vuln:changed

ci:force:
  cmds:
    - task: drift:check
    # ... existing lint:force, test, vuln steps
```

### 2. lefthook — pre-push drift check

New command in the existing `pre-push` block:

```yaml
pre-push:
  parallel: true
  commands:
    go-test:
      # ... existing ...

    spec-drift:
      run: tools/drift-detector.sh 5
```

Design decisions:
- **`parallel: true`**: Runs alongside `go-test`. Both are independent checks.
- **No glob filter**: The script itself determines if any specs are affected. If none, exits 0 in ~50ms. Pre-filtering would introduce false negatives.
- **Calls script directly** (not `task drift:check`): lefthook should not depend on Task being installed. Matches the existing pattern where `go-lint` calls `golangci-lint` directly.

### 3. CI workflow — `spec-drift` job in `test.yml`

New job running in parallel with lint/test/vuln:

```yaml
spec-drift:
  name: Spec Drift Check
  runs-on: ubuntu-latest
  steps:
    - name: Checkout code
      uses: actions/checkout@v4
      with:
        fetch-depth: 0

    - name: Check spec drift
      run: tools/drift-detector.sh 20
```

Design decisions:
- **`fetch-depth: 0`**: Drift-detector compares `git log` timestamps. Shallow clones give incorrect results.
- **20 commits**: PRs accumulate commits; CI should look deeper than local checks (5).
- **No dependency on `detect-changes`**: Drift detection is global, not scoped to changed services. Adding a dependency would couple jobs unnecessarily.
- **Parallel with lint/test/vuln**: Independent check. Adds zero wall-clock time.
- **No PR comment**: Job stdout (the drift-detector table) shows in the Actions log. Comments add permissions, maintenance, and noise. YAGNI.

### 4. CLAUDE.md updates

**Quick Reference** — add entry:
> **Spec drift**: `task drift:check` (checks last 5 commits). Runs automatically on pre-push and CI. Hard gate — PRs cannot merge with stale specs.

**CRITICAL Rules — Before Committing** — add item 5:
> 5. **Verify specs are current**: `task drift:check` — if your changes touch service code, the corresponding spec in `docs/specs/` must be updated in the same PR

**Git Workflow — Before Committing** — add item 5:
> 5. Check spec drift: `task drift:check`

No changes to the orchestration table (already maps services to specs). No changes to the `updating-codified-context` skill (it already advises spec updates — the gate enforces what the skill recommends).

### 5. Drift-detector output improvements

Update `tools/drift-detector.sh` output for clarity:

- `WARNING` → `STALE` (unambiguous for a hard gate)
- Add explicit `Fix:` line with the spec path (copy-pasteable)
- Show `OK` specs for confirmation the detector scanned the full mapping
- Final summary: `N spec(s) stale. Update specs before merging.`

Target output:

```
=== Drift Detector ===
Checking last 20 commits for spec drift...

Affected specs:

  STALE: docs/specs/classification.md
    Service code updated more recently than spec.
    Changed files:
      classifier/internal/router/topic.go
      classifier/internal/config/config.go
    Fix: Update docs/specs/classification.md to reflect current behavior, then commit.

  OK: docs/specs/content-routing.md (updated after latest service change)

1 spec(s) stale. Update specs before merging.
```

No JSON output, no machine-readable format, no PR comment API. The script stays simple. The CI log is the interface.

## What this does NOT cover

- **Classifier ML drift**: Handled separately by `ai-observer` drift governor + `.github/workflows/drift-remediation.yml`. Orthogonal system.
- **Auto-issue creation for spec drift**: Spec drift should be fixed in the PR that caused it, not tracked as a separate issue.
- **Spec content validation**: The detector checks freshness (timestamps), not correctness (content). Content review remains a human/session responsibility.

## Files modified

| File | Change |
|---|---|
| `Taskfile.yml` | Add `drift:check` task; wire into `ci`, `ci:changed`, `ci:force` |
| `lefthook.yml` | Add `spec-drift` command to `pre-push` |
| `.github/workflows/test.yml` | Add `spec-drift` job |
| `CLAUDE.md` | Add Quick Reference, CRITICAL Rules, Git Workflow entries |
| `tools/drift-detector.sh` | Improve output formatting (STALE/OK labels, Fix line, summary) |
