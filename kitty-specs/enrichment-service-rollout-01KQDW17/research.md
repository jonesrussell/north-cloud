# Enrichment Service Rollout Research

## Decisions

### D-001: Implement enrichment as an isolated Go service

Build a new top-level `enrichment/` module with its own `cmd/main.go`, `internal/` packages, `go.mod`, `Taskfile.yml`, and `CLAUDE.md`. This follows the repository convention for independently deployable services and keeps the change local to the issue-backed service boundary.

Evidence: `spec.md` FR-001, C-003; `plan.md` Architecture.

### D-002: Keep the Waaseyaa callback contract external

The service accepts `callback_url` and `callback_api_key` in each request and sends results back to that URL. No Waaseyaa code, schema, or receiver behavior changes are part of this mission.

Evidence: `spec.md` FR-002, FR-005, C-001.

### D-003: Return 202 before enrichment work runs

`POST /api/v1/enrich` validates the payload and returns `202 Accepted`, then runs requested enrichers asynchronously. Unknown enrichment types are logged and skipped without failing known requested types.

Evidence: `spec.md` FR-003, FR-004; `plan.md` Work Packages WP01 and WP04.

### D-004: Split responsibilities by package

Use `internal/handler` for HTTP validation and response behavior, `internal/callback` for authenticated callback delivery, and `internal/enricher` for request/result types, registry, Elasticsearch-backed implementations, and confidence scoring.

Evidence: `plan.md` Architecture.

### D-005: Validate locally with available tools

Go and Task are available in the local environment. `golangci-lint`, Docker, and Ansible are not currently installed, so local validation should run focused Go tests and Task targets where possible, while documenting skipped lint/deployment checks.

Evidence: local tool probe on 2026-04-29.

## Open Questions and Risks

- The exact Elasticsearch index names and field mappings should be inferred from existing service conventions during WP03.
- The callback payload shape must stay compatible with Waaseyaa expectations, but Waaseyaa cannot be modified in this mission.
- Production Ansible work may require a companion change outside this repository, captured explicitly during WP05.
