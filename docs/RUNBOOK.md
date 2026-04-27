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

### Signal crawler timer

The production `signal-crawler` job is a Docker Compose oneshot managed by
`~/dev/northcloud-ansible`, not a hand-maintained cron entry. The timer runs
daily at 06:00 UTC and uses `image-tags.env` so it follows the same
`SIGNAL_CRAWLER_TAG` that CI wrote during deploy.

```bash
# Apply or refresh the systemd unit, timer, data directory, and .env values
cd ~/dev/northcloud-ansible
ansible-playbook playbooks/site.yml --tags north-cloud

# Inspect schedule and recent runs on the VPS
ssh deployer@northcloud.one
systemctl list-timers signal-crawler.timer
journalctl -u signal-crawler.service -n 100 --no-pager

# Run once without waiting for the next scheduled scan
sudo systemctl start signal-crawler.service
journalctl -u signal-crawler.service -f
```

---

## Signal Producer

The `signal-producer` is a host systemd `Type=oneshot` unit that fires every
15 minutes (`OnCalendar=*:0/15`). Each run reads classified-content hits
since the last successful checkpoint, maps them to the Waaseyaa signal
format, and POSTs them to `https://northops.ca/api/signals`. Unit files
live in `signal-producer/deploy/`; checkpoint state lives at
`/var/lib/signal-producer/checkpoint.json`; secrets at
`/etc/signal-producer/env` (root:root, 0600).

- **Production install is Ansible-managed.** Binary, systemd unit + timer,
  user, and env file are deployed via the `north-cloud` role in
  [`jonesrussell/northcloud-ansible`](https://github.com/jonesrussell/northcloud-ansible).
  The smoke-test commands below assume Ansible has run successfully. If
  `systemctl status signal-producer.timer` says `Loaded: not-found`, the
  Ansible run hasn't happened — apply the playbook before troubleshooting
  further.

### First-run smoke test

After a deploy that touched signal-producer, run on the VPS:

```bash
ssh prod
sudo systemctl status signal-producer.timer
sudo systemctl list-timers | grep signal-producer
# Wait up to 15 min for the first fire, then:
sudo journalctl -u signal-producer --since "20 minutes ago" | grep run_summary
```

Expect at least one `run_summary` line within 30 minutes. If the first
run failed because the env file was freshly seeded from `env.example`,
populate `WAASEYAA_API_KEY` from 1Password and trigger a manual run:

```bash
sudo $EDITOR /etc/signal-producer/env   # paste the real API key
sudo systemctl start signal-producer.service
sudo journalctl -u signal-producer -n 50 --no-pager
```

### Source-down triage

The producer emits a single WARN with code `signal_producer.source_down`
after three consecutive runs return `ingested == 0`. If you wired this to
an alerter (see `signal-producer/quickstart.md`), you'll see the alert
fire. Triage:

```bash
# Was the alert raised? (filter the last 24h)
sudo journalctl -u signal-producer --since "24 hours ago" \
  | grep '"code":"signal_producer.source_down"'

# What were the recent run summaries? Look for total_signals=0 streaks.
sudo journalctl -u signal-producer --since "24 hours ago" \
  | grep '"event":"run_summary"'

# Check whether ES has anything new in the producer's window.
curl -s "http://localhost:9200/*_classified_content/_count?q=*" | jq .
```

If `total_signals=0` for hours but the upstream classified indexes are
growing, the producer's checkpoint may be ahead of reality — see the
force-rewind recipe below. If the upstream indexes themselves are flat,
the issue is in the content pipeline (crawler / classifier), not the
producer.

### Force a checkpoint rewind

Use this when Waaseyaa lost data and asked for a replay, or when
diagnosing a source-down false positive. Waaseyaa-side dedup via
`external_id` is the backstop against duplicate leads, but verify with
the Waaseyaa team before rewinding more than a few hours.

```bash
sudo systemctl stop signal-producer.timer
sudo -u signal-producer python3 -c "
import json, datetime
path = '/var/lib/signal-producer/checkpoint.json'
data = json.load(open(path))
new_ts = (datetime.datetime.fromisoformat(data['last_successful_run'].rstrip('Z')) - datetime.timedelta(hours=1)).strftime('%Y-%m-%dT%H:%M:%SZ')
data['last_successful_run'] = new_ts
json.dump(data, open(path, 'w'))
"
sudo systemctl start signal-producer.timer
# Force a fire now rather than waiting for the next 15-min boundary:
sudo systemctl start signal-producer.service
```

Adjust the `timedelta(hours=1)` to taste. The next run reprocesses the
rewound window; Waaseyaa-side dedup keyed on `external_id` prevents
duplicate leads.

### Failed-run triage

```bash
sudo systemctl status signal-producer.service
sudo journalctl -u signal-producer -n 100 --no-pager
sudo journalctl -u signal-producer.service -p err --since "7 days ago"
```

Common error patterns:

| Symptom (in journald)                                            | Cause                                            | Fix                                                                                          |
| ---------------------------------------------------------------- | ------------------------------------------------ | -------------------------------------------------------------------------------------------- |
| `error="config: WAASEYAA_API_KEY missing"`                       | `EnvironmentFile=` did not load                  | `ls -l /etc/signal-producer/env` — must be `root:root 0600` and contain `WAASEYAA_API_KEY=`. |
| HTTP 401 from Waaseyaa                                           | API key invalid or rotated upstream              | Rotate via 1Password; see "Rotating the API key" below.                                      |
| HTTP 5xx after all retries                                       | Waaseyaa is down                                 | Wait one timer cycle (15 min); the next fire retries from the same checkpoint.               |
| `error="checkpoint: write failed: permission denied"`            | `/var/lib/signal-producer/` ownership wrong      | `sudo chown -R signal-producer:signal-producer /var/lib/signal-producer/`.                   |
| `signal_producer.source_down` repeating for hours                | Upstream classifier / crawler not producing hits | Investigate the content pipeline, not the producer. Start with the crawler dashboards.       |

A failed run leaves the checkpoint untouched, so the next 15-min fire
retries automatically. No manual intervention is needed unless the
failure is persistent.

### Rotating the API key

```bash
sudo systemctl stop signal-producer.timer
sudo $EDITOR /etc/signal-producer/env       # paste new WAASEYAA_API_KEY
sudo systemctl daemon-reload                # picks up env file changes
sudo systemctl start signal-producer.timer
sudo systemctl start signal-producer.service   # force one immediate run
sudo journalctl -u signal-producer -n 30 --no-pager   # verify success
```

Expect a `run_summary` line with `errors=0`. If the new key is bad
you'll see HTTP 401 in the journal — fix the env file and rerun the
last two commands.

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
ssh deployer@northcloud.one
cd /home/deployer/north-cloud

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
ssh deployer@northcloud.one
cd /home/deployer/north-cloud

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
  -v /home/deployer/north-cloud/crawler/migrations:/migrations \
  migrate/migrate:latest \
  -path /migrations \
  -database "postgres://${POSTGRES_CRAWLER_USER}:${POSTGRES_CRAWLER_PASSWORD}@postgres-crawler:5432/${POSTGRES_CRAWLER_DB:-crawler}?sslmode=disable" \
  down 1

# If migration is in dirty state, force to previous version
docker run --rm --network north-cloud_north-cloud-network \
  -v /home/deployer/north-cloud/crawler/migrations:/migrations \
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
ssh deployer@northcloud.one
cd /home/deployer/north-cloud

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
ssh deployer@northcloud.one
rm /home/deployer/north-cloud/<path-to-stale-file>
```

### Nginx config not reloading

Nginx uses `--force-recreate` in deploy.sh (Step 3.5). If config still stale:

```bash
ssh deployer@northcloud.one
cd /home/deployer/north-cloud
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml \
  up -d --force-recreate nginx
```

Caddy (TLS termination) is a host process, not Docker:

```bash
sudo systemctl reload caddy
```
