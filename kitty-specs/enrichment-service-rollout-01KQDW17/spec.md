# Enrichment Service Rollout

**Mission**: `enrichment-service-rollout-01KQDW17`
**Mission type**: software-dev
**Issues covered**: #596, #598, #597, #600
**Target branch**: `main`

## Goal

Build and deploy the `enrichment/` Go service for North Cloud. The service accepts enrichment requests from Waaseyaa, runs the requested enrichers asynchronously, and posts enrichment results back to the callback URL supplied in the request.

## Functional Requirements

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-001 | Add a new top-level `enrichment/` Go module following the repo's service conventions: `cmd/main.go`, `internal/`, `Taskfile.yml`, `go.mod`, and `CLAUDE.md`. | Required |
| FR-002 | Expose `POST /api/v1/enrich` and validate the Waaseyaa request payload: `lead_id`, `company_name`, optional `domain`, optional `sector`, `requested_types`, `signals`, `callback_url`, and `callback_api_key`. | Required |
| FR-003 | Return `202 Accepted` after request validation and run enrichment callbacks asynchronously so one slow enrichment does not block the HTTP response. | Required |
| FR-004 | Define an `Enricher` interface and registry for `company_intel`, `tech_stack`, and `hiring`. Unknown requested types must be logged and skipped without failing known requested types. | Required |
| FR-005 | Implement `enrichment/internal/callback` with `CallbackClient.SendEnrichment(ctx, callbackURL, apiKey, result)` using `X-Api-Key`, JSON, retry on 5xx/network errors, no retry on 4xx, and context-aware backoff. | Required |
| FR-006 | Implement `company_intel`, `tech_stack`, and `hiring` enrichers against Elasticsearch with deterministic confidence scoring and graceful low-confidence empty results. | Required |
| FR-007 | Add `GET /health` returning HTTP 200 with a small JSON status object. | Required |
| FR-008 | Add build/test/lint/vuln Taskfile integration at service and root levels. | Required |
| FR-009 | Add deploy artifacts and Ansible support for a systemd service on port 8095 with journald logs, restart-on-failure, env templating, and health-check documentation. | Required |

## Constraints

| ID | Constraint | Priority |
| --- | --- | --- |
| C-001 | Do not modify Waaseyaa. Treat callback URL, callback API key, and response semantics as an external contract. | Required |
| C-002 | Do not add new enrichment types beyond `company_intel`, `tech_stack`, and `hiring`. | Required |
| C-003 | Keep shared code imports limited to `infrastructure/` and standard approved service patterns. | Required |
| C-004 | Do not store callback API keys or enrichment payloads outside normal request-local logs; logs must not print raw secrets. | Required |
| C-005 | Deployment changes that belong in `northcloud-ansible` must be captured as companion changes or a clearly named follow-up PR, not hidden in this repo only. | Required |

## Work Package Plan

1. **WP01 service scaffold and API contract**: create the service module, config loading, HTTP server, request/response structs, validation, `/health`, and root Taskfile hooks.
2. **WP02 callback client**: implement callback payload shape, retry/backoff, auth header, timeout handling, logging, and unit tests with `httptest`.
3. **WP03 enrichers**: implement the `Enricher` interface, registry, ES query builders, three enrichers, confidence scoring, and mock-ES tests.
4. **WP04 integration loop**: wire handler to registry and callback client, run each requested enrichment independently, and cover async/error isolation behavior.
5. **WP05 production deployment**: add deploy docs/artifacts plus `northcloud-ansible` systemd/env/Caddy or direct-port wiring as needed.

## Success Criteria

- `task enrichment:build`, `task enrichment:test`, and service-level lint pass in the available toolchain.
- Tests cover validation, immediate `202`, callback retry policy, each enricher's happy/empty/error paths, and unknown requested types.
- Ansible deployment templates render with required env values and the service can be enabled as a restartable systemd unit.
- Issues #596, #597, #598, and #600 can be closed with links to the mission merge and verification evidence.

## Out of Scope

- Waaseyaa schema or receiver changes.
- Dead-letter queues for failed callbacks.
- New enrichment types beyond the three issue-backed types.
- Replacing `signal-crawler` or `signal-producer`.
