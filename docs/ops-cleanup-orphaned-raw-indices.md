# Cleanup Orphaned Raw Indices

> Related: issue `#489`

After PR `#487`, the Colly raw-content path uses the configured source-manager source name as its canonical raw-index identity. Older raw indices created from URL hosts can remain in Elasticsearch beside the new canonical index names.

Examples:
- canonical: `sudbury_com_raw_content`
- legacy URL-derived: `www_sudbury_com_raw_content`

This runbook explains how to audit and safely delete those legacy raw indices.

## Audit First

Run the audit script against Source Manager and Elasticsearch. When running inside the Docker network (e.g. via `docker exec`), no JWT is needed. When running from outside, provide a valid JWT:

```bash
# Inside Docker network (no auth needed):
SOURCE_MANAGER_URL=http://source-manager:8050 \
ELASTICSEARCH_URL=http://elasticsearch:9200 \
./scripts/audit-orphaned-raw-indices.sh

# Outside Docker network (JWT required):
SOURCE_MANAGER_URL=http://localhost:8050 \
ELASTICSEARCH_URL=http://localhost:9200 \
SOURCE_MANAGER_JWT="$(curl -fsS http://localhost:8040/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"..."}' | jq -r .token)" \
./scripts/audit-orphaned-raw-indices.sh
```

JSON output is available with:

```bash
SOURCE_MANAGER_URL=http://localhost:8050 \
ELASTICSEARCH_URL=http://localhost:9200 \
./scripts/audit-orphaned-raw-indices.sh --json
```

The script compares:
- canonical raw index names derived from source-manager source names
- legacy URL-host-derived raw index names derived from source URLs plus `www`/non-`www` variants
- actual `*_raw_content` indices in Elasticsearch

It paginates through the source-manager API, so large source catalogs are audited completely instead of stopping at the first page.

For each likely legacy index, it reports:
- `legacy_index`
- `canonical_index`
- `canonical_exists`
- `doc_count`
- `pending_count`
- `latest_crawled_at`
- `review_state`

## Delete Gates

Treat a legacy raw index as safe to delete only when all of these are true:

1. The canonical replacement index exists.
2. The legacy index has no pending classifier backlog: `pending_count = 0`.
3. `latest_crawled_at` is older than the rollout window you want to retain.
4. No host-collision ambiguity exists for that legacy index.

Recommended interpretation of `review_state`:
- `delete_candidate`: ready for operator review and likely safe to delete
- `wait_pending_zero`: do not delete yet; classifier backlog still exists
- `review_missing_canonical`: do not delete yet; canonical replacement was not found
- `review_collision`: do not delete automatically; one host-derived legacy index appears to map to multiple configured sources

## Why Pending Count Matters

`raw_content` is transient, but deleting a legacy raw index with pending documents can still discard unclassified content that has not moved downstream yet. Wait until pending backlog reaches zero, or explicitly accept re-crawl as the recovery path.

## Delete Procedure

Delete one index through index-manager:

```bash
curl -X DELETE http://localhost:8090/api/v1/indexes/<legacy_index_name>
```

Or delete directly through Elasticsearch if index-manager is unavailable:

```bash
curl -X DELETE http://localhost:9200/<legacy_index_name>
```

## Post-Delete Checks

After deletion, confirm:

1. The canonical raw index still exists.
2. The classifier backlog for that source remains healthy.
3. Dashboard or source-health views do not show an unexpected drop for the canonical source.

Useful checks:

```bash
curl "http://localhost:8090/api/v1/indexes?type=raw_content&search=sudbury"
curl "http://localhost:8090/api/v1/aggregations/source-health"
```

## Limits Of The Audit

This audit is intentionally conservative.

It covers the common migration case by using:
- the configured source `name` for canonical index naming
- the source URL host plus `www`/non-`www` variants for legacy index naming

Manual review is still required for:
- redirected crawl hosts outside the source URL host
- historic indices created from alternate subdomains
- multi-tenant hosts where one host maps to multiple logical sources
