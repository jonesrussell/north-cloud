# Adding Anishinaabe sources (production)

This doc describes how to add the tiered Anishinaabe/Indigenous sources to **production** North Cloud so more content flows to diidjaaheer.live via the existing Redis channels (`articles:anishinaabe`, `anishinaabe:category:*`). No code or config changes are needed on Diidjaaheer.

## Prerequisites on production

1. **Anishinaabe ML enabled**
   - In `.env` on the North Cloud server set:
     - `ANISHINAABE_ENABLED=true`
     - `ANISHINAABE_ML_SERVICE_URL=http://anishinaabe-ml:8080`
   - Ensure the **anishinaabe-ml** service is running (e.g. `docker compose -f docker-compose.base.yml -f docker-compose.prod.yml ps`). Without this, new articles will not get `anishinaabe` classification and will not be published to Anishinaabe channels.

2. **Auth credentials**
   - You need a JWT for source-manager and crawler API calls. Either:
     - Set `JWT` to an existing token, or
     - Set `AUTH_USERNAME` and `AUTH_PASSWORD` so the script can obtain a token from the auth service.

3. **jq**
   - The script uses `jq` to parse JSON. Install if missing: `apt-get install jq` / `yum install jq`.

## Run the script on production

On the North Cloud server (e.g. `jones@northcloud.biz`, app at `/opt/north-cloud`), auth, source-manager, and crawler **do not publish ports to the host** in production. Run the script **inside a container on the Docker network** so it can reach those services by hostname.

```bash
# From the North Cloud server
docker run --rm --network north-cloud_north-cloud-network \
  -v /opt/north-cloud:/opt/north-cloud:ro -w /opt/north-cloud \
  -e AUTH_USERNAME -e AUTH_PASSWORD --env-file /opt/north-cloud/.env \
  -e AUTH_URL=http://auth:8040 \
  -e SOURCE_MANAGER_URL=http://source-manager:8050 \
  -e CRAWLER_URL=http://crawler:8080 \
  alpine:3.19 sh -c 'apk add --no-cache curl jq bash && bash /opt/north-cloud/scripts/add-anishinaabe-sources.sh'
```

**Important:** Use `CRAWLER_URL=http://crawler:8080` (port **8080** is the crawler’s internal port; 8060 is only used when the crawler port is published to the host, e.g. in dev).

Export `AUTH_USERNAME` and `AUTH_PASSWORD` in your shell before the `docker run` so the container receives them (e.g. `export AUTH_USERNAME=admin` and set the password).

If you run the script directly on the host (e.g. with nginx proxying to auth/source-manager/crawler), use `AUTH_URL`, `SOURCE_MANAGER_URL`, and `CRAWLER_URL` pointing at localhost or the proxy (e.g. `http://localhost:8040`, `http://localhost:8050`, `http://localhost:8060` if those ports are exposed).

## What the script does

- Reads source list from **scripts/anishinaabe-sources-data.json** (Tier 1–6 from the plan).
- For each entry:
  1. **POST /api/v1/sources** (source-manager): creates a source with default selectors (`h1`, `article`, `time[datetime]`), `rate_limit=10`, `max_depth=3`, `ingestion_mode=spider`, `enabled=true`. Optional `feed_url` from the JSON is set when present.
  2. **POST /api/v1/jobs** (crawler): creates a recurring job for that source with `schedule_enabled=true`, default interval 360 minutes (6 hours). Override with `INTERVAL_MINUTES` and `INTERVAL_TYPE` if needed.

After the script runs, the crawler will pick up the new jobs; raw content goes to `{source}_raw_content`, the classifier (with anishinaabe-ml) writes to `{source}_classified_content`, and the publisher’s Layer 7 routes Anishinaabe-classified articles to `articles:anishinaabe` and `anishinaabe:category:*`. Diidjaaheer already subscribes to those channels, so no change there.

## Optional: test selectors first

The plan recommends testing selectors per source when a site structure is non-standard. To do that on production:

1. Get a JWT (e.g. `curl -s -X POST http://localhost:8040/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"..."}'`).
2. Call **POST /api/v1/sources/test-crawl** with a sample article URL and selectors (see source-manager CLAUDE.md). Adjust selectors if needed.
3. Then add the source (via the script or manually) using the validated selectors. The script uses a single default selector set; for custom selectors you’d add the source via the dashboard or API and then create the crawler job separately.

## Diidjaaheer

- No env or code changes are required on Diidjaaheer (deployer@coforge.xyz, diidjaaheer/current).
- Ensure Diidjaaheer’s `NORTHCLOUD_REDIS_*` points at the same Redis the North Cloud publisher uses so it receives the new articles.

## See also

- Plan: `~/.cursor/plans/anishinaabe_sources_production_*.plan.md` (tiered source list and operational notes).
- [Publisher CLAUDE.md](../publisher/CLAUDE.md) — Layer 7 Anishinaabe routing.
- [Source Manager CLAUDE.md](../source-manager/CLAUDE.md) — API and selectors.
