---
work_package_id: WP02
title: Callback Client
dependencies:
- WP01
requirement_refs:
- FR-005
- C-001
- C-004
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T008
- T009
- T010
- T011
- T012
agent: "claude:gpt-5:implementer:implementer"
shell_pid: "28104"
history:
- timestamp: '2026-04-30T00:00:00Z'
  agent: claude
  action: created
  note: Initial WP prompt generated from enrichment rollout mission.
authoritative_surface: enrichment/internal/callback/
execution_mode: code_change
owned_files:
- enrichment/internal/callback/**
tags: []
---

# WP02 - Callback Client

## Objective

Implement `enrichment/internal/callback` so enrichment results can be POSTed back to the request-provided Waaseyaa callback URL with the supplied API key.

Implementation command:

```bash
spec-kitty agent action implement WP02 --mission enrichment-service-rollout-01KQDW17 --agent <name>
```

## Subtasks

### T008: Define callback payload types

Create callback request/result payload types that can carry `lead_id`, enrichment type, status, confidence, data, and error information. Reuse API/result contract shapes from WP01 where that avoids duplication without creating cross-service imports.

### T009: Implement CallbackClient

Add `CallbackClient.SendEnrichment(ctx, callbackURL, apiKey, result)` with JSON encoding, `Content-Type: application/json`, and `X-Api-Key` auth. Make HTTP client, timeout, retry count, and backoff injectable or configurable for tests.

### T010: Implement retry policy

Retry network errors and HTTP 5xx responses with context-aware backoff. Do not retry HTTP 4xx responses. Always close response bodies.

### T011: Protect secrets in errors and logs

Never include raw callback API keys in errors, logs, or test failure messages. Redact URL userinfo if a malformed URL includes it.

### T012: Add httptest coverage

Cover success, `X-Api-Key` header, JSON body, 5xx retry success, network/context cancellation behavior, and no retry on 4xx.

## Definition of Done

- Callback client has deterministic retry behavior.
- 4xx callback failures return without retrying.
- Context cancellation stops retries promptly.
- Tests use `httptest` and fast backoff settings.

## Validation

```bash
cd enrichment
go test ./internal/callback ./...
```

Document skipped lint/vuln checks if local tools are unavailable.

## Reviewer Guidance

Confirm this WP does not change Waaseyaa and does not introduce persistent storage for callback secrets or payloads.

## Activity Log

- 2026-04-30T00:37:50Z – claude:gpt-5:implementer:implementer – shell_pid=28104 – Started implementation via action command
- 2026-04-30T00:40:10Z – claude:gpt-5:implementer:implementer – shell_pid=28104 – Ready for review: callback payload/client, X-Api-Key JSON POST, retry/no-retry policy, context-aware backoff, and httptest coverage implemented in commit 900c5aa9
- 2026-04-30T00:49:34Z – claude:gpt-5:implementer:implementer – shell_pid=28104 – Approved per user request: approve all current for_review WPs
- 2026-04-30T12:47:02Z – claude:gpt-5:implementer:implementer – shell_pid=28104 – Mission accepted and merged locally; marking WP done after acceptance repair. | Done override: Spec Kitty merge already merged and deleted the lane branch after gate evidence passed; acceptance commit 11dfb50c records final acceptance.
