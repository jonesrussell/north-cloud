# Idiomatic Go Workspaces Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the hybrid go.work + replace-directives + GOWORK=off pattern with idiomatic Go workspace usage: go.work resolves cross-module deps, replace directives removed, GOWORK enabled everywhere.

**Architecture:** Go workspaces were designed to eliminate `replace` directives for local multi-module repos. Currently every service has both a `replace github.com/north-cloud/infrastructure => ../infrastructure` directive AND a `GOWORK=off` env that prevents the workspace from being used — making go.work dead weight. The fix: remove replace directives (workspace handles resolution), remove GOWORK=off (workspace is now active), keep the golangci-lint-action CI step GOWORK=off-isolated (it runs from repo root which has no module).

**Tech Stack:** Go 1.26+, go.work (workspace), golangci-lint, Taskfile, GitHub Actions

---

## Services with replace directives to remove

Nine services have `replace github.com/north-cloud/infrastructure => ../infrastructure`:
`auth`, `classifier`, `click-tracker`, `index-manager`, `mcp-north-cloud`, `pipeline`, `publisher`, `search`, `source-manager`

One service (`crawler`) also replaces `github.com/jonesrussell/north-cloud/index-manager => ../index-manager`.

Eleven service Taskfiles have `GOWORK: "off"` at line 13:
`auth`, `classifier`, `click-tracker`, `crawler`, `index-manager`, `mcp-north-cloud`, `nc-http-proxy`, `pipeline`, `publisher`, `search`, `source-manager`

---

### Task 1: Remove replace directives — auth

**Files:**
- Modify: `auth/go.mod`

**Step 1: Remove the replace directive**

Find and delete this line:
```
replace github.com/north-cloud/infrastructure => ../infrastructure
```

**Step 2: Run go mod tidy to verify**

```bash
cd /home/fsd42/dev/north-cloud/auth && go mod tidy
```
Expected: no errors. go.sum updated.

**Step 3: Run tests to verify nothing broke**

```bash
cd /home/fsd42/dev/north-cloud/auth && go test ./...
```
Expected: PASS

**Step 4: Commit**

```bash
git add auth/go.mod auth/go.sum
git commit -m "chore(auth): remove infrastructure replace directive (use workspace)"
```

---

### Task 2: Remove replace directives — classifier

**Files:**
- Modify: `classifier/go.mod`

**Step 1: Remove the replace directive**

Find and delete this line from `classifier/go.mod`:
```
replace github.com/north-cloud/infrastructure => ../infrastructure
```

**Step 2: Run go mod tidy**

```bash
cd /home/fsd42/dev/north-cloud/classifier && go mod tidy
```

**Step 3: Run tests**

```bash
cd /home/fsd42/dev/north-cloud/classifier && go test ./...
```
Expected: PASS

**Step 4: Commit**

```bash
git add classifier/go.mod classifier/go.sum
git commit -m "chore(classifier): remove infrastructure replace directive (use workspace)"
```

---

### Task 3: Remove replace directives — click-tracker

**Files:**
- Modify: `click-tracker/go.mod`

**Step 1: Remove the replace directive**

Delete from `click-tracker/go.mod`:
```
replace github.com/north-cloud/infrastructure => ../infrastructure
```

**Step 2: Run go mod tidy**

```bash
cd /home/fsd42/dev/north-cloud/click-tracker && go mod tidy
```

**Step 3: Run tests**

```bash
cd /home/fsd42/dev/north-cloud/click-tracker && go test ./...
```

**Step 4: Commit**

```bash
git add click-tracker/go.mod click-tracker/go.sum
git commit -m "chore(click-tracker): remove infrastructure replace directive (use workspace)"
```

---

### Task 4: Remove replace directives — crawler

**Files:**
- Modify: `crawler/go.mod`

**Step 1: Remove the entire replace block**

The crawler has a multi-line replace block (around line 27):
```
replace (
	github.com/jonesrussell/north-cloud/index-manager => ../index-manager
	github.com/north-cloud/infrastructure => ../infrastructure
)
```
Delete the entire block (both entries are workspace modules listed in go.work).

**Step 2: Run go mod tidy**

```bash
cd /home/fsd42/dev/north-cloud/crawler && go mod tidy
```

**Step 3: Run tests**

```bash
cd /home/fsd42/dev/north-cloud/crawler && go test ./...
```

**Step 4: Commit**

```bash
git add crawler/go.mod crawler/go.sum
git commit -m "chore(crawler): remove workspace replace directives (use workspace)"
```

---

### Task 5: Remove replace directives — index-manager

**Files:**
- Modify: `index-manager/go.mod`

**Step 1: Remove the replace directive**

Delete from `index-manager/go.mod`:
```
replace github.com/north-cloud/infrastructure => ../infrastructure
```

**Step 2: Run go mod tidy**

```bash
cd /home/fsd42/dev/north-cloud/index-manager && go mod tidy
```

**Step 3: Run tests**

```bash
cd /home/fsd42/dev/north-cloud/index-manager && go test ./...
```

**Step 4: Commit**

```bash
git add index-manager/go.mod index-manager/go.sum
git commit -m "chore(index-manager): remove infrastructure replace directive (use workspace)"
```

---

### Task 6: Remove replace directives — mcp-north-cloud

**Files:**
- Modify: `mcp-north-cloud/go.mod`

**Step 1: Remove the replace directive**

Delete from `mcp-north-cloud/go.mod`:
```
replace github.com/north-cloud/infrastructure => ../infrastructure
```

**Step 2: Run go mod tidy**

```bash
cd /home/fsd42/dev/north-cloud/mcp-north-cloud && go mod tidy
```

**Step 3: Run tests**

```bash
cd /home/fsd42/dev/north-cloud/mcp-north-cloud && go test ./...
```

**Step 4: Commit**

```bash
git add mcp-north-cloud/go.mod mcp-north-cloud/go.sum
git commit -m "chore(mcp-north-cloud): remove infrastructure replace directive (use workspace)"
```

---

### Task 7: Remove replace directives — pipeline

**Files:**
- Modify: `pipeline/go.mod`

**Step 1: Remove the replace directive**

Delete from `pipeline/go.mod`:
```
replace github.com/north-cloud/infrastructure => ../infrastructure
```

**Step 2: Run go mod tidy**

```bash
cd /home/fsd42/dev/north-cloud/pipeline && go mod tidy
```

**Step 3: Run tests**

```bash
cd /home/fsd42/dev/north-cloud/pipeline && go test ./...
```

**Step 4: Commit**

```bash
git add pipeline/go.mod pipeline/go.sum
git commit -m "chore(pipeline): remove infrastructure replace directive (use workspace)"
```

---

### Task 8: Remove replace directives — publisher

**Files:**
- Modify: `publisher/go.mod`

**Step 1: Remove the replace directive**

Delete from `publisher/go.mod`:
```
replace github.com/north-cloud/infrastructure => ../infrastructure
```

**Step 2: Run go mod tidy**

```bash
cd /home/fsd42/dev/north-cloud/publisher && go mod tidy
```

**Step 3: Run tests**

```bash
cd /home/fsd42/dev/north-cloud/publisher && go test ./...
```

**Step 4: Commit**

```bash
git add publisher/go.mod publisher/go.sum
git commit -m "chore(publisher): remove infrastructure replace directive (use workspace)"
```

---

### Task 9: Remove replace directives — search

**Files:**
- Modify: `search/go.mod`

**Step 1: Remove the replace directive**

Delete from `search/go.mod`:
```
replace github.com/north-cloud/infrastructure => ../infrastructure
```

**Step 2: Run go mod tidy**

```bash
cd /home/fsd42/dev/north-cloud/search && go mod tidy
```

**Step 3: Run tests**

```bash
cd /home/fsd42/dev/north-cloud/search && go test ./...
```

**Step 4: Commit**

```bash
git add search/go.mod search/go.sum
git commit -m "chore(search): remove infrastructure replace directive (use workspace)"
```

---

### Task 10: Remove replace directives — source-manager

**Files:**
- Modify: `source-manager/go.mod`

**Step 1: Remove the replace directive**

Delete from `source-manager/go.mod`:
```
replace github.com/north-cloud/infrastructure => ../infrastructure
```

**Step 2: Run go mod tidy**

```bash
cd /home/fsd42/dev/north-cloud/source-manager && go mod tidy
```

**Step 3: Run tests**

```bash
cd /home/fsd42/dev/north-cloud/source-manager && go test ./...
```

**Step 4: Commit**

```bash
git add source-manager/go.mod source-manager/go.sum
git commit -m "chore(source-manager): remove infrastructure replace directive (use workspace)"
```

---

### Task 11: Sync go.work.sum

After all replace directives are removed and go.mod files are tidy, the workspace checksum file needs updating.

**Files:**
- Modify: `go.work.sum`

**Step 1: Sync the workspace**

```bash
cd /home/fsd42/dev/north-cloud && go work sync
```
Expected: no errors, `go.work.sum` updated.

**Step 2: Commit**

```bash
git add go.work.sum
git commit -m "chore(workspace): sync go.work.sum after removing replace directives"
```

---

### Task 12: Remove GOWORK=off from service Taskfiles

Every service Taskfile has `GOWORK: "off"` as a top-level env (around line 13). Remove it from all 11 services. The workspace is now the source of truth.

**Files:**
- Modify: `auth/Taskfile.yml`
- Modify: `classifier/Taskfile.yml`
- Modify: `click-tracker/Taskfile.yml`
- Modify: `crawler/Taskfile.yml`
- Modify: `index-manager/Taskfile.yml`
- Modify: `mcp-north-cloud/Taskfile.yml`
- Modify: `nc-http-proxy/Taskfile.yml`
- Modify: `pipeline/Taskfile.yml`
- Modify: `publisher/Taskfile.yml`
- Modify: `search/Taskfile.yml`
- Modify: `source-manager/Taskfile.yml`

**Step 1: Remove GOWORK env from each service Taskfile**

In each file, find and remove:
```yaml
  GOWORK: "off"
```
(It appears as a key under the `env:` block near the top of each file, around line 13.)

**Step 2: Run lint to verify Taskfiles are valid**

```bash
cd /home/fsd42/dev/north-cloud && task lint:publisher && task lint:auth
```
Expected: no errors (linter now runs with workspace active, infrastructure resolved via go.work).

**Step 3: Run tests for a couple services to verify**

```bash
cd /home/fsd42/dev/north-cloud && task test:publisher && task test:auth
```
Expected: PASS

**Step 4: Commit**

```bash
git add auth/Taskfile.yml classifier/Taskfile.yml click-tracker/Taskfile.yml \
        crawler/Taskfile.yml index-manager/Taskfile.yml mcp-north-cloud/Taskfile.yml \
        nc-http-proxy/Taskfile.yml pipeline/Taskfile.yml publisher/Taskfile.yml \
        search/Taskfile.yml source-manager/Taskfile.yml
git commit -m "chore(taskfiles): remove GOWORK=off — workspace now active for all services"
```

---

### Task 13: Clean up root Taskfile

The root `Taskfile.yml` has `env: { GOWORK: "off" }` on the `vendor:infrastructure` task and `GOWORK=off` in the integration test command.

**Files:**
- Modify: `Taskfile.yml`

**Step 1: Remove GOWORK=off from vendor:infrastructure task**

Find:
```yaml
  vendor:infrastructure:
    desc: "Vendor infrastructure module"
    dir: infrastructure
    env: { GOWORK: "off" }
    cmds:
      - go mod vendor
```
Remove the `env: { GOWORK: "off" }` line.

**Step 2: Remove GOWORK=off from integration test command**

Find (around line 1439):
```bash
cd tests/integration/pipeline && GOWORK=off go test -tags=integration -v -timeout=10m ./...
```
Change to:
```bash
cd tests/integration/pipeline && go test -tags=integration -v -timeout=10m ./...
```

**Step 3: Verify integration test module is in go.work**

```bash
grep "integration" /home/fsd42/dev/north-cloud/go.work
```
Expected: `./tests/integration/pipeline` is listed — it already is.

**Step 4: Commit**

```bash
git add Taskfile.yml
git commit -m "chore(taskfile): remove GOWORK=off from root tasks — workspace now active"
```

---

### Task 14: Update CI workflow

The `golangci-lint-action` step has `GOWORK: off` — this is correct and must stay (the action runs from the repo root which has no Go module; workspace mode would also fail from root). No change needed there.

**Files:**
- Modify: `.github/workflows/test.yml`

**Step 1: Verify current lint action config is correct**

Check that the golangci-lint-action step still has `env: GOWORK: off`:
```yaml
      - name: Install golangci-lint
        uses: golangci/golangci-lint-action@v7
        with:
          version: v2.10.1
          install-only: true
        env:
          GOWORK: off
```
This must stay as-is. The actual linting is done by `task lint:SERVICE` which runs from each service directory where the workspace resolves correctly.

**Step 2: No changes needed — commit note only**

If no edits were needed, skip the commit. The CI workflow is already correct from the earlier fix.

---

### Task 15: Run full local verification

**Step 1: Run all linters**

```bash
cd /home/fsd42/dev/north-cloud && task lint
```
Expected: all services pass, no GOWORK-related errors.

**Step 2: Run all tests**

```bash
cd /home/fsd42/dev/north-cloud && task test
```
Expected: all services pass.

**Step 3: Verify no replace directives remain for workspace modules**

```bash
grep -r "replace.*infrastructure\|replace.*index-manager" /home/fsd42/dev/north-cloud/*/go.mod
```
Expected: no output.

**Step 4: Verify no GOWORK=off remains in service Taskfiles**

```bash
grep -r "GOWORK" /home/fsd42/dev/north-cloud/*/Taskfile.yml
```
Expected: no output.

**Step 5: Push to trigger CI**

```bash
git push origin main
```

Monitor: https://github.com/jonesrussell/north-cloud/actions

---

## Notes

- `nc-http-proxy` does not import `infrastructure` — its Taskfile GOWORK removal is still needed (it was set to off preventatively), but no go.mod change is required.
- The `golangci-lint-action` `GOWORK: off` must remain — it's there because the action runs from the repo root (no module there), not because workspace is wrong.
- After this change, `go mod tidy` in any service will work correctly with workspace active — no need to remember to set GOWORK=off manually.
- `go work sync` (Task 11) ensures `go.work.sum` has checksums for all workspace modules' transitive dependencies.
