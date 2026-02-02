# Crawler

Web crawler service for the North Cloud content pipeline. See [CLAUDE.md](CLAUDE.md) and [docs/](docs/) for architecture, scheduler, and API details.

## Colly behavior

**Robots.txt**  
Respected by default. Colly only ignores robots.txt when explicitly configured. To disable, set `CRAWLER_RESPECT_ROBOTS_TXT=false`.

**Random user agents**  
Disabled by default. To rotate user agents per request, set `CRAWLER_USE_RANDOM_USER_AGENT=true`.

**Production**  
- Keep `CRAWLER_RESPECT_ROBOTS_TXT=true` (default).  
- Set `CRAWLER_USE_RANDOM_USER_AGENT=true` if you want UA rotation.

## Sync enabled sources to jobs

To ensure every enabled source in source-manager has a scheduled crawler job (e.g. on production after missed events), call the admin sync endpoint:

```bash
# From repo root; requires JWT (e.g. from dashboard login)
JWT="your-token" ./scripts/sync-enabled-sources-jobs.sh
```

- **Endpoint:** `POST /api/v1/admin/sync-enabled-sources` (JWT protected).  
- **Behavior:** Lists enabled sources, creates missing jobs (with deterministic stagger), resumes paused jobs; returns a report (`created`, `already_has_job`, `resumed`, `skipped_disabled`, `skipped_rate_limit_parse_failure`).

See [scripts/sync-enabled-sources-jobs.sh](../scripts/sync-enabled-sources-jobs.sh) for `CRAWLER_URL` and `JWT` usage.
