# Enrichment Service Rollout Plan

Build the `enrichment/` service as a conventional isolated Go service, then deploy it as a host-managed systemd service through the north-cloud Ansible role.

## Architecture

- `enrichment/cmd/main.go` loads config, constructs logger, ES client, callback client, enricher registry, and HTTP handler.
- `internal/handler` owns request validation, `202 Accepted`, and async fan-out.
- `internal/callback` owns Waaseyaa callback POSTs, auth, retry, and timeout behavior.
- `internal/enricher` owns the shared request/result types, registry, ES query helpers, and the three issue-backed enrichers.
- Deployment uses systemd/journald and a vault-rendered env file; secrets are never written into repo templates.

## Work Packages

1. WP01: scaffold service, config, `/health`, `POST /api/v1/enrich`, validation, and Taskfile hooks.
2. WP02: callback client with retries, no-retry 4xx behavior, API-key header, and tests.
3. WP03: enrichers and registry with mock ES tests.
4. WP04: handler-to-enricher/callback integration and async/error isolation tests.
5. WP05: deploy artifacts and `northcloud-ansible` support.

## Validation

- Run service build/test/lint where tools are installed.
- Run focused `go test` for handler, callback, and enricher packages after each WP.
- Render Ansible templates or run syntax checks in an environment with Ansible installed.
