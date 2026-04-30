# Infrastructure Lint Cleanup Plan

Resolve issue #646 by fixing the existing infrastructure lint findings in small behavior-preserving WPs.

## Architecture

- Keep changes inside `infrastructure/` unless callers must adapt to a config/API cleanup.
- Prefer named constants, smaller tests/functions, explicit config fields, and real security fixes.
- Avoid broad linter suppressions.

## Work Packages

1. WP01: magic-number constants and focused tests.
2. WP02: profiling env/config cleanup and caller updates.
3. WP03: test-package hygiene.
4. WP04: gosec, nilnil, govet, nestif, exhaustive, and gocognit fixes.

## Validation

- `GOWORK=off go test ./...` from `infrastructure/`.
- `GOWORK=off golangci-lint run --config ../.golangci.yml ./...` from `infrastructure/`.
- Pre-commit lint path on a staged infrastructure file.
