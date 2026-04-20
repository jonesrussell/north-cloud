# CLAUDE.md

This file provides quick-reference guidance for Claude Code when working in this repository.
For deep architecture documentation, see `ARCHITECTURE.md`.

---

## Orchestration — Where to Look

When modifying files, read the relevant service CLAUDE.md first. Deep specs in `docs/specs/`.

| File pattern | Service context | Spec |
|---|---|---|
| `crawler/**` | `crawler/CLAUDE.md` | `docs/specs/content-acquisition.md` |
| `classifier/**`, `ml-sidecars/**` | `classifier/CLAUDE.md` | `docs/specs/classification.md` |
| `publisher/**` | `publisher/CLAUDE.md` | `docs/specs/content-routing.md` |
| `search/**`, `index-manager/**` | `search/CLAUDE.md`, `index-manager/CLAUDE.md` | `docs/specs/discovery-querying.md` |
| `infrastructure/**` | — | `docs/specs/shared-infrastructure.md` |
| `source-manager/**` | `source-manager/CLAUDE.md` | `docs/specs/source-manager.md` |
| `dashboard/**` | `dashboard/CLAUDE.md` | `docs/specs/dashboard.md` |
| `pipeline/**` | `pipeline/CLAUDE.md` | `docs/specs/pipeline.md` |
| `social-publisher/**` | `social-publisher/CLAUDE.md` | `docs/specs/social-publisher.md` |
| `rfp-ingestor/**` | `rfp-ingestor/CLAUDE.md` | `docs/specs/rfp-ingestor.md` |
| `mcp-north-cloud/**` | `mcp-north-cloud/CLAUDE.md` | `docs/specs/mcp-server.md` |
| `ai-observer/**` | `ai-observer/CLAUDE.md` | `docs/specs/ai-observer.md` |
| `auth/**` | `auth/CLAUDE.md` | `docs/specs/auth.md` |
| `click-tracker/**` | `click-tracker/CLAUDE.md` | `docs/specs/click-tracker.md` |
| `nc-http-proxy/**` | `nc-http-proxy/CLAUDE.md` | `docs/specs/nc-http-proxy.md` |
| `search-frontend/**` | `search-frontend/CLAUDE.md` | `docs/specs/search-frontend.md` |
| `render-worker/**` | `render-worker/CLAUDE.md` | `docs/specs/content-acquisition.md` |
| `signal-crawler/**` | `signal-crawler/CLAUDE.md` | `docs/specs/lead-pipeline.md` (sprint design: `docs/superpowers/specs/2026-04-05-signal-crawler-sprint-design.md`) |
| `classifier/**/need_signal*`, `publisher/**/claudriel_lead*`, `publisher/**/leads_export*`, `infrastructure/signal/**` | — | `docs/specs/lead-pipeline.md` |
| `docs/specs/**`, `.claude/**`, `**/CLAUDE.md` | updating-codified-context | — |
| `docker-compose*.yml`, `Taskfile.yml` | `DOCKER.md` | — |

---

## Content Pipeline Layers

```
Sources → [Crawler] → ES raw_content → [Classifier + ML Sidecars] → ES classified_content
  → [Publisher Router] → Redis channels → [Consumers: Streetcode, Social Publisher]
```

**Publisher routing** (11 layers, evaluated in order):
- L1: Topic auto-detect (skips: mining, indigenous, coforge, recipe, jobs, rfp)
- L2: DB Channels | L3: Crime | L4: Location | L5: Mining | L6: Entertainment | L7: Indigenous | L8: CoForge | L9: Recipe | L10: Job | L11: RFP

**Drift Governor** (ai-observer, 6h ticker): KL divergence + PSI against 7-day baseline → LLM analysis → GitHub issue + draft PR. Config: `AI_OBSERVER_DRIFT_ENABLED`.

**RFP ingestor** (bypasses classifier): Polls CanadaBuys CSV → bulk-indexes to `rfp_classified_content`. Uses `*_classified_content` pattern so search wildcard picks it up.

**Dependency rule**: Services import only from `infrastructure/`. No cross-service imports.

---

## Common Operations

**Add a new source**: See `source-manager/CLAUDE.md` for API details. Flow: source-manager API → crawler picks up on next schedule → classifier processes → publisher routes.

**Add a new ML sidecar**: Create module in `ml-modules/{name}/` → add `{NAME}_ENABLED` + `{NAME}_ML_SERVICE_URL` to classifier config → add routing layer in publisher → update docker-compose. See `classifier/CLAUDE.md`.

**Add a new publisher channel**: Create channel via publisher API with topic rules → content matching rules routes to Redis channel → consumers subscribe.

**Modify ES mappings**: Update `classifier/internal/elasticsearch/mappings/` → reindex via index-manager. **Note**: `content_type` must be `text` type (not `keyword`) — search queries `content_type.keyword` sub-field.

**Add env vars to compose**: Names must exactly match the `env:` struct tag in the service's config struct. Mismatches are silent — the var is set but never read. Verify with: `grep -r 'VAR_NAME' SERVICE/internal/config/`

**Add a migration**: Create up/down SQL in `{service}/internal/database/migrations/` → run `task migrate:SERVICE` → test with `task test:SERVICE`. **Check for duplicate prefixes**: `ls {service}/migrations/ | cut -d_ -f1 | sort | uniq -d` — golang-migrate crashes on duplicates.

---

## Quick Reference

### Most Common Commands

**Docker**: `task docker:dev:up` (core), `task docker:dev:up:ml` (+ML sidecars), `task docker:dev:up:search` (+search), `task docker:dev:up:observability` (+Loki/Grafana), `task docker:dev:up:full` (everything). Logs: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f SERVICE`. Rebuild: `... up -d --build SERVICE`. Stop: `... down`.

**Dev Postgres**: Single shared instance (7 DBs). Auto-creates via `infrastructure/postgres/init-dev.sql` on first startup. Re-init: `... down -v`.

**Taskfile (Preferred)**: `task lint`, `task test`, `task test:cover` (all services). Per-service: `task lint:SERVICE`, `task test:SERVICE`. Migrations: `task migrate:up`, `task migrate:SERVICE`. Tools: `task install:tools`. Use `task lint:force` before pushing (cache-clean, matches CI). Changed-only: `task lint:changed`, `task ci:changed`. Spec drift: `task drift:check` checks for stale specs vs recent service changes.

**Spec Drift**: `task drift:check` (checks last 5 commits). Runs automatically as first step of `task ci`, `task ci:changed`, `task ci:force`. Also runs in lefthook pre-push and CI. Fails if any spec is stale or missing.

**Layer Check**: `task layers:check` verifies internal package imports respect layer boundaries. Each service has a `.layers` file defining package→layer mappings. Runs in `task ci`, `task ci:changed`, `task ci:force`, and lefthook pre-push. Fails if any package imports from a higher layer. Use `allow SOURCE TARGET` in `.layers` to track known violations.

**Go Workspace**: `GOWORK=off` per service. `go.work` is IDE-only. After dep changes: `task vendor`.

**Worktree CI**: `task ci` fails in worktrees (missing Node deps for dashboard). Use `task ci:changed` for Go-only work.

---

## Service Ports

auth:8040 | source-manager:8050 | crawler:8080 | publisher:8070 | classifier:8070 | pipeline:8075 | nc-http-proxy:8055 | index-manager:8090 | search:8092(dev)/8090(prod) | click-tracker:8093 | rfp-ingestor:8095 | ai-observer:8096 | dashboard:3002 | render-worker:3000. ML sidecars under `ml-sidecars/`: mining-ml:8077, indigenous-ml:8081.

---

## CRITICAL Rules - YOU MUST Follow

### Before Making Changes

1. **Read first**: Service README.md or CLAUDE.md, files you will modify, existing patterns
2. **Check dependencies**: docker-compose `depends_on`, API integrations, database schemas
3. **Plan multi-service changes**: Identify affected services, determine change order
4. **Understand service boundaries**: Each service is independent with its own database
5. **Check spec drift**: Run `task drift:check` — if a spec is STALE, update it before or alongside your code changes

### Linting Prevention - CRITICAL

**The linter flags all violations as errors. Key rules:**

- Use `any` not `interface{}` | Check all JSON marshal/unmarshal errors | Define named constants (no magic numbers)
- Pre-allocate slices: `make([]T, 0, len(src))` | All test helpers start with `t.Helper()`
- Cognitive complexity <= 20 (`gocognit`) | Function length <= 100 lines (`funlen`) — extract helpers
- Lines under 150 chars | No variable shadowing (use `unmarshalErr`, `marshalErr`, etc.)
- **No `os.Getenv`** — use `infrastructure/config` (`forbidigo` enforced; exception: `cmd/`, `infrastructure/config/`)

**Before committing**: `cd SERVICE && golangci-lint run` (or rely on lefthook pre-commit hook)

### Git Hooks (lefthook)

Pre-commit hooks run automatically via [lefthook](https://github.com/evilmartians/lefthook). Config: `lefthook.yml`.

- **pre-commit**: `go-fmt` (auto-fix), `go-lint` (golangci-lint), `dashboard-lint` — only changed services
- **pre-push**: `go-test` (only changed services), `spec-drift` (drift-detector check), `layer-check` (layer boundary check)
- **Install**: `go install github.com/evilmartians/lefthook@latest && lefthook install`
- **Skip (emergency)**: `git commit --no-verify`

---

## Code Conventions

### Go Services

- **Go Version**: 1.26+ (all services)
- **Standards**: `gofmt`, `goimports`, standard Go formatting
- **Error Handling**: Always wrap errors: `fmt.Errorf("context: %w", err)`
- **Logging**: All services use `infrastructure/logger` package directly
  - Import: `infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"`
  - Always JSON format, structured fields with snake_case
  - Example: `log.Info("Service started", infralogger.String("service", "crawler"))`
- **Database**: Always use context-aware methods (`PingContext()`, `QueryContext()`, etc.)
- **Testing**: Target 80%+ coverage, all helper functions use `t.Helper()`

### Frontend (Vue.js 3)

- **Framework**: Vue 3 Composition API + TypeScript
- **Build**: Vite
- **Type Safety**: No `any` types — use `unknown` for generic values, specific interfaces for known types
- **Types**: Defined in `/dashboard/src/types/` directory

---

## Bootstrap Pattern

All HTTP services follow a consistent bootstrap pattern: simple services (auth, search) use helper
functions in `main.go`; complex services (crawler, classifier, source-manager) use an
`internal/bootstrap/` package with phased modules (config, database, storage, server, lifecycle).
Phase ordering is always: Profiling -> Config -> Logger -> Database -> Services -> Server -> Lifecycle.
See `ARCHITECTURE.md` for the full bootstrap pattern reference.

---

## Docker Conventions

- **ALWAYS use**: `docker compose` (not `docker-compose`)
- **Development**: `-f docker-compose.base.yml -f docker-compose.dev.yml`
- **Production**: `-f docker-compose.base.yml -f docker-compose.prod.yml`
- **Container naming**: `north-cloud-{service}` pattern
- **Database access**: `docker exec -it north-cloud-postgres-SERVICE psql -U postgres -d DATABASE`

### Production URLs

- **Dashboard/PHP app**: `https://northcloud.one` (Caddy → Waaseyaa PHP)
- **Go API services**: `https://api.northcloud.one` (Caddy → nginx → Go services)
- **Search API**: `POST https://api.northcloud.one/api/v1/search` or `GET https://api.northcloud.one/api/v1/search?q=...`
- Do NOT use `northcloud.one` for Go API calls — it routes to the PHP dashboard, not the Go services

### Production Deployment

- Production (`/home/deployer/north-cloud`) is **NOT a git repo** — do not use `git pull`
- CI/CD (GitHub Actions) syncs files via tar archive and runs `deploy.sh`
- To deploy manually: push to main → CI runs tests → deploy workflow triggers automatically
- **Nginx uses `--force-recreate`** — volume-mounted config changes aren't detected by `up -d`
- **Force deploy**: `gh workflow run deploy.yml -f force_rebuild_all=true` to rebuild all services
- **Stale file cleanup**: Deploy pre-deletes `*/migrations/*.sql` and `infrastructure/` configs before extracting tar. Renamed/removed files in these paths are cleaned automatically.
- **Migration prefix validation**: Deploy fails fast if duplicate migration prefixes are detected (prevents golang-migrate crashes).
- **Health checks + auto-rollback**: `deploy.sh` snapshots images before deploy, runs health checks after restart, and auto-rolls-back failed services.
- **Runbook**: See `docs/RUNBOOK.md` for rollback procedures and troubleshooting.

---

## Git Workflow

**Branch Naming**: MUST start with `claude/` and end with the session ID
- Format: `claude/{description}-{session-id}`
- Example: `claude/create-claude-md-01YMXWZpqv3utVH69jyNnLaE`

**Before Committing**:
1. Run tests: `go test ./...`
2. Run linter: `task lint:force` (bypasses cache so local results match CI exactly)
3. Run spec drift check: `task drift:check` (ensure affected specs are up to date)
4. Verify no linting violations (see Critical Rules above)
5. Check multi-service dependencies if applicable

**Pre-push hook** (lefthook): Runs `tools/drift-detector.sh` to check for stale specs before push.

**CI pipeline**: `task drift:check` runs first (before lint) in `ci:`, `ci:changed:`, and `ci:force:` tasks. GitHub Actions also runs a parallel `spec-drift` job.

**Pushing**: Always use `git push -u origin {branch-name}` — never force push to main

### GitHub Workflow Rules

1. **Every issue gets a milestone** — untriaged issues surfaced by SessionStart hook
2. **PRs close issues explicitly** — use `Closes #NNN` in PR body
3. **Milestones have due dates** — stale milestones flagged by SessionStart hook
4. **Conventional commits** — `type(scope): description` (feat/fix/chore/docs/refactor/test/ci/perf)
5. **PR template enforces checklist** — issue ref, milestone, lint, tests, spec updates
6. **Stacked PR merge sequence** — retarget dependents to `main` before merging the parent; see `docs/specs/workflow.md` Rule 6

See `docs/specs/workflow.md` for full details. Governance hook: `bin/check-milestones`.

---

## Troubleshooting

**Nginx crash-loop on fresh deploy**: Self-signed certs in `infrastructure/nginx/certs/` are gitignored. Generate per `infrastructure/nginx/certs/README.md`. Symptom: `cannot load certificate "/etc/nginx/certs/server.crt"`.

**Server migration gotcha**: When rsyncing stateful files to a new path, also check for gitignored files that services bind-mount (certs, generated configs). `git ls-files --others --ignored --exclude-standard` lists them.

**Production docker commands**: `jones` user requires `sudo` for all `docker compose` commands on prod — `.env` is not readable without it.

**Redis auth on prod**: Production Redis requires password (`REDIS_PASSWORD` in `.env`). Predis URL format: `tcp://:PASSWORD@redis:6379`. For pub/sub consumers, must set `read_write_timeout=0` in client config or the connection times out and crashes.

**Docker compose prod overrides lose base config**: `<<: *prod-defaults` replaces the entire service definition. `command`, `volumes`, `ports`, `healthcheck` from base are lost — re-declare them in the prod override.

**ES container name**: `north-cloud-elasticsearch-1` (note `-1` suffix from compose scaling), not `north-cloud-elasticsearch`.

**Squid proxy crash-loop after deploy**: If Squid logs (`squid/logs/`) get wrong ownership (e.g. after path migration), Squid crashes with `Cannot open access.log for writing`. Fix: `sudo rm squid/logs/*.log && docker compose ... restart squid`. See #498.

**Classifier binary path**: `/root/classifier` inside container (not `/app/`). No `migrate` CLI command — use `scripts/run-migration.sh classifier up` from the host (requires `golang-migrate` Docker image).

**Rules API field names**: POST `/api/v1/rules` expects `topic` (not `topic_name`), `priority` as string (`"high"`, `"normal"`, `"low"`), and `keywords` as array. The handler auto-generates `rule_name` as `{topic}_detection`.

**ES `_update_by_query` with wildcard**: `*_classified_content` works for `_count` and `_search` but `_update_by_query` returns `total: 0`. Run updates per-index instead.

**Deploy skips SQL-only changes**: The deploy pipeline filters on changed service directories. Migrations and docs-only PRs won't trigger a deploy. Run migrations manually or use the classifier Rules API to hot-reload topic rules.

**Production is not a git repo**: NC on production (`/home/deployer/north-cloud/`) is deployed as Docker images, not cloned from git. Migration files aren't on disk — use the Rules API or copy files manually.

**Docker IPs are ephemeral**: Container IPs (e.g. `172.18.0.x`) change on restart. Don't hardcode them in Caddy or config. Use Docker DNS names or port mapping instead.

**New Go service checklist**: When adding a new Go service, you need: (1) `vuln:` task in the service's own `Taskfile.yml`, (2) `vuln:service-name` delegation in root `Taskfile.yml`, (3) service name in `GO_SERVICES` in `scripts/detect-changed-services.sh`. Missing any of these breaks CI.

**Oneshot Docker services** (like signal-crawler): Use `restart: "no"` in compose, add the service to the health-check skip list in `scripts/deploy.sh`, and manage the systemd timer via Ansible (`northcloud-ansible` repo, `north-cloud` role).

**Non-root container volume ownership**: If a Dockerfile creates a non-root user (e.g. uid 1000), host-mounted volumes must be owned by that uid. Ansible `file:` tasks default to `deploy_user` (uid 1001), which causes "unable to open database file" errors. Use `owner: "1000"` explicitly.

**signal-crawler config.yml overrides SetDefaults**: Non-empty values in `config.yml` prevent `config.go` `SetDefaults()` from applying. If you change a default URL in code, also update `config.yml` or leave the YAML field empty (`""`).

**GC Jobs blocked from DigitalOcean**: The government site (`emploisfp-psjobs.cfp-psc.gc.ca`) is unreachable from DO VPS IPs. Needs a residential proxy or the crawl-proxy. See #617.

Check logs: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs SERVICE`
| Check ports: `netstat -tulpn | grep PORT` | DB test: `docker exec -it north-cloud-postgres-SERVICE psql -U postgres -d DATABASE`
| Health: `curl http://localhost:PORT/health` | See `DOCKER.md` for Docker firewall (UFW) details.

---

## Architectural Boundaries

North Cloud is the **content pipeline layer**. It owns crawling, classification (rules + ML), enrichment, routing, Redis pub/sub, and the source registry.

**North Cloud does NOT own:**
- Entity model, frontend rendering, or dialect/language data (that's Minoo)
- Framework internals, entity storage, or ingestion envelope contract (that's Waaseyaa)
- Content curation or editorial decisions (that's the consuming apps)

**Import rules:**
- NC classifier must import category/region slugs from `jonesrussell/indigenous-taxonomy` Go package — not hardcode them
- NC must not reference Minoo entity types, PHP classes, or templates
- NC source-manager is the single registry for all content sources (crawled + structured + API)

**Shared contracts:**
- `jonesrussell/indigenous-taxonomy` — categories, regions (Go module)
- Redis pub/sub channels follow taxonomy slugs: `indigenous:category:{slug}`, `indigenous:region:{slug}`
