# North Cloud Deploy User Migration

> Migrate the north-cloud production deployment from `/opt/north-cloud/` (owned by `jones`) to `/home/deployer/north-cloud/` (owned by `deployer`), aligning with the existing Ansible convention used by all Laravel and Waaseyaa apps.

## Problem

The north-cloud Docker stack on production has mixed ownership:
- `/opt/north-cloud/` directory itself is owned by `deployer` (set by Ansible)
- 25 of 27 files inside are owned by `jones` (created by GH Actions deploying as `jones`)
- `.env` is now owned by `deployer` (just fixed via Ansible template)
- GH Actions SSHs as `jones` via `DEPLOY_USER` secret
- Ansible convention is `deploy_user: deployer` for all other apps
- Crontab runs as `jones` but writes to `/home/deployer/`

This creates permission conflicts, confusion about which user should own what, and diverges from the pattern used by every other app on the server.

## Design

### Target State

| Aspect | Before | After |
|---|---|---|
| Deploy path | `/opt/north-cloud/` | `/home/deployer/north-cloud/` |
| File ownership | `jones:jones` (most files) | `deployer:deployer` (all files) |
| GH Actions SSH user | `jones` | `deployer` |
| Ansible `north_cloud_path` | `/opt/north-cloud` | `/home/deployer/north-cloud` |
| Docker project name | `north-cloud` (from dir name) | `north-cloud` (unchanged) |
| Crontab | `jones` | `deployer` |

### Why Docker Volumes Are Safe

Docker named volumes are identified by name, not filesystem path. The current volumes are prefixed `north-cloud_*` which comes from the Docker Compose project name. Since the directory name stays `north-cloud`, the project name is unchanged and all volumes reattach automatically at the new path. No data migration needed for:
- 8 Postgres databases (auth, classifier, click_tracker, crawler, index_manager, pipeline, publisher, source_manager)
- Elasticsearch, Redis, MinIO data
- Grafana, Loki, Prometheus
- Certbot TLS certificates

**Pre-flight check**: Verify `.env` does not set `COMPOSE_PROJECT_NAME` (which would override the directory-based name). Currently it does not.

### Stateful Files to Copy

These files are not in git and must be moved:

| File | Size | Notes |
|---|---|---|
| `.env` | 8 KB | Ansible re-templates this, but copy as safety net |
| `.env.backup` | 5 KB | Old backup |
| `Caddyfile` | 122 B | Should be Ansible-templated in future |
| `proxy-ips.conf` | small | IP allowlist for squid |
| `image-tags.env` | small | Current deployed image digests |
| `backups/` | 27 MB | Manual DB backups |
| `data/communities/communities.ndjson` | varies | Communities dataset for ES |
| `deploy.sh` | 22 KB | Deployment orchestration |
| `scripts/db-backup.sh` | small | Backup utility |
| `scripts/db-utils.sh` | small | DB utility |
| `squid/` | small | Squid config + logs (bind-mounted by docker-compose.prod.yml) |

Everything else (service dirs, migrations, docker-compose files, ML code) is recreated from git on each deploy.

### Migration Sequence

**Phase 1: Prepare (no downtime)**

1. Add SSH key for `deployer` user on production (copy from `jones` or generate new)
2. Update Ansible `north_cloud_path` default to `/home/deployer/north-cloud`
3. Run Ansible to create new directory and template `.env` at new path
4. Copy stateful files: `rsync -a /opt/north-cloud/{Caddyfile,proxy-ips.conf,image-tags.env,backups,data,deploy.sh,scripts,squid} /home/deployer/north-cloud/`
5. `chown -R deployer:deployer /home/deployer/north-cloud/`
6. Update code (pre-merge, no deploy yet):
   - `deploy.sh`: change `DEPLOY_DIR="/opt/north-cloud"` to `/home/deployer/north-cloud`
   - `.github/workflows/deploy.yml`: update all 3 hardcoded `cd /opt/north-cloud` (lines 207, 233, 256) to `/home/deployer/north-cloud`
   - `docker-compose.prod.yml`: update squid bind-mount paths (lines 129-130, 697) from `/opt/north-cloud/squid/` to `/home/deployer/north-cloud/squid/`
   - `scripts/manage-ips.sh`: update default paths (lines 32-34, 42)
7. Migrate crontab: `sudo crontab -u deployer -e` — move the backup job from `jones` crontab, updating paths from `/opt/north-cloud` to `/home/deployer/north-cloud`

**Phase 2: Switch (brief downtime, ~5 min)**

8. `cd /opt/north-cloud && docker compose -f docker-compose.base.yml -f docker-compose.prod.yml down`
9. Create symlink: `sudo ln -sfn /home/deployer/north-cloud /opt/north-cloud` (catches any missed references)
10. Update GH Actions secrets: `DEPLOY_USER=deployer`, `DEPLOY_SSH_KEY=<deployer's key>`
11. Merge the code changes from step 6 — GH Actions deploys to new path as `deployer`
12. If deploy doesn't auto-trigger: `cd /home/deployer/north-cloud && docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d`
13. Health check all services: `curl http://localhost:PORT/health` for each service

**Phase 3: Cleanup (after 7-day confidence period)**

14. Remove old jones crontab entry: `sudo crontab -u jones -r` (if no other entries)
15. Update north-cloud `CLAUDE.md` and `DOCKER.md` with new path references
16. After 7 days stable: remove symlink, `sudo rm -rf /opt/north-cloud.old` (rename first, then delete)

### Changes Required

**northcloud-ansible:**
- `roles/north-cloud/defaults/main.yml`: change `north_cloud_path` to `/home/deployer/north-cloud`
- `roles/north-cloud/tasks/main.yml`: add deployer SSH key setup task

**north-cloud (GitHub) — all before cutover:**
- `.github/workflows/deploy.yml`: update 3 hardcoded `cd /opt/north-cloud` paths (lines 207, 233, 256)
- `scripts/deploy.sh` line 26: change `DEPLOY_DIR="/opt/north-cloud"` to `/home/deployer/north-cloud`
- `docker-compose.prod.yml`: update squid bind-mount paths (lines 129-130, 697)
- `scripts/manage-ips.sh`: update default paths (lines 32-34, 42)
- `CLAUDE.md`, `DOCKER.md`: update path references

**GitHub Actions secrets:**
- `DEPLOY_USER`: change from `jones` to `deployer`
- `DEPLOY_SSH_KEY`: set to deployer's SSH private key

**Production server (manual):**
- Add SSH authorized_key for deployer
- Migrate crontab entry from jones to deployer (Phase 1, not Phase 3)

### Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Docker volumes don't reattach | Pre-flight: `docker compose config \| grep "^name:"` — verify project name is `north-cloud` |
| `.env` sets `COMPOSE_PROJECT_NAME` | Pre-flight: verify it does not (currently clean) |
| GH Actions deploy fails as deployer | Test SSH before switching: `ssh deployer@northcloud.one whoami` |
| Squid container fails (bind-mount paths) | Update `docker-compose.prod.yml` paths in Phase 1 code changes |
| Stray hardcoded `/opt/north-cloud` refs | Symlink created in Phase 2 step 9 catches these during confidence period |
| Downtime during switch | Phase 2 is ~5 minutes: stop, symlink, deploy, start |
| Backup cron misses runs | Crontab migrated in Phase 1 (step 7), not deferred to Phase 3 |

### Out of Scope

- Ansible-templating `Caddyfile` (separate task)
- Ansible-templating `deploy.sh` (it's deployed from git)
- Changing the Docker Compose project name
- Migrating Docker volumes to different names
