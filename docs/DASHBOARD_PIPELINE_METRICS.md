# Dashboard Pipeline Metrics — Operator Guide

This document explains what the Pipeline Monitor numbers mean and how to verify them when they look wrong (especially when **Intelligence Overview** shows all zeros).

## What Each Number Is

| Dashboard metric | Source | Definition |
|------------------|--------|------------|
| **Items Crawled** | Crawler `GET /api/v1/stats` | Sum of `items_crawled` from job executions that **started today** (server date) and completed. These are **URLs/pages**, not articles. |
| **Indexed** | Same crawler stats | Sum of `items_indexed` for those same executions. |
| **Classified** | Classifier `GET /api/v1/stats?date=today` | Count of rows in `classification_history` where `classified_at >= start of today` (server date). |
| **Routed / Published** | Publisher `GET /api/v1/stats/overview?period=today` | Count of articles in `publish_history` published today. |
| **Active Channels** | Publisher `GET /api/v1/channels` | Number of **channels** (not routes) that are enabled. |
| **Intelligence Overview** | Index Manager `GET /api/v1/aggregations/overview` | Elasticsearch search across **all** indexes matching `*_classified_content`. |

“Today” is **each service’s server-local date**. If servers use different timezones, the windows won’t match.

---

## When Intelligence Overview Shows All Zeros

The Intelligence Overview (Total Documents, Crime Related %, Quality Distribution, etc.) comes from **index-manager** querying Elasticsearch with the index pattern `*_classified_content`.

If everything shows zero, one of these is true:

1. **No `*_classified_content` indexes exist** in the cluster index-manager uses.
2. **Those indexes are empty.**
3. **Index-manager is pointed at a different ES cluster** than the one the classifier writes to (e.g. dev vs prod).
4. **Index naming or aliases** don’t match the pattern (e.g. rotation without alias update).
5. **Wildcard URL-encoding bug (fixed)** — The go-elasticsearch client used to URL-encode the index pattern, so `*_classified_content` became `%2A_classified_content` and ES matched zero indices. Index-manager now uses a raw request path so the wildcard is sent literally. If you run an older image, redeploy to pick up the fix.

### The One Check That Confirms It

On the **same Elasticsearch cluster that index-manager uses**, run (e.g. in Kibana Dev Tools or with curl):

```
GET _cat/indices/*_classified_content?v
```

- **No indices or 0 docs** → Explains the zeros; fix by ensuring the classifier writes to this cluster and that indexes exist.
- **Indices with docs** → Index-manager is likely pointed at the wrong cluster; fix its config.

### Index-Manager Elasticsearch Config

Index-manager gets the ES URL from:

- **Environment**: `ELASTICSEARCH_URL` (e.g. `http://elasticsearch:9200`)
- **Config file**: `elasticsearch.url` in the service config (e.g. `config.yml`)

Verify in production that index-manager’s `ELASTICSEARCH_URL` (or config) points at the cluster where the classifier writes `*_classified_content` indexes.

---

## Quick Reference

- **Crawler “today”**: `job_executions.started_at >= CURRENT_DATE`, `status = 'completed'` (crawler DB).
- **Classifier “today”**: `classification_history.classified_at >= start of today` (classifier DB).
- **Publisher “today”**: `publish_history.published_at >= start of today` (publisher DB).
- **Intelligence Overview**: ES search on `*_classified_content` (no date filter; all time).
