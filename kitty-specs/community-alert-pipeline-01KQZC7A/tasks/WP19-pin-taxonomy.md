---
work_package_id: WP19
title: Pin Taxonomy v1.1.0
dependencies:
- WP04
- WP18
requirement_refs:
- C-003
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T082
- T083
- T084
- T085
phase: C
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: alert-crawler/go.mod
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- alert-crawler/go.mod
- alert-crawler/go.sum
priority: P1
tags: []
---

# WP19 — Pin Taxonomy v1.1.0

## Objective

Remove the `replace github.com/jonesrussell/indigenous-taxonomy => ../indigenous-taxonomy` directive from `alert-crawler/go.mod` and pin to the released `v1.1.0` tag. Verify the build works without `replace`. This WP gates final mission merge — leaving `replace` in place would break consumers of the alert-crawler image when deployed.

## Context

- Plan §TC-008, §Phased Build Sequence Phase C.1
- Spec §5 C-003 (direct import of indigenous-taxonomy)
- Risk PR-001 (coordination drift)

## Branch Strategy

Standard. **Critical dependency**: WP04 (taxonomy v1.1.0 tag) MUST be live (visible to `proxy.golang.org`) before WP19 can succeed. WP18 must be feature-complete (this WP changes go.mod, which would invalidate the parallel-dev environment).

## Subtasks

### T082 — Wait for indigenous-taxonomy v1.1.0 tag

**Purpose**: Confirm the dependency is available before modifying `go.mod`.

**Steps**:
1. Verify the tag exists upstream:
   ```bash
   git ls-remote --tags https://github.com/jonesrussell/indigenous-taxonomy.git | grep v1.1.0
   ```
2. Verify the Go module proxy has indexed it:
   ```bash
   curl -fsSL https://proxy.golang.org/github.com/jonesrussell/indigenous-taxonomy/@v/v1.1.0.info
   ```
3. If either check fails, this WP is blocked. Surface to the maintainer; do NOT proceed until WP04 is fully complete (proxy.golang.org indexing may take a few minutes after the tag is pushed).

**Files**:
- None (verification step).

**Validation**:
- The `proxy.golang.org` URL returns valid JSON with the version metadata.

### T083 — Remove `replace` directive

**Purpose**: Delete the temporary local-path bridge.

**Steps**:
1. Edit `alert-crawler/go.mod`. Remove the line:
   ```
   replace github.com/jonesrussell/indigenous-taxonomy => ../../indigenous-taxonomy
   ```
2. Confirm no other `replace` directives reference the sibling repo.

**Files**:
- `alert-crawler/go.mod` (modified, -1 line).

### T084 — Pin to v1.1.0

**Purpose**: Update the `require` directive to the released version.

**Steps**:
1. In `alert-crawler/go.mod`, find the `require` block and change:
   ```
   require github.com/jonesrussell/indigenous-taxonomy v0.0.0  // or whatever placeholder Phase B used
   ```
   to:
   ```
   require github.com/jonesrussell/indigenous-taxonomy v1.1.0
   ```

**Files**:
- `alert-crawler/go.mod` (modified, ~1 line).

### T085 — `go mod tidy`; CI gate

**Purpose**: Verify the build succeeds without `replace`. CI failures here are the safety net (PR-001).

**Steps**:
1. Run:
   ```bash
   cd alert-crawler && go mod tidy && go build ./... && task test:alert-crawler
   ```
2. Verify `go.sum` is regenerated with the new module hash.
3. Verify all imports resolve. If any test still references package symbols that were renamed in v1.1.0 (shouldn't happen — v1.1.0 is additive — but verify), fix imports.
4. Update Dockerfile (`alert-crawler/Dockerfile`) to remove the `COPY ../indigenous-taxonomy /indigenous-taxonomy` line that was added in WP05. Now Docker pulls the module from the proxy instead.
5. Re-run `docker build -f alert-crawler/Dockerfile -t alert-crawler:dev .` to verify the image builds without the local sibling-repo path.

**Files**:
- `alert-crawler/go.mod` (already)
- `alert-crawler/go.sum` (regenerated)
- `alert-crawler/Dockerfile` (modify to remove sibling-repo COPY)

**Validation**:
- `go build ./...` clean.
- `task test:alert-crawler` clean.
- `docker build` clean.
- `task lint:alert-crawler` clean.
- `task drift:check` clean.

## Definition of Done

- `replace` directive removed.
- `v1.1.0` pinned in `require`.
- `go.sum` updated.
- Dockerfile no longer references the sibling repo path.
- Local build, tests, and Docker image build all pass.
- CI build also passes (the implementer should push a draft PR to verify).

## Risks

- **PR-001**: WP04 not yet tagged. Mitigation: T082 verifies tag availability.
- **Module proxy lag**: rare but possible. Retry T082 after a delay if needed.
- **Dockerfile cleanup**: easy to miss; if left, the image build silently uses an old sibling repo state.

## Reviewer Guidance

- Verify `replace` directive is gone (grep `go.mod` for `replace`).
- Verify `go.sum` reflects the new module hash.
- Verify Dockerfile's build context no longer assumes `../indigenous-taxonomy/`.
- Verify a fresh `go mod download` from a clean GOPATH succeeds.

## Implementation Command

```bash
spec-kitty agent action implement WP19 --agent <name>
```

Depends on WP04 (taxonomy v1.1.0 tag) AND WP18 (Phase B feature-complete).
