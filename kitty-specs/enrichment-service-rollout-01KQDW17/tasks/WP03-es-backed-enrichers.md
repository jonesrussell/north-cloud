---
work_package_id: WP03
title: ES-Backed Enrichers
dependencies:
- WP01
requirement_refs:
- FR-004
- FR-006
- C-002
- C-003
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T013
- T014
- T015
- T016
- T017
- T018
agent: "claude:gpt-5:implementer:implementer"
shell_pid: "4132"
history:
- timestamp: '2026-04-30T00:00:00Z'
  agent: claude
  action: created
  note: Initial WP prompt generated from enrichment rollout mission.
authoritative_surface: enrichment/internal/enricher/
execution_mode: code_change
owned_files:
- enrichment/internal/enricher/**
tags: []
---

# WP03 - ES-Backed Enrichers

## Objective

Implement the `Enricher` interface, registry, and Elasticsearch-backed enrichers for `company_intel`, `tech_stack`, and `hiring`.

Implementation command:

```bash
spec-kitty agent action implement WP03 --mission enrichment-service-rollout-01KQDW17 --agent <name>
```

## Subtasks

### T013: Define Enricher interface and registry

Create an interface with a stable type key and enrichment method. Add a registry that supports known enrichers and returns a clear unknown-type result for orchestration to skip/log.

### T014: Add Elasticsearch client abstraction

Use a narrow interface for ES search calls so unit tests can use fakes. Keep infrastructure imports limited to approved repository patterns.

### T015: Implement company_intel enricher

Search by company name plus optional domain/sector. Produce deterministic confidence scores and graceful empty results when signal quality is low.

### T016: Implement tech_stack enricher

Search existing indexed evidence for technology signals. Return structured data and confidence without inventing unsupported technologies.

### T017: Implement hiring enricher

Search hiring/job evidence relevant to the company/domain/sector. Return structured hiring signals and low-confidence empty results when evidence is absent.

### T018: Add mock-ES tests

Cover happy, empty, and ES-error paths for all three enrichers, plus registry behavior for known and unknown types.

## Definition of Done

- Only `company_intel`, `tech_stack`, and `hiring` enrichers are registered.
- Confidence scoring is deterministic and covered by tests.
- ES errors are surfaced in a way WP04 can isolate per requested type.

## Validation

```bash
cd enrichment
go test ./internal/enricher ./...
```

Document if integration checks requiring real Elasticsearch cannot run locally.

## Reviewer Guidance

Confirm this WP does not add new enrichment types and does not import code from sibling services.

## Activity Log

- 2026-04-30T00:51:23Z – claude:gpt-5:implementer:implementer – shell_pid=4132 – Started implementation via action command
- 2026-04-30T00:55:16Z – claude:gpt-5:implementer:implementer – shell_pid=4132 – Ready for review: registry, ES search abstraction, company_intel/tech_stack/hiring enrichers, deterministic scoring, empty/error behavior, and fake-ES tests implemented in commit fc4c97e8
- 2026-04-30T00:56:15Z – claude:gpt-5:implementer:implementer – shell_pid=4132 – Approved per user request: WP03 reviewed and accepted
