# alert-crawler CLAUDE.md

> Read this file before modifying any alert-crawler code.
> Deep spec: `docs/specs/community-alert-pipeline.md` (WP23 will create it).

## Overview

Oneshot Go service that polls RSS/Atom community-alert feeds, scores items by severity,
deduplicates via SQLite catalogue, and publishes to Elasticsearch + Redis pub/sub.
Managed by a systemd timer (via Ansible `northcloud-ansible`, `north-cloud` role).

## Architecture

```
main.go
internal/
├── config/       L0 — YAML + env config; SetDefaults provides all defaults
├── domain/       L0 — Alert type, SeverityLevel, RescissionState
├── adapter/      L1 — Source interface
│   └── rss/      L1 — RSS/Atom fetch + parse (gofeed)
├── catalogue/    L1 — SQLite dedup/rescission store (data/alerts.db)
├── elasticsearch/ L1 — Index + update alert documents
├── redis/        L1 — Publish alert events to channel
├── severity/     L1 — Keyword scoring → SeverityLevel
├── scope/        L1 — Taxonomy slug matching via indigenous-taxonomy
├── observability/ L1 — Structured logging helpers
└── runner/       L2 — Orchestrator: fetch → score → dedup → publish
```

Layer boundaries enforced by `.layers` and `task layers:check`.

## Known Gotchas

### RSS HTTP client User-Agent (WP08)

The RSS HTTP client (`internal/adapter/rss/Client`) sends:

```
User-Agent: alert-crawler/1.0 (+https://northcloud.one)
```

This string is intentionally public and identifies the service if safersites.ca or
other upstream operators want to contact us. If debugging 403/Cloudflare blocks, match
this string against server-side access logs. Override via `rss.WithUserAgent(ua)` in
tests or if the upstream demands a custom header. See RR-001 in WP08 for the
Cloudflare-block risk and mitigation deferral.

### RR-007 — config.yml overrides SetDefaults silently

`infrastructure/config.LoadWithDefaults[T]` merges YAML BEFORE applying SetDefaults. A non-empty
YAML field silently shadows SetDefaults's value. As a result, a code-side default change has no
effect if the YAML field carries an old value. Mitigation: keep `config.yml` mostly comments;
only uncomment fields you intend to override per-environment. Unit test
`internal/config/pitfall_test.go` enforces this.

### ES mapping is in-service (single-writer rationale)

The `community_alerts` ES index is single-writer (alert-crawler is the only producer). Per the
resolved charter tension (`kitty-specs/community-alert-pipeline-01KQZC7A/plan.md` section 1,
approved 2026-05-06), the mapping lives in `internal/elasticsearch/mapping.json` and is applied
via idempotent `EnsureIndex` (HEAD -> PUT on 404) at startup. The `index-manager` service is NOT
involved. Charter rule "ES mapping changes go through index-manager" applies to shared indices
(`*_classified_content` family).

### go.mod replace directive must be removed before merge (WP19)

During Phase B (parallel dev with the indigenous-taxonomy v1.1.0 release), `go.mod` carries a
`replace github.com/jonesrussell/indigenous-taxonomy => ../indigenous-taxonomy` directive. WP19
removes it and pins `v1.1.0`. CI build fails fast if `replace` is left in.

Also: the `replace github.com/jonesrussell/north-cloud/infrastructure => ../infrastructure`
directive follows the same pattern as all other north-cloud services and stays permanently.

### Rescission semantics: parser-degraded items NEVER auto-rescinded

Parser-degraded items (`parse_quality: degraded`) are persisted as alerts with the flag visible to
operators. They are NEVER auto-rescinded due to the parse failure. Rescission only happens when an
alert is ABSENT from the upstream feed on a subsequent poll (catalogue diff in runner). This is
per TC-010.

Parser-failed items (`parse_quality: failed`) - required fields missing - are SKIPPED entirely and
never enter the catalogue. Failed parses emit a `parse_failure_total` metric.

### SQLite catalogue rebuild path

On startup, if `data/state.db` is missing or empty, alert-crawler can rebuild the active-alert
catalogue from ES (`lifecycle_state == "active"` query). The rebuild is idempotent and does NOT
emit `created` events for the rebuilt entries (they already exist downstream). Rebuild is triggered
by passing `--rebuild` flag or auto-detected from row count == 0.

### Docker volume ownership uid 1000

The container runs as uid 1000 (`alert-crawler` user). Host-mounted volumes (e.g. `data/`)
must be owned by uid 1000 on the host. In Ansible `file:` tasks, use `owner: "1000"` —
NOT `owner: "{{ deploy_user }}"` which maps to uid 1001 and causes "unable to open
database file" errors at runtime. See root CLAUDE.md "Non-root container volume ownership".

### First-deploy backfill burst

Backfill subcommand emits up to 20 `created` lifecycle events on first deploy (TC-011). Downstream
consumers must handle this burst (rate-limit consumer-side if needed). Out of mission scope;
flagged as PR-002.

## Development

```bash
task build          # Compile stub binary (CGO_ENABLED=1 required for SQLite)
task run:dry        # Dry run — no ES/Redis writes
task test           # Unit tests
task lint           # golangci-lint via ../.golangci.yml
task vuln           # govulncheck
```

## Environment Variables

| Variable | Purpose | Required |
|----------|---------|----------|
| `ELASTICSEARCH_URL` | ES base URL | No (default: http://localhost:9200) |
| `ALERT_ES_INDEX` | ES index name | No (default: alert_classified_content) |
| `REDIS_URL` | Redis connection URL | No (default: redis://localhost:6379) |
| `ALERT_REDIS_CHANNEL` | Redis pub/sub channel | No (default: indigenous:alerts) |
| `ALERT_DB_PATH` | SQLite catalogue path | No (default: data/alerts.db) |
| `ALERT_SOURCE_URLS` | Comma-separated RSS/Atom feed URLs | Yes (prod) |
| `LOG_LEVEL` | Log level (default: info) | No |
| `LOG_FORMAT` | Log format: json, console (default: json) | No |
