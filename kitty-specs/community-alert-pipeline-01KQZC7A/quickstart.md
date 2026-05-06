# Quickstart — Community Alert Pipeline

**Mission**: `community-alert-pipeline-01KQZC7A`
**Audience**: Implementer, reviewer, operator running alert-crawler locally for the first time.

This is the developer-facing quickstart. For the consumer contract see `contracts/redis-channels.md`. For the canonical envelope see `contracts/community-alert.schema.json`.

---

## 1. Prerequisites

- North Cloud monorepo cloned at `/home/jones/dev/north-cloud`.
- `indigenous-taxonomy` sibling repo cloned at `/home/jones/dev/indigenous-taxonomy` (required for the Phase B `replace` directive).
- `task` (Taskfile) installed.
- Docker + Docker Compose installed.
- Go 1.26+ installed.

```bash
cd /home/jones/dev/north-cloud
task install:tools          # ensures golangci-lint version pin is honoured
```

---

## 2. Bring up dependencies

Start the core north-cloud stack and a search overlay (alert-crawler depends on Elasticsearch and Redis):

```bash
cd /home/jones/dev/north-cloud
task docker:dev:up:search   # core + search (includes ES + Redis)
```

Verify ES is ready:

```bash
docker exec north-cloud-elasticsearch-1 curl -s localhost:9200/_cluster/health?pretty
```

Verify Redis is ready:

```bash
docker exec north-cloud-redis-1 redis-cli PING
# → PONG
```

---

## 3. Build alert-crawler

```bash
cd /home/jones/dev/north-cloud/alert-crawler
task build
```

The first build will fetch the `indigenous-taxonomy` package via the `replace` directive in `go.mod`. If the build fails with `missing go.sum entry`, run `go mod tidy` once.

---

## 4. Configure

`alert-crawler/config.yml` is the developer default. Inspect it; it should leave `SetDefaults`-controlled fields blank (RR-007 pitfall).

For local development, set the dev `.env` overrides:

```bash
# At repo root
cat >> .env <<'EOF'
ALERT_CRAWLER_FEED_URL=https://www.safersites.ca/drugalerts.rss
ALERT_CRAWLER_POLL_INTERVAL=30m
ALERT_CRAWLER_DB_PATH=/app/data/state.db
ALERT_CRAWLER_ES_URL=http://elasticsearch:9200
ALERT_CRAWLER_ES_INDEX=community_alerts
ALERT_CRAWLER_REDIS_URL=redis://redis:6379
ALERT_CRAWLER_REDIS_CHANNEL=community_alerts:lifecycle
ALERT_CRAWLER_DEFAULT_EXPIRY=720h
EOF
```

---

## 5. Run a single poll cycle

Alert-crawler is a oneshot binary. Run it manually:

```bash
cd /home/jones/dev/north-cloud
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml run --rm alert-crawler
```

Expected output (structured JSON via Zap):

```
{"level":"info","msg":"alert-crawler starting","service":"alert-crawler"}
{"level":"info","msg":"ES index ready","index":"community_alerts"}
{"level":"info","msg":"polling source","source_id":"safersites","feed_url":"https://www.safersites.ca/drugalerts.rss"}
{"level":"info","msg":"feed parsed","items":20,"http_status":200}
{"level":"info","msg":"alert created","alert_id":"safersites:20260505fentanyl","severity":"critical"}
...
{"level":"info","msg":"poll cycle complete","duration_ms":4521,"created":20,"updated":0,"rescinded":0}
```

---

## 6. Inspect Elasticsearch

```bash
# Confirm the index exists
docker exec north-cloud-elasticsearch-1 curl -s 'localhost:9200/community_alerts?pretty'

# Count documents
docker exec north-cloud-elasticsearch-1 curl -s 'localhost:9200/community_alerts/_count?pretty'

# Query currently-active alerts (consumers' read pattern)
docker exec north-cloud-elasticsearch-1 curl -s 'localhost:9200/community_alerts/_search?pretty' \
  -H 'Content-Type: application/json' \
  -d '{
    "query": {
      "bool": {
        "must": [
          { "term": { "lifecycle_state": "active" } },
          { "range": { "expires_at": { "gt": "now" } } }
        ]
      }
    },
    "size": 5
  }'

# Query alerts scoped to Treaty 1
docker exec north-cloud-elasticsearch-1 curl -s 'localhost:9200/community_alerts/_search?pretty' \
  -H 'Content-Type: application/json' \
  -d '{
    "query": { "term": { "scope": "treaty:1" } },
    "size": 5
  }'
```

---

## 7. Subscribe to lifecycle events

In one terminal:

```bash
docker exec -it north-cloud-redis-1 redis-cli SUBSCRIBE community_alerts:lifecycle
```

In another terminal, run a poll cycle (step 5). The first terminal receives one JSON message per `created`/`updated`/`rescinded` event.

Validate the payload against `contracts/lifecycle-event.schema.json`.

---

## 8. Test rescission semantics

To exercise rescission detection without modifying the live upstream feed, you can:

1. Populate the catalogue from a normal poll (step 5).
2. Edit the dev fixture (planned in WP B.14) to omit one alert from the simulated feed response.
3. Re-run the poll cycle. Observe a `rescinded` event for the omitted alert.
4. Verify the ES document now has `lifecycle_state = "rescinded"`.

The integration test `internal/runner/rescission_integration_test.go` (`//go:build integration`) automates this scenario end-to-end against an ephemeral ES + Redis + SQLite stack via the existing CI integration harness.

---

## 9. Test idempotency (NFR-006)

Run the poll cycle twice in a row without changing upstream content:

```bash
docker compose ... run --rm alert-crawler   # creates events
docker compose ... run --rm alert-crawler   # should be a no-op
```

Subscribe to Redis (step 7) during the second run. Expected: zero messages on the channel during the second run. If any messages appear, the idempotency check has regressed — open a bug.

---

## 10. Run the test suite

```bash
cd /home/jones/dev/north-cloud
task test:alert-crawler                  # unit tests, fast
task test:alert-crawler -- -tags integration  # integration tests against real ES+Redis+SQLite (slower)
task lint:alert-crawler                  # golangci-lint
task vuln:alert-crawler                  # govulncheck
```

---

## 11. Drift, ports, and layers

```bash
task drift:check        # verify spec vs code is in sync
task ports:check        # verify ports SSOT after compose changes
task layers:check       # verify .layers boundary is clean
```

These run automatically in lefthook pre-push and CI. Run locally before pushing.

---

## 12. Reset state (developer convenience)

```bash
# Stop and remove alert-crawler containers and volumes
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml down -v

# Or just wipe the SQLite state and ES index
docker volume rm north-cloud_alert-crawler-data
docker exec north-cloud-elasticsearch-1 curl -X DELETE 'localhost:9200/community_alerts'
```

---

## 13. Observability

Structured metrics flow through Zap to the existing Loki/Grafana stack (start with `task docker:dev:up:observability`). Key metrics to dashboard:

- `alert_crawler.poll.duration_ms` — feed-fetch latency by source.
- `alert_crawler.alert.created_total` / `updated_total` / `rescinded_total` — lifecycle volume.
- `alert_crawler.parse.failure_total` — parser-degradation signal (RR-002).
- `alert_crawler.consecutive_failures` — operator-actionable when ≥6 (NFR-005).

A v1.5 follow-up may add a Prometheus `/metrics` endpoint if needed; out of scope here.

---

## 14. Where to look when something is wrong

| Symptom | First look |
|---|---|
| Container fails to start, "unable to open database file" | Check volume ownership: container user is uid 1000; Ansible `file:` task in Phase C.4 must use `owner: "1000"` (not `deploy_user`). |
| `replace` directive fails CI | Phase C.1 must remove the directive before merge. Pin `github.com/jonesrussell/indigenous-taxonomy v1.1.0` first. |
| ES write fails with 400 | Check the index mapping; the `dynamic: "strict"` setting rejects unmapped fields. Add the field to `mapping.go` and bump the index version. |
| Redis publish fails | Verify `REDIS_PASSWORD` is set in `.env`. Production Redis requires auth (NC convention). |
| Polls all return 304 | The 4-hour server-side cache on safersites.ca means most polls are no-ops. Force a fresh fetch by clearing the stored ETag in SQLite. |

---

## 15. Next

After this quickstart works for you, run `/spec-kitty.tasks` to generate the work-package files. Then `/spec-kitty.implement` per WP.
