# Domain Migration: northcloud.biz → northcloud.one

**Date**: 2026-02-26
**Status**: Approved

## Overview

Switch the public domain from `northcloud.biz` to `northcloud.one`. Same server, same IP — DNS-only change with config updates across the codebase.

## Scope

**Update**: All config, infrastructure, source code, scripts, living docs, and operational references.

**Leave as-is**: Historical plan files in `docs/plans/` (point-in-time records).

## Files to Update

### Infrastructure (critical — affects production)

- `infrastructure/nginx/nginx.conf` — `server_name` directives
- `infrastructure/certbot/scripts/check-cert-expiry.sh` — `DOMAIN` variable
- `infrastructure/certbot/scripts/renew-and-reload.sh` — `DOMAIN` variable
- `infrastructure/certbot/README.md` — cert paths, domain references
- `infrastructure/certbot/QUICK_REFERENCE.md` — curl examples
- `infrastructure/nginx/certs/README.md` — self-signed cert generation commands
- `infrastructure/fetcher/cloud-init.yml` — user-agent string
- `infrastructure/grafana/README.md` — SSH references

### Docker Compose

- `docker-compose.base.yml` — click tracker URL, SMTP from, alert email
- `docker-compose.prod.yml` — Grafana root URL, CSRF origins, SMTP from
- `docker-compose.dev.yml` — SMTP from

### Environment

- `.env.example` — user-agent, click tracker URL, SMTP from
- `.env` — user-agent (if present)

### Source Code

- `dashboard/src/components/SourceQuickCreateModal.vue` — default user-agent string

### MCP / Tooling

- `.mcp.json` — SSH host
- `.cursor/mcp.json` — SSH host
- `mcp-north-cloud/CLAUDE.md` — SSH host in MCP config example
- `mcp-north-cloud/README.md` — SSH host in MCP config example

### Living Docs & Runbooks

- `ARCHITECTURE.md`
- `search/README.md`
- `search-frontend/README.md`
- `publisher/docs/DEPLOYMENT.md`
- `docs/STREETCODE_RUNBOOK.md`
- `docs/RECLASSIFY_AFTER_CLASSIFIER_FIXES.md`
- `docs/ops-add-anishinaabe-sources.md`
- `dashboard/PHASE_5_INLINE_VALIDATION.md`

### Scripts

- `scripts/sync-enabled-sources-jobs.sh`
- `scripts/validate-dashboard-numbers.sh`
- `scripts/add-anishinaabe-sources.sh`

### Claude Memory

- `.claude/projects/-home-fsd42-dev-north-cloud/memory/MEMORY.md`

## Production Deployment Steps

1. **DNS** — Add A record for `northcloud.one` pointing to same server IP
2. **Deploy** — Pull updated configs to `/opt/north-cloud`
3. **Certbot** — Issue new Let's Encrypt cert: `certbot certonly --webroot -w /var/www/certbot -d northcloud.one`
4. **Restart nginx** — Pick up new `server_name` and cert paths
5. **Verify** — `curl -I https://northcloud.one/` returns 200
6. **Optional** — Keep `northcloud.biz` DNS pointing to server temporarily for redirect grace period

## What This Does NOT Change

- Service-to-service communication (all internal Docker network)
- Database connection strings (all internal)
- Elasticsearch/Redis configs (all internal)
- Historical plan documents in `docs/plans/`
