# Enrichment Service Rollout Tasks

## WP01 — Service Scaffold and API Contract

Create the `enrichment/` module, config, HTTP server, `/health`, `POST /api/v1/enrich`, request validation, root Taskfile hooks, and service `CLAUDE.md`.

## WP02 — Callback Client

Implement `enrichment/internal/callback` with JSON POST, `X-Api-Key`, retry on 5xx/network errors, no retry on 4xx, context-aware backoff, timeout handling, and `httptest` coverage.

## WP03 — ES-Backed Enrichers

Implement `company_intel`, `tech_stack`, and `hiring` enrichers plus the registry, deterministic confidence scoring, empty-result behavior, and mock ES tests.

## WP04 — Async Enrichment Orchestration

Wire handler, registry, and callback client so known requested types run independently after the `202` response; unknown types are logged and skipped.

## WP05 — Production Deployment

Add deploy docs/artifacts and `northcloud-ansible` role changes for env templating, systemd service, restart policy, journald logs, and health verification.
