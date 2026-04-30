# Infrastructure Lint Cleanup Plan

Resolve issue #646 by fixing the existing infrastructure lint findings in small behavior-preserving WPs.

## Architecture

- Keep changes inside `infrastructure/` unless callers must adapt to a config/API cleanup.
- Prefer named constants, smaller tests/functions, explicit config fields, and real security fixes.
- Avoid broad linter suppressions.

## Current Baseline

- `GOWORK=off go test ./...` from `infrastructure/` currently stops with
  `go: updates to go.mod needed; to update it: go mod tidy`.
- `golangci-lint` is not installed on the local PATH; `.tool-versions` pins
  `golangci-lint 2.10.1`, and the root `install:tools` task installs that
  version.
- Initial hotspot scan found direct profiling env reads in
  `infrastructure/profiling/`, pprof server startup without explicit timeouts,
  SSE broker context/cancel lifecycle code, and likely magic-number duration or
  limit literals in `infrastructure/retry`, `infrastructure/config`, and
  `infrastructure/sse`.

## Work Packages

1. WP01: magic-number constants and focused tests.
2. WP02: profiling env/config cleanup and caller updates.
3. WP03: test-package hygiene.
4. WP04: gosec, nilnil, govet, nestif, exhaustive, and gocognit fixes.

## Validation

- `GOWORK=off go test ./...` from `infrastructure/`.
- `GOWORK=off golangci-lint run --config ../.golangci.yml ./...` from `infrastructure/`.
- Pre-commit lint path on a staged infrastructure file.
