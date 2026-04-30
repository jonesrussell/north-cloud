# Infrastructure Lint Cleanup Data Model

This mission does not introduce runtime data entities. The useful model is the
cleanup inventory that connects linter findings to code surfaces and validation.

## Entities

### LintFinding

- `id`: stable local identifier, usually `<linter>:<file>:<line>`
- `linter`: `mnd`, `gosec`, `gocognit`, `nestif`, `nilnil`, `govet`,
  `exhaustive`, or related golangci-lint checker
- `file`: repository-relative path under `infrastructure/`
- `symbol`: function/type/test name when available
- `category`: constants, profiling config, test package hygiene, security, or
  complexity
- `expected_fix`: behavior-preserving remediation
- `validation`: focused test/lint command that proves the fix

### CleanupSurface

- `path`: infrastructure package or direct caller path
- `owner_wp`: WP01, WP02, WP03, or WP04
- `public_api_changed`: boolean
- `caller_updates`: list of repo paths that must change if the public API moves

### ValidationEvidence

- `command`: exact command run
- `working_directory`: directory where the command ran
- `result`: pass/fail/blocked
- `blocked_reason`: missing tool, module tidy required, or command output
- `captured_at`: timestamp

## Relationships

- A `CleanupSurface` can have many `LintFinding` records.
- Each `LintFinding` belongs to exactly one work package.
- Each work package must produce at least one `ValidationEvidence` record.
- Public API changes in `infrastructure/` must link to all direct
  `caller_updates` in the same work package.
