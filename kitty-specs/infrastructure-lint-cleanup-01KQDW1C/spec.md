# Infrastructure Lint Cleanup

**Mission**: `infrastructure-lint-cleanup-01KQDW1C`
**Mission type**: software-dev
**Issue covered**: #646
**Target branch**: `main`

## Goal

Clean up the pre-existing `golangci-lint` violations in `infrastructure/` so ordinary infrastructure edits can pass pre-commit without `--no-verify`.

## Functional Requirements

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-001 | Resolve the baseline magic-number findings by extracting named constants in the affected config, monitoring, health, and retry code without changing behavior. | Required |
| FR-002 | Replace direct profiling-related `os.Getenv` usage with config-driven inputs or explicitly justified exceptions that satisfy the configured linter policy. | Required |
| FR-003 | Convert the affected infrastructure tests to external `_test` packages or expose narrow test seams where needed. | Required |
| FR-004 | Fix the security and correctness findings called out by #646, including pprof server timeouts and the SSE broker context/cancel leak. | Required |
| FR-005 | Split or simplify complex tests/functions enough for `gocognit`, `nestif`, `nilnil`, `govet`, and `exhaustive` to pass under the repo's `.golangci.yml`. | Required |
| FR-006 | Preserve public infrastructure package APIs unless a small signature change is required to remove the env/config smell; any such change must update all repo callers in the same WP. | Required |

## Constraints

| ID | Constraint | Priority |
| --- | --- | --- |
| C-001 | Keep this mission limited to lint cleanup and directly required caller updates; do not refactor unrelated infrastructure behavior. | Required |
| C-002 | Do not suppress findings with broad `nolint` comments. A narrow `nolint` is allowed only with a specific explanation where the linter is demonstrably wrong. | Required |
| C-003 | Preserve existing tests' behavioral intent while moving packages or reducing complexity. | Required |
| C-004 | Treat gosec findings as real until proven otherwise; prefer code fixes over suppressions. | Required |

## Work Package Plan

1. **WP01 constants and low-risk lint**: address `mnd` findings with named constants and rerun focused tests.
2. **WP02 profiling config cleanup**: remove direct env reads in profiling packages, update config structs/loaders and callers, and cover startup behavior.
3. **WP03 test-package hygiene**: move affected tests to external packages and add narrow exported helpers only when necessary.
4. **WP04 security and complexity fixes**: fix gosec, `nilnil`, `govet`, `nestif`, `exhaustive`, and `gocognit` findings with focused behavior-preserving changes.

## Success Criteria

- From `infrastructure/`, `GOWORK=off golangci-lint run --config ../.golangci.yml ./...` reports zero issues.
- `GOWORK=off go test ./...` passes from `infrastructure/`.
- Pre-commit on staged infrastructure edits no longer requires `--no-verify`.
- Issue #646 can be closed with the final lint and test output.

## Out of Scope

- Changing lint policy to make the issue pass.
- Sweeping monorepo-wide lint cleanup outside direct caller updates.
- Reworking infrastructure package architecture beyond what the linter fixes require.
