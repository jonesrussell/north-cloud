# Contract Tests Centralization

## Problem

4 services (crawler, classifier, publisher, search) have `replace ../index-manager` in their `go.mod` files, but zero production code imports index-manager. The only usage is contract tests in `tests/contracts/*_test.go` that import `index-manager/pkg/contracts`.

This causes:
- Every service Dockerfile must `COPY index-manager ./index-manager` for `go mod download` to succeed
- Tight coupling between service modules and index-manager
- Adding a new replace directive silently breaks CI until someone updates the Dockerfile

## Solution

Move all contract test files from individual services into a shared `tests/contracts/` module. This matches the existing pattern used by `tests/integration/pipeline/`.

## Changes

### 1. Create `tests/contracts/go.mod`

New Go module with the index-manager replace directive — the one place that needs it.

### 2. Move test files

| From | To |
|------|----|
| `crawler/tests/contracts/raw_content_producer_test.go` | `tests/contracts/raw_content_producer_test.go` |
| `classifier/tests/contracts/raw_content_consumer_test.go` | `tests/contracts/raw_content_consumer_test.go` |
| `classifier/tests/contracts/classified_content_producer_test.go` | `tests/contracts/classified_content_producer_test.go` |
| `publisher/tests/contracts/classified_content_consumer_test.go` | `tests/contracts/publisher_classified_content_consumer_test.go` |
| `search/tests/contracts/classified_content_consumer_test.go` | `tests/contracts/search_classified_content_consumer_test.go` |

Publisher and search files are renamed to avoid collision (both were `classified_content_consumer_test.go`).

### 3. Clean up 4 service `go.mod` files

Remove `replace github.com/jonesrussell/north-cloud/index-manager => ../index-manager` from:
- `crawler/go.mod`
- `classifier/go.mod`
- `publisher/go.mod`
- `search/go.mod`

Then `go mod tidy` in each to drop the unused require.

### 4. Clean up 4 Dockerfiles

Remove `COPY index-manager ./index-manager` from:
- `crawler/Dockerfile`
- `classifier/Dockerfile`
- `publisher/Dockerfile`
- `search/Dockerfile`

### 5. Delete empty directories

- `crawler/tests/contracts/`
- `classifier/tests/contracts/`
- `publisher/tests/contracts/`
- `search/tests/contracts/`

### 6. Add Taskfile target

Add `test:contracts` to Taskfile so `task test` includes contract tests.

## Verification

1. `cd tests/contracts && go test -v ./...` — all contract tests pass
2. `cd crawler && go mod tidy && golangci-lint run` — no index-manager dependency
3. Same for classifier, publisher, search
4. Docker builds succeed without index-manager in context
