# North Cloud Production Runbook

## Deployment

### How deploys work

1. Push to `main` triggers CI (`.github/workflows/test.yml`)
2. On CI success, `.github/workflows/deploy.yml` runs automatically
3. Deploy detects changed services, builds Docker images, pushes to Docker Hub
4. Syncs compose files, infrastructure configs, and migrations to production via tar
5. Runs `deploy.sh` on the server: pulls images, runs migrations, restarts services
6. Health checks run automatically — failed services are rolled back

### Manual deploy

```bash
# Deploy all services
gh workflow run deploy.yml

# Force rebuild all services (ignores change detection)
gh workflow run deploy.yml -f force_rebuild_all=true

# Deploy specific services only
gh workflow run deploy.yml -f services="crawler,classifier"
```

### Monitor a deploy

```bash
gh run watch
gh run list --workflow=deploy.yml
gh run view <run-id> --log
```

---

## Rollback Procedures

### Automatic rollback (built into deploy.sh)

`deploy.sh` automatically:
1. Snapshots current Docker image IDs before pulling new ones
2. Runs health checks after restart (Step 4)
3. If any health check fails, re-tags the old image and restarts the service
4. Reports rollback success or failure

No manual intervention needed for service-level failures.

### Manual rollback — failed service

If automatic rollback failed or you need to force a specific version:

```bash
ssh jones@northcloud.one
cd /opt/north-cloud

# Check which services are unhealthy
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml ps

# Restart a specific service with the previous image
# (Docker keeps the previous image locally after pull)
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml \
  up -d --force-recreate <service-name>

# Check logs
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml \
  logs -f <service-name> --tail=50
```

### Manual rollback — bad migration

If a migration broke the database:

```bash
ssh jones@northcloud.one
cd /opt/north-cloud

# Source environment for DB credentials
source .env

# Check migration state (example: crawler)
docker run --rm --network north-cloud_north-cloud-network \
  migrate/migrate:latest \
  -path /dev/null \
  -database "postgres://${POSTGRES_CRAWLER_USER}:${POSTGRES_CRAWLER_PASSWORD}@postgres-crawler:5432/${POSTGRES_CRAWLER_DB:-crawler}?sslmode=disable" \
  version

# Run the down migration (rolls back one step)
docker run --rm --network north-cloud_north-cloud-network \
  -v /opt/north-cloud/crawler/migrations:/migrations \
  migrate/migrate:latest \
  -path /migrations \
  -database "postgres://${POSTGRES_CRAWLER_USER}:${POSTGRES_CRAWLER_PASSWORD}@postgres-crawler:5432/${POSTGRES_CRAWLER_DB:-crawler}?sslmode=disable" \
  down 1

# If migration is in dirty state, force to previous version
docker run --rm --network north-cloud_north-cloud-network \
  -v /opt/north-cloud/crawler/migrations:/migrations \
  migrate/migrate:latest \
  -path /migrations \
  -database "postgres://..." \
  force <previous-version-number>
```

### Manual rollback — full deploy revert

To redeploy the last known good state:

```bash
# Find the last successful deploy commit
git log --oneline deployed

# Trigger a deploy from that commit
gh workflow run deploy.yml -f force_rebuild_all=true
# (This rebuilds from current main — if main is broken, cherry-pick the fix)
```

---

## Troubleshooting

### Service won't start

```bash
ssh jones@northcloud.one
cd /opt/north-cloud

# Check container status
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml ps

# Check logs (last 100 lines)
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml \
  logs <service-name> --tail=100

# Check if port is already in use
netstat -tulpn | grep <port>

# Force recreate
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml \
  up -d --force-recreate <service-name>
```

### Database connection failures

```bash
# Test DB connectivity
docker exec -it north-cloud-postgres-<service> psql -U postgres -d <database> -c "SELECT 1"

# Check if DB container is running
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml ps postgres-<service>

# Restart DB (warning: brief downtime)
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml restart postgres-<service>
```

### Migration duplicate prefix crash

```bash
# Identify duplicates
for dir in */migrations/; do
  dupes=$(ls "$dir" | grep '\.sql$' | sed 's/_.*//' | sort | uniq -d)
  [ -n "$dupes" ] && echo "$dir: $dupes"
done

# Remove the stale file (the old renamed one)
rm <service>/migrations/<old-prefix>_<old-name>.{up,down}.sql

# Re-run migrations
bash deploy.sh
```

### Stale files after deploy

The deploy pipeline cleans migrations and infrastructure configs before extracting.
If other file types persist after rename/removal, manually delete on production:

```bash
ssh jones@northcloud.one
rm /opt/north-cloud/<path-to-stale-file>
```

### Nginx config not reloading

Nginx uses `--force-recreate` in deploy.sh (Step 3.5). If config still stale:

```bash
ssh jones@northcloud.one
cd /opt/north-cloud
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml \
  up -d --force-recreate nginx
```

Caddy (TLS termination) is a host process, not Docker:

```bash
sudo systemctl reload caddy
```
