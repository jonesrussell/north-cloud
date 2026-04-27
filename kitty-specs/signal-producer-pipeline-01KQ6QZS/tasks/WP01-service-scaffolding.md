---
work_package_id: WP01
title: Service Scaffolding
dependencies: []
requirement_refs:
- FR-020
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-signal-producer-pipeline-01KQ6QZS
base_commit: d8e2276ba5c1964c6e5031af06e454d6d53ca9f6
created_at: '2026-04-27T06:22:41.713171+00:00'
subtasks:
- T001
- T002
- T003
- T004
- T005
shell_pid: "89496"
agent: "claude:opus-4.7:implementer:implementer"
history:
- event: created
  at: '2026-04-27T05:55:00Z'
  by: /spec-kitty.tasks
authoritative_surface: signal-producer/
execution_mode: code_change
mission_id: 01KQ6QZSNBQF3VW515AKM0SQ46
mission_slug: signal-producer-pipeline-01KQ6QZS
owned_files:
- signal-producer/go.mod
- signal-producer/go.sum
- signal-producer/Taskfile.yml
- signal-producer/config.yml.example
- signal-producer/CLAUDE.md
- signal-producer/.layers
- signal-producer/.gitignore
- Taskfile.yml
- scripts/detect-changed-services.sh
- docs/specs/lead-pipeline.md
tags: []
---

# WP01 — Service Scaffolding

## Objective

Lay the `signal-producer/` service skeleton so the three leaf-package WPs (WP02 checkpoint, WP03 mapper, WP04 client) can land in parallel without colliding. The deliverable is a buildable, lintable, drift-clean Go module with all CI hooks wired in, but no business logic yet.

## Context

Read first:
- [spec.md](../spec.md) FR-020 (Taskfile targets), C-002/C-004 (lint, config), C-009 (drift + layers + GO_SERVICES + Taskfile delegations), C-007 (no docker-compose).
- [plan.md](../plan.md) "Technical Context" and the "Execution Approach" diagram.
- [research.md](../research.md) D1 (ES client), D4 (simple `cmd/main.go`).
- Root `CLAUDE.md` "New Go service checklist" — three things must land together: per-service `vuln:` task, root delegation, `GO_SERVICES` entry. Missing any breaks CI.
- An existing simple Go service to mirror patterns: look at `auth/` and `search/` for the cmd/main.go shape and Taskfile structure.

Charter constraints:
- C-002: Go 1.26+; golangci-lint clean; no `os.Getenv` outside `cmd/` and `infrastructure/config/`.
- C-009: drift + layers + ports checks pass.

## Branch Strategy

Planning base: `main`. Merge target: `main`. Lane workspace allocated by `lanes.json`. WP01 has no dependencies and is the foundation; subsequent WPs branch from this WP's lane.

## Subtask Guidance

### T001 — Create `signal-producer/` directory tree

**Purpose**: Stand up the service module with all the conventional files an empty Go service needs.

**Steps**:

1. Create `signal-producer/` at the repo root with this layout:
   ```
   signal-producer/
   ├── cmd/                        (empty for now; WP05 owns cmd/main.go)
   ├── internal/                   (empty; WP02-WP04 will populate)
   ├── go.mod
   ├── Taskfile.yml
   ├── config.yml.example
   ├── CLAUDE.md
   ├── .layers
   └── .gitignore
   ```

2. `signal-producer/go.mod`:
   ```
   module github.com/jonesrussell/north-cloud/signal-producer

   go 1.26
   ```
   No dependencies in WP01; `go.sum` will not exist yet (created when WP02–WP04 add deps).

3. `signal-producer/Taskfile.yml` — mirror the structure used by `auth/Taskfile.yml`. Required tasks: `build`, `test`, `test:cover`, `lint`, `vuln`. The `build` task targets `./cmd/...` even though it's empty — `go build` on an empty directory is a no-op success.

4. `signal-producer/config.yml.example` — concrete example matching the Config shape from `data-model.md`. Use placeholder env-var refs:
   ```yaml
   waaseyaa:
     url: "${WAASEYAA_URL}"
     api_key: "${WAASEYAA_API_KEY}"
     batch_size: 50
     min_quality_score: 40
   elasticsearch:
     url: "${ES_URL}"
     indexes: ["*_classified_content"]
   schedule:
     lookback_buffer: "5m"
   checkpoint:
     file: "/var/lib/signal-producer/checkpoint.json"
   ```

5. `signal-producer/CLAUDE.md` — short, ≤ 60 lines. Explain the service's role, the three internal packages, how to run locally, and the deployment posture (systemd one-shot). Reference `kitty-specs/signal-producer-pipeline-01KQ6QZS/spec.md` and the runbook entry that WP06 will add.

6. `signal-producer/.layers` — define layer mappings for the three internal packages plus `cmd`. Use the same format as `classifier/.layers` or whichever existing service has the cleanest example. Conservative default:
   ```
   cmd                : 4
   internal/producer  : 3
   internal/mapper    : 2
   internal/client    : 2
   ```
   Adjust if existing services use a different convention. The point: `cmd` imports `producer`; `producer` imports `mapper` and `client`; `mapper` and `client` are independent leaves.

7. `signal-producer/.gitignore`:
   ```
   /signal-producer
   /coverage.out
   ```

**Files**: `signal-producer/{go.mod, Taskfile.yml, config.yml.example, CLAUDE.md, .layers, .gitignore}`.

**Validation**:

- [ ] `cd signal-producer && go build ./...` exits 0 (no Go files, no error).
- [ ] `task lint:signal-producer` exits 0.
- [ ] Layout matches the diagram above.

### T002 — Root `Taskfile.yml` delegations

**Purpose**: Make `task build:signal-producer`, `task test:signal-producer`, `task lint:signal-producer`, and `task vuln:signal-producer` work from the repo root.

**Steps**:

1. Open `Taskfile.yml` at the repo root.
2. Find the existing per-service delegation pattern (look for `build:auth`, `test:auth`, `lint:auth`).
3. Add the same four entries for `signal-producer` immediately after `auth` or in alphabetical position, whichever the file conventions prefer.
4. Verify the aggregate tasks (`task build`, `task test`, `task lint`) include the new service automatically (most use `for: [services]` lists — append `signal-producer`).

**Files**: `Taskfile.yml` (root).

**Validation**:

- [ ] `task build:signal-producer` runs and exits 0.
- [ ] `task lint:signal-producer` runs and exits 0.
- [ ] `task vuln:signal-producer` runs and exits 0 (or returns vuln output cleanly).
- [ ] `task test` does not regress (other services still run).

### T003 — Update `scripts/detect-changed-services.sh`

**Purpose**: Make CI's changed-services detection aware of `signal-producer/` so PRs only run signal-producer pipelines when its files change.

**Steps**:

1. Open `scripts/detect-changed-services.sh`.
2. Find the `GO_SERVICES` array.
3. Add `"signal-producer"` in alphabetical position.
4. Skim the rest of the script for any hard-coded service lists; update them too. (Some scripts repeat the list under different names — search for `"auth"` and add the new entry next to every occurrence.)

**Files**: `scripts/detect-changed-services.sh`.

**Validation**:

- [ ] `grep -n signal-producer scripts/detect-changed-services.sh` shows the entry in `GO_SERVICES`.
- [ ] Bash linter (`shellcheck scripts/detect-changed-services.sh`) is no worse than before.
- [ ] Run `bash scripts/detect-changed-services.sh` against a fixture if the script supports it; ensure it doesn't crash.

### T004 — `docs/specs/lead-pipeline.md` section

**Purpose**: Add a brief authoritative section on the producer so `task drift:check` accepts subsequent WP commits under `signal-producer/`.

**Steps**:

1. Open `docs/specs/lead-pipeline.md`.
2. Add a new section titled `## Signal Producer` (or fit the existing heading hierarchy). 30–60 lines.
3. Cover: scope (one paragraph), the three internal packages and their responsibilities, the systemd one-shot posture, the Waaseyaa contract dependency (link to `contracts/signals-post.yaml`), and the explicit out-of-scope list (Waaseyaa receiver, enrichment service, signal-crawler).
4. Link to the spec at `kitty-specs/signal-producer-pipeline-01KQ6QZS/spec.md` for full detail.
5. Re-run `task drift:check` to confirm the spec accepts code changes under `signal-producer/`.

**Files**: `docs/specs/lead-pipeline.md`.

**Validation**:

- [ ] Section exists, is well-formed markdown, ≤ 60 lines.
- [ ] `task drift:check` (run after WP02–WP06 land code) won't flag this service.

### T005 — Verify all gates green

**Purpose**: Final smoke check that WP01 left the repo in a CI-clean state for downstream WPs.

**Steps**:

1. From repo root:
   ```bash
   task lint:signal-producer
   task drift:check
   task layers:check
   task ports:check
   ```
2. All four exit 0.
3. Run `task lint:force` to match CI's cache-clean run.
4. Run `git status` — no unintentional changes outside the WP01 file list.

**Files**: none new; this is a verification step.

**Validation**:

- [ ] All four checks green.
- [ ] `git status` shows only files in this WP's `owned_files`.

## Definition of Done

- [ ] All five subtasks complete with their validation checklists ticked.
- [ ] `task lint:signal-producer`, `task drift:check`, `task layers:check`, `task ports:check`, `task lint:force` all green.
- [ ] No files modified outside this WP's `owned_files`.
- [ ] Conventional commit message; lefthook hooks pass.

## Reviewer Guidance

When reviewing this WP, focus on:

1. **Did the diff respect ownership?** `git diff --stat` should show files only inside `signal-producer/`, plus the three explicit edits to root `Taskfile.yml`, `scripts/detect-changed-services.sh`, and `docs/specs/lead-pipeline.md`. Anything else is out of scope.
2. **CI-aware**: `task lint:signal-producer` must work from a fresh clone. Run it.
3. **Drift acceptance**: `task drift:check` should accept this state. Cross-check the new spec section actually mentions the producer.
4. **Layer file is sane**: `.layers` defines a downward-only DAG (cmd → producer → mapper, client). Reject if cmd appears as a leaf or if mapper/client import each other.
5. **No `os.Getenv` in any of the WP01 files** (none should have any Go code yet, but check).
6. **Empty cmd/ and internal/**: WP01 must NOT add `cmd/main.go` or files under `internal/*`. Those belong to WP05 / WP02-WP04 respectively.

## Risks and Mitigations

| Risk                                                                          | Mitigation                                                                                                               |
| ----------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| Adding to `Taskfile.yml` clashes with another in-flight mission's edits.      | Reviewer compares against latest `main`; conflicts are routine git merges, not WP rework.                                |
| `.layers` mappings disagree with existing service conventions.                | Read `auth/.layers` and `classifier/.layers` first; match style.                                                         |
| `lead-pipeline.md` section grows unbounded.                                  | 30–60 line cap; link out to the spec for detail.                                                                        |

## Implementation Command

```bash
spec-kitty agent action implement WP01 --agent <agent-name> --mission signal-producer-pipeline-01KQ6QZS
```

## Activity Log

- 2026-04-27T06:22:43Z – claude:opus-4.7:implementer:implementer – shell_pid=89496 – Assigned agent via action command
