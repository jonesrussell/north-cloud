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

Non-empty values in `config.yml` prevent `config.go` `SetDefaults()` from applying.
If you change a default URL or value in code, also update `config.yml` to leave that
field blank (`""`), or the code default is never reached. This is the same pattern
as signal-crawler (see root CLAUDE.md troubleshooting section).

### ES mapping is in-service (single-writer rationale)

alert-crawler owns its own ES index mapping (`alert_classified_content`).
No other service writes to this index. Keeping the mapping in-service avoids the
cross-service coupling that would be required if index-manager managed it.
Reindex path: update mapping in `internal/elasticsearch/`, restart service, run
`task migrate:alert-crawler` once that task exists (WP17+).

### go.mod replace directive must be removed before merge (WP19)

The `replace github.com/jonesrussell/indigenous-taxonomy => ../../indigenous-taxonomy`
directive enables parallel-dev without publishing every change. WP19 will replace it
with a real `v1.1.0` pin. Do NOT remove it before WP19 is approved.

Also: the `replace github.com/jonesrussell/north-cloud/infrastructure => ../infrastructure`
directive follows the same pattern as all other north-cloud services and stays permanently.

### Rescission semantics: parser-degraded items NEVER auto-rescinded

If an alert item fails to parse cleanly (missing GUID, malformed date, empty title),
it is logged and skipped — it does NOT trigger rescission of a previously published
alert. Rescission is only triggered by an explicit `<rescind>` element or an item
with `status: cancelled` in the feed. This prevents flapping during feed degradation.

### SQLite catalogue rebuild path

If `data/alerts.db` is corrupt or missing, the service creates a fresh one on startup.
Historical alert state is lost — items that were previously published will be re-published
on the next run. This is acceptable; downstream consumers must be idempotent.
To rebuild manually: `rm data/alerts.db && task run:dry` to verify before a live run.

### Docker volume ownership uid 1000

The container runs as uid 1000 (`alert-crawler` user). Host-mounted volumes (e.g. `data/`)
must be owned by uid 1000 on the host. In Ansible `file:` tasks, use `owner: "1000"` —
NOT `owner: "{{ deploy_user }}"` which maps to uid 1001 and causes "unable to open
database file" errors at runtime. See root CLAUDE.md "Non-root container volume ownership".

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
