<!--
SYNC IMPACT REPORT
==================
Version change: (uninitialized template) → 1.0.0
Bump rationale: Initial ratification of the project constitution. All template placeholders replaced with concrete values derived from existing CLAUDE.md, ARCHITECTURE.md, and docs/specs/.

Principles defined (7):
  I.   Spec-Driven Development with Drift Detection
  II.  Service Independence & Architectural Boundaries
  III. Strict Static Analysis (NON-NEGOTIABLE)
  IV.  Test Discipline & Coverage
  V.   Structured Observability
  VI.  Contract & Configuration as Single Source of Truth
  VII. Conventional Workflow & Branch Hygiene

Added sections:
  - Core Principles (7 principles)
  - Technical Constraints
  - Development Workflow & Quality Gates
  - Governance

Removed sections: none (initial version)

Templates requiring updates:
  ✅ .specify/templates/plan-template.md — "Constitution Check" gate references this file (no changes needed; gates resolve at plan time)
  ✅ .specify/templates/spec-template.md — no constitution-specific changes required
  ✅ .specify/templates/tasks-template.md — no constitution-specific changes required
  ✅ .specify/templates/checklist-template.md — no constitution-specific changes required

Deferred / TODO:
  - TODO(RATIFICATION_DATE): Original adoption date unknown; project predates Spec Kit installation. Set when stakeholders confirm.
-->

# North Cloud Constitution

## Core Principles

### I. Spec-Driven Development with Drift Detection

Every service has a spec under `docs/specs/`. Code and spec evolve together: a behavior change to a service MUST be accompanied by an update to its spec in the same PR. Drift is enforced mechanically — `task drift:check` runs in pre-push, in CI, and as the first step of `task ci`/`task ci:changed`/`task ci:force`. A stale or missing spec MUST fail the build.

**Rationale**: The codified context (CLAUDE.md hierarchy + `docs/specs/`) is the root of trust for multi-repo orchestration, cross-project audits, and the drift governor. If specs drift from implementation, every downstream system (AI agents, runbooks, audits) becomes probabilistic.

### II. Service Independence & Architectural Boundaries

Services MUST NOT import from other services. The only shared code is `infrastructure/`. Layer boundaries within a service are enforced by `task layers:check` (per-service `.layers` file). North Cloud owns the **content pipeline only** — crawling, classification, enrichment, routing, Redis pub/sub, and the source registry. NC MUST NOT own or reference: entity model (Minoo), framework internals or ingestion envelope (Waaseyaa), editorial decisions (consuming apps). Shared contracts go through `jonesrussell/indigenous-taxonomy` and the documented Redis channel naming.

**Rationale**: Service independence keeps the blast radius of changes small and enables per-service deploys. Architectural boundaries with Minoo/Waaseyaa prevent NC from absorbing concerns it shouldn't own and keep the platform composable.

### III. Strict Static Analysis (NON-NEGOTIABLE)

All Go code MUST pass `golangci-lint` with the repository's `.golangci.yml`. Specifically:

- Use `any` not `interface{}`.
- All JSON marshal/unmarshal errors MUST be checked.
- No magic numbers — define named constants.
- Pre-allocate slices with capacity hints (`make([]T, 0, len(src))`).
- All test helpers MUST call `t.Helper()`.
- Cognitive complexity ≤ 20 (`gocognit`); function length ≤ 100 lines (`funlen`); line length ≤ 150 chars.
- No variable shadowing.
- **No `os.Getenv`** — use `infrastructure/config` (`forbidigo` enforced; only exceptions are `cmd/` and `infrastructure/config/`).

`task lint:force` MUST be run before pushing changes. CI runs the same lint with cache disabled.

**Rationale**: A consistent, mechanically enforced bar removes whole classes of defects and keeps reviews focused on design rather than style.

### IV. Test Discipline & Coverage

All Go services target ≥ 80% test coverage. Every test helper MUST call `t.Helper()`. Database tests MUST use context-aware methods (`PingContext`, `QueryContext`, etc.). Integration tests MUST cover: new library contract surfaces, contract changes, inter-service communication, and shared schemas. Pre-push runs `go-test` for changed services via lefthook.

**Rationale**: Coverage alone is necessary but not sufficient — the helper, context, and integration rules close the gaps where coverage metrics typically lie.

### V. Structured Observability

All services MUST log via `infrastructure/logger` (import alias `infralogger`). Logs MUST be JSON, with structured fields in `snake_case`. Direct use of `log` or `fmt.Println` for service-level events is prohibited. Health endpoints (`/health`) MUST exist for every HTTP service.

**Rationale**: Uniform structured logging is a precondition for the drift governor, the AI observer, Loki/Grafana dashboards, and incident response. Free-form text logs break all of those.

### VI. Contract & Configuration as Single Source of Truth

- **Ports & env**: `task ports:check` enforces compose ↔ docs parity. After compose changes, run `task ports:generate`. The generated `docs/generated/ports-and-env.md` is the SSOT for service ports.
- **Env names**: Compose env var names MUST exactly match the `env:` struct tag in the service config struct. Mismatches are silent — verify with grep before merge.
- **Migrations**: Migration prefixes MUST be unique within a service (`golang-migrate` crashes on duplicates). Deploy validates this; PRs MUST too.
- **ES mappings**: `content_type` MUST be `text` (not `keyword`); search queries the `.keyword` sub-field.

**Rationale**: Silent drift between code, compose, and docs is the most common production-only failure mode. A few mechanical gates eliminate it.

### VII. Conventional Workflow & Branch Hygiene

- **Branch naming**: All Claude-driven branches MUST be `claude/{description}-{session-id}`. Never force-push to `main`.
- **Commits**: Conventional commits — `type(scope): description` (`feat`/`fix`/`chore`/`docs`/`refactor`/`test`/`ci`/`perf`).
- **PR template** enforces: linked issue, milestone, lint status, test status, spec updates, layer-check.
- **Stacked PRs**: Retarget dependents to `main` before merging the parent (see `docs/specs/workflow.md` Rule 6).
- **Issues**: Every issue gets a milestone; PRs close issues with `Closes #NNN`.
- **Hooks**: Lefthook is mandatory; `--no-verify` is reserved for documented emergencies and MUST be flagged in the PR description.

**Rationale**: Predictable Git history makes auditing, cherry-picks, and rollbacks tractable. The branch-naming convention is what lets multi-agent orchestration disambiguate sessions.

## Technical Constraints

- **Go version**: 1.26+ across all services. `GOWORK=off` per service; `go.work` is IDE-only.
- **Frontend**: Vue 3 Composition API + TypeScript + Vite. No `any` — use `unknown` or specific interfaces. Types live under `dashboard/src/types/`.
- **Docker**: Always `docker compose` (not `docker-compose`). Dev: base + dev overlay. Prod: base + prod overlay. Container names follow `north-cloud-{service}`.
- **Production target**: `/home/deployer/north-cloud` is **not a git repo** — it is an image-based deploy target managed via GitHub Actions. Manual `git pull` on prod is forbidden.
- **Security & secrets**: `.env` files MUST be gitignored. Secrets MUST flow via deployment secrets, not committed config. Self-signed nginx certs are gitignored — they MUST be regenerated per `infrastructure/nginx/certs/README.md` on a new host.
- **Bootstrap pattern**: HTTP services follow the documented phase ordering: Profiling → Config → Logger → Database → Services → Server → Lifecycle. See `ARCHITECTURE.md`.

## Development Workflow & Quality Gates

**Before committing**, the following MUST pass locally (lefthook enforces most automatically):

1. `task drift:check` — affected specs are current.
2. `task lint:force` — matches CI exactly.
3. `task test` (or per-service `task test:SERVICE`) — for changed services.
4. `task layers:check` — no cross-layer imports.
5. `task ports:check` — when compose was modified.

**CI pipeline order** (`task ci`, `task ci:changed`, `task ci:force`):

1. `task drift:check`
2. `task ports:check`
3. lint
4. test
5. layer-check

**PR checklist** (enforced by template): issue link, milestone, lint pass, test pass, spec updated (or `N/A` justified), no unexplained `--no-verify`.

**Code review focus areas**: principle compliance (this constitution), service-boundary respect, observability hygiene, migration safety, env-var/struct-tag parity.

## Governance

This constitution supersedes informal practices. Where this constitution conflicts with `CLAUDE.md`, `ARCHITECTURE.md`, or service-level docs, **this constitution wins** and the conflicting doc MUST be reconciled in the same PR that surfaces the conflict.

**Amendment procedure**:

1. Open a PR that modifies `.specify/memory/constitution.md`.
2. Update the version line and prepend a Sync Impact Report (HTML comment at top of file).
3. Propagate changes to dependent templates (`plan-template.md`, `spec-template.md`, `tasks-template.md`, `checklist-template.md`) in the same PR.
4. Update agent guidance (`CLAUDE.md`, `ARCHITECTURE.md`) where principles are referenced.
5. PR title MUST be `docs: amend constitution to vX.Y.Z (...)`.

**Versioning policy** (semver):

- **MAJOR**: Backward-incompatible governance changes; principle removal or redefinition.
- **MINOR**: New principle or section, materially expanded guidance.
- **PATCH**: Clarifications, wording, typo fixes, non-semantic refinements.

**Compliance review**:

- Every PR review MUST verify constitution compliance for the changed surface.
- Quarterly: maintainers review the constitution against operational reality (incidents, near-misses, drift findings) and amend.
- Complexity that violates a principle MUST be justified in writing in the PR description, with a follow-up issue to remediate.

For runtime guidance, refer to `CLAUDE.md` (orchestration map, common operations, troubleshooting) and per-service `CLAUDE.md` files.

**Version**: 1.0.0 | **Ratified**: TODO(RATIFICATION_DATE): set when stakeholders confirm original adoption date | **Last Amended**: 2026-04-27
