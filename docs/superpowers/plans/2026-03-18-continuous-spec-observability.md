# Continuous Spec Observability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the existing `tools/drift-detector.sh` into Taskfile, lefthook, and CI as a hard gate that blocks merges when specs are stale.

**Architecture:** Three enforcement surfaces (Taskfile, lefthook pre-push, CI job) all call the same drift-detector script. No new logic — just integration and output improvements to the existing engine.

**Tech Stack:** Bash, Taskfile v3, lefthook, GitHub Actions

**Spec:** `docs/superpowers/specs/2026-03-18-continuous-spec-observability-design.md`

---

### Task 1: Improve drift-detector output

**Files:**
- Modify: `tools/drift-detector.sh:70-136`

The script works but uses ambiguous `WARNING` language and doesn't give actionable fix instructions. Update the output section to use `STALE`/`OK` labels with explicit fix lines.

- [ ] **Step 1: Verify script is executable and runs clean**

Run: `tools/drift-detector.sh 5`
Expected: Exits 0 or 1 with current output format. Confirms baseline works.

- [ ] **Step 2: Update the output section — STALE label with fix instruction**

Replace the reporting loop (lines 93-127) with improved output:

```bash
echo "Affected specs:"
echo ""

WARNINGS=0
for spec in "${!AFFECTED_SPECS[@]}"; do
  spec_path="$REPO_ROOT/$spec"
  if [ -f "$spec_path" ]; then
    # Compare git commit timestamps: when was the spec last touched vs the service code?
    spec_last_commit=$(git log -1 --format=%ct -- "$spec" 2>/dev/null)
    spec_last_commit=${spec_last_commit:-0}
    # Find the latest commit time for any matched service file
    service_last_commit=0
    for pattern in "${!PATTERN_TO_SPEC[@]}"; do
      if [ "${PATTERN_TO_SPEC[$pattern]}" = "$spec" ]; then
        pattern_commit=$(git log -1 --format=%ct -- "$pattern" 2>/dev/null)
        pattern_commit=${pattern_commit:-0}
        if [ "$pattern_commit" -gt "$service_last_commit" ]; then
          service_last_commit=$pattern_commit
        fi
      fi
    done

    if [ "$spec_last_commit" -lt "$service_last_commit" ]; then
      echo "  STALE: $spec"
      echo "    Service code updated more recently than spec."
      echo "    Changed files:"
      echo -e "${SPEC_CHANGES[$spec]}" | sort -u | while IFS= read -r line; do
        [ -z "$line" ] && continue
        echo "      $line"
      done
      echo "    Fix: Update $spec to reflect current behavior, then commit."
      echo ""
      WARNINGS=$((WARNINGS + 1))
    else
      echo "  OK: $spec (updated after latest service change)"
      echo -e "${SPEC_CHANGES[$spec]}" | sort -u | head -10
    fi
  else
    echo "  MISSING: $spec (does not exist yet)"
    echo "    Fix: Create $spec with current architecture documentation."
    echo ""
    WARNINGS=$((WARNINGS + 1))
  fi
done
```

- [ ] **Step 3: Update the summary line (lines 129-136)**

Replace the final summary block:

```bash
echo ""
if [ $WARNINGS -gt 0 ]; then
  echo "$WARNINGS spec(s) stale. Update specs before merging."
  exit 1
else
  echo "All affected specs are up to date."
  exit 0
fi
```

- [ ] **Step 4: Run drift-detector and verify new output format**

Run: `tools/drift-detector.sh 5`
Expected: Output uses `STALE`/`OK` labels. If any specs are stale, shows `Fix:` line and exits 1. If all current, shows `OK` labels and exits 0.

- [ ] **Step 5: Commit**

```bash
git add tools/drift-detector.sh
git commit -m "fix(tools): improve drift-detector output with STALE/OK labels and fix instructions"
```

---

### Task 2: Add `drift:check` Taskfile task

**Files:**
- Modify: `Taskfile.yml:78-97` (tasks section — add new task, modify ci/ci:changed/ci:force)

- [ ] **Step 1: Add the `drift:check` task**

Add after line 82 (after the `default` task), before `ci:`:

```yaml
  drift:check:
    desc: "Check for spec drift (service code newer than its spec)"
    cmds:
      - tools/drift-detector.sh {{.CLI_ARGS | default "5"}}
```

- [ ] **Step 2: Wire `drift:check` as first step in `ci:`**

Modify the `ci:` task (line 84) to add drift:check before lint:

```yaml
  ci:
    desc: "Run lint, test, and vuln (CI pipeline)"
    cmds:
      - task: drift:check
      - task: lint
      - task: test
      - task: vuln
```

- [ ] **Step 3: Wire `drift:check` as first step in `ci:changed`**

Modify the `ci:changed` task (line 91):

```yaml
  ci:changed:
    desc: "Run lint, test, and vuln only for changed services (vs HEAD~1)"
    cmds:
      - task: drift:check
      - task: lint:changed
      - task: test:changed
      - task: vuln:changed
```

- [ ] **Step 4: Wire `drift:check` as first step in `ci:force`**

Modify the `ci:force` task (line 98) — add as first command before `lint:force`:

```yaml
  ci:force:
    desc: "Run full CI pipeline (lint, test, vuln) ignoring cache. Use before pushing to match CI."
    cmds:
      - task: drift:check
      - task: lint:force
      # ... rest unchanged
```

- [ ] **Step 5: Verify `task drift:check` works standalone**

Run: `task drift:check`
Expected: Runs drift-detector.sh with default 5 commits, shows STALE/OK output.

- [ ] **Step 6: Verify `task drift:check -- 10` passes CLI args**

Run: `task drift:check -- 10`
Expected: Runs drift-detector.sh with 10 commits.

- [ ] **Step 7: Commit**

```bash
git add Taskfile.yml
git commit -m "feat(taskfile): add drift:check task, wire into ci pipelines"
```

---

### Task 3: Add lefthook pre-push drift check

**Files:**
- Modify: `lefthook.yml:38-54` (pre-push section)

- [ ] **Step 1: Add `spec-drift` command to pre-push block**

Add after the existing `go-test` command (after line 54):

```yaml
    spec-drift:
      run: tools/drift-detector.sh 5
```

The full `pre-push` block should now be:

```yaml
pre-push:
  parallel: true
  commands:
    go-test:
      glob: "**/*.go"
      run: |
        changed_services=""
        for f in {push_files}; do
          svc=$(echo "$f" | cut -d/ -f1)
          if [ -f "$svc/go.mod" ] && ! echo "$changed_services" | grep -q "$svc"; then
            changed_services="$changed_services $svc"
          fi
        done
        for svc in $changed_services; do
          echo "Testing $svc..."
          (cd "$svc" && GOWORK=off go test ./... -count=1)
        done

    spec-drift:
      run: tools/drift-detector.sh 5
```

- [ ] **Step 2: Verify lefthook config is valid**

Run: `lefthook run pre-push --dry-run`
Expected: Shows both `go-test` and `spec-drift` commands listed.

- [ ] **Step 3: Commit**

```bash
git add lefthook.yml
git commit -m "feat(hooks): add spec-drift check to lefthook pre-push"
```

---

### Task 4: Add CI workflow job

**Files:**
- Modify: `.github/workflows/test.yml:14` (jobs section — add new job)

- [ ] **Step 1: Add `spec-drift` job to test.yml**

Add the new job after `detect-changes` (after line 61, before the `lint:` job). This job has no `needs:` — it runs in parallel with all other jobs:

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

- [ ] **Step 2: Verify YAML syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/test.yml'))"`
Expected: No errors. (If python3-yaml not available, use `yamllint` or just review the indentation manually.)

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/test.yml
git commit -m "ci: add spec-drift hard gate to test workflow"
```

---

### Task 5: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md:91` (Quick Reference — after Taskfile line)
- Modify: `CLAUDE.md:107-112` (CRITICAL Rules — Before Making Changes checklist)
- Modify: `CLAUDE.md:195-199` (Git Workflow — Before Committing checklist)

- [ ] **Step 1: Add spec drift entry to Quick Reference**

After line 91 (the Taskfile line ending with `task ci:changed`), add:

```
**Spec drift**: `task drift:check` (checks last 5 commits). Runs automatically on pre-push and CI. Hard gate — PRs cannot merge with stale specs.
```

- [ ] **Step 2: Add spec drift to CRITICAL Rules — Before Making Changes**

After line 112 (`4. **Understand service boundaries**: Each service is independent with its own database`), add:

```
5. **Verify specs are current**: `task drift:check` — if your changes touch service code, the corresponding spec in `docs/specs/` must be updated in the same PR
```

- [ ] **Step 3: Add spec drift to Git Workflow — Before Committing checklist**

After line 199 (`4. Check multi-service dependencies if applicable`), add:

```
5. Check spec drift: `task drift:check`
```

- [ ] **Step 4: Verify CLAUDE.md reads correctly**

Read the modified sections and confirm the new entries fit the surrounding format and style.

- [ ] **Step 5: Commit**

```bash
git add CLAUDE.md
git commit -m "docs(claude): add spec drift hard gate to quick reference and commit checklist"
```

---

### Task 6: End-to-end verification

**Files:** None (verification only)

- [ ] **Step 1: Run `task drift:check` and confirm output format**

Run: `task drift:check`
Expected: STALE/OK output with fix instructions where applicable.

- [ ] **Step 2: Run `task ci:changed` and confirm drift:check runs first**

Run: `task ci:changed`
Expected: Drift check output appears before lint/test/vuln output.

- [ ] **Step 3: Verify lefthook shows spec-drift in pre-push**

Run: `lefthook run pre-push --dry-run`
Expected: `spec-drift` command listed alongside `go-test`.

- [ ] **Step 4: Verify test.yml has valid syntax and spec-drift job**

Run: `grep -A 10 'spec-drift:' .github/workflows/test.yml`
Expected: Shows the spec-drift job with fetch-depth: 0 and drift-detector.sh 20.

- [ ] **Step 5: Verify CLAUDE.md has all new entries**

Run: `grep -n 'drift' CLAUDE.md`
Expected: Shows entries in Quick Reference, CRITICAL Rules (Before Making Changes), and Git Workflow (Before Committing) sections.
