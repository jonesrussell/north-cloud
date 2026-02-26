# Domain Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace all references to `northcloud.biz` with `northcloud.one` across config, infrastructure, source code, scripts, and living docs.

**Architecture:** Pure find-and-replace across ~30 files. No logic changes. No tests needed — this is config/docs only. Each task groups related files so a single commit makes sense.

**Tech Stack:** Bash (sed), Git

---

### Task 1: Infrastructure — Nginx Config

**Files:**
- Modify: `infrastructure/nginx/nginx.conf:101` — HTTP server_name
- Modify: `infrastructure/nginx/nginx.conf:121` — HTTPS server_name

**Step 1: Update nginx.conf**

Line 101: `server_name northcloud.biz localhost;` → `server_name northcloud.one localhost;`
Line 121: `server_name northcloud.biz;` → `server_name northcloud.one;`

**Step 2: Commit**

```bash
git add infrastructure/nginx/nginx.conf
git commit -m "infra: update nginx server_name to northcloud.one"
```

---

### Task 2: Infrastructure — Certbot Scripts

**Files:**
- Modify: `infrastructure/certbot/scripts/check-cert-expiry.sh:7`
- Modify: `infrastructure/certbot/scripts/renew-and-reload.sh:7`

**Step 1: Update check-cert-expiry.sh**

Line 7: `DOMAIN="northcloud.biz"` → `DOMAIN="northcloud.one"`

**Step 2: Update renew-and-reload.sh**

Line 7: `DOMAIN="northcloud.biz"` → `DOMAIN="northcloud.one"`

**Step 3: Commit**

```bash
git add infrastructure/certbot/scripts/check-cert-expiry.sh infrastructure/certbot/scripts/renew-and-reload.sh
git commit -m "infra: update certbot scripts domain to northcloud.one"
```

---

### Task 3: Docker Compose Files

**Files:**
- Modify: `docker-compose.base.yml:260` — CLICK_TRACKER_BASE_URL default
- Modify: `docker-compose.base.yml:625` — GF_SMTP_FROM_ADDRESS default
- Modify: `docker-compose.base.yml:627` — GRAFANA_ALERT_EMAIL default
- Modify: `docker-compose.prod.yml:631` — GF_SERVER_ROOT_URL
- Modify: `docker-compose.prod.yml:633` — GF_SECURITY_CSRF_TRUSTED_ORIGINS
- Modify: `docker-compose.prod.yml:641` — GF_SMTP_FROM_ADDRESS
- Modify: `docker-compose.dev.yml:741` — GF_SMTP_FROM_ADDRESS

**Step 1: Update docker-compose.base.yml**

Replace all `northcloud.biz` → `northcloud.one` (3 occurrences):
- `CLICK_TRACKER_BASE_URL: ${CLICK_TRACKER_BASE_URL:-https://northcloud.one/api}`
- `GF_SMTP_FROM_ADDRESS: ${GF_SMTP_FROM_ADDRESS:-noreply@northcloud.one}`
- `GRAFANA_ALERT_EMAIL: ${GRAFANA_ALERT_EMAIL:-alerts@northcloud.one}`

**Step 2: Update docker-compose.prod.yml**

Replace all `northcloud.biz` → `northcloud.one` (3 occurrences):
- `GF_SERVER_ROOT_URL: "https://northcloud.one/grafana/"`
- `GF_SECURITY_CSRF_TRUSTED_ORIGINS: "northcloud.one"`
- `GF_SMTP_FROM_ADDRESS: "noreply@northcloud.one"`

**Step 3: Update docker-compose.dev.yml**

Replace `northcloud.biz` → `northcloud.one` (1 occurrence):
- `GF_SMTP_FROM_ADDRESS: "noreply@northcloud.one"`

**Step 4: Commit**

```bash
git add docker-compose.base.yml docker-compose.prod.yml docker-compose.dev.yml
git commit -m "config: update docker compose domain to northcloud.one"
```

---

### Task 4: Environment Files

**Files:**
- Modify: `.env.example:65` — CRAWLER_USER_AGENT
- Modify: `.env.example:182` — CLICK_TRACKER_BASE_URL
- Modify: `.env.example:309` — GF_SMTP_FROM_ADDRESS
- Modify: `.env:64` — CRAWLER_USER_AGENT

**Step 1: Update .env.example**

Replace all `northcloud.biz` → `northcloud.one` (3 occurrences):
- `CRAWLER_USER_AGENT=Mozilla/5.0 (compatible; NorthCloud/1.0; +https://northcloud.one)`
- `CLICK_TRACKER_BASE_URL=https://northcloud.one/api`
- `GF_SMTP_FROM_ADDRESS=noreply@northcloud.one`

**Step 2: Update .env**

Replace `northcloud.biz` → `northcloud.one` (1 occurrence):
- `CRAWLER_USER_AGENT=Mozilla/5.0 (compatible; NorthCloud/1.0; +https://northcloud.one)`

**Step 3: Commit**

```bash
git add .env.example .env
git commit -m "config: update env files domain to northcloud.one"
```

---

### Task 5: Source Code — Dashboard Vue Component

**Files:**
- Modify: `dashboard/src/components/SourceQuickCreateModal.vue:504` — default user_agent
- Modify: `dashboard/src/components/SourceQuickCreateModal.vue:711` — reset user_agent

**Step 1: Update SourceQuickCreateModal.vue**

Replace all `northcloud.biz` → `northcloud.one` (2 occurrences):
- Line 504: `user_agent: 'Mozilla/5.0 (compatible; NorthCloud/1.0; +https://northcloud.one)',`
- Line 711: `user_agent: 'Mozilla/5.0 (compatible; NorthCloud/1.0; +https://northcloud.one)',`

**Step 2: Commit**

```bash
git add dashboard/src/components/SourceQuickCreateModal.vue
git commit -m "fix(dashboard): update default user-agent domain to northcloud.one"
```

---

### Task 6: MCP / Tooling Configs

**Files:**
- Modify: `.mcp.json:18` — SSH host
- Modify: `.cursor/mcp.json:18` — SSH host
- Modify: `mcp-north-cloud/CLAUDE.md` — SSH host in example config
- Modify: `mcp-north-cloud/README.md` — SSH host in example config

**Step 1: Update .mcp.json and .cursor/mcp.json** *(done — files renamed to `.example`, originals gitignored)*

Both files were renamed to `.example` variants with placeholder `user@your-server`. The security credential scrub (commit `34334bb`) superseded the original domain-only migration for these files.

**Step 2: Update mcp-north-cloud/CLAUDE.md and README.md** *(done)*

SSH host references updated to `northcloud.one`; credentials replaced with `user@your-server` placeholder.

**Step 3: Commit**

```bash
git add .mcp.json .cursor/mcp.json mcp-north-cloud/CLAUDE.md mcp-north-cloud/README.md
git commit -m "config: update MCP configs SSH host to northcloud.one"
```

---

### Task 7: Infrastructure — Fetcher Cloud Init

**Files:**
- Modify: `infrastructure/fetcher/cloud-init.yml:15`

**Step 1: Update cloud-init.yml**

Line 15: `FETCHER_USER_AGENT=NorthCloud-Fetcher/1.0 (+https://northcloud.biz/crawler)` → `FETCHER_USER_AGENT=NorthCloud-Fetcher/1.0 (+https://northcloud.one/crawler)`

**Step 2: Commit**

```bash
git add infrastructure/fetcher/cloud-init.yml
git commit -m "infra: update fetcher user-agent domain to northcloud.one"
```

---

### Task 8: Scripts

**Files:**
- Modify: `scripts/sync-enabled-sources-jobs.sh` — 3 references (comments + URLs)
- Modify: `scripts/validate-dashboard-numbers.sh` — 1 reference (comment)
- Modify: `scripts/add-anishinaabe-sources.sh` — 1 reference (comment)

**Step 1: Update all scripts**

Replace all `northcloud.biz` → `northcloud.one` in each file.

**Step 2: Commit**

```bash
git add scripts/sync-enabled-sources-jobs.sh scripts/validate-dashboard-numbers.sh scripts/add-anishinaabe-sources.sh
git commit -m "scripts: update domain references to northcloud.one"
```

---

### Task 9: Living Docs — Infrastructure

**Files:**
- Modify: `infrastructure/certbot/README.md` — ~8 references
- Modify: `infrastructure/certbot/QUICK_REFERENCE.md` — 1 reference
- Modify: `infrastructure/nginx/certs/README.md` — 2 references
- Modify: `infrastructure/grafana/README.md` — 1 reference

**Step 1: Update all infrastructure docs**

Replace all `northcloud.biz` → `northcloud.one` in each file.

**Step 2: Commit**

```bash
git add infrastructure/certbot/README.md infrastructure/certbot/QUICK_REFERENCE.md infrastructure/nginx/certs/README.md infrastructure/grafana/README.md
git commit -m "docs: update infrastructure docs domain to northcloud.one"
```

---

### Task 10: Living Docs — Service READMEs and Runbooks

**Files:**
- Modify: `ARCHITECTURE.md:85` — 1 reference
- Modify: `search/README.md:47` — 1 reference
- Modify: `search-frontend/README.md:9` — 1 reference
- Modify: `publisher/docs/DEPLOYMENT.md` — ~5 references
- Modify: `docs/STREETCODE_RUNBOOK.md` — ~3 references
- Modify: `docs/RECLASSIFY_AFTER_CLASSIFIER_FIXES.md` — ~6 references
- Modify: `docs/ops-add-anishinaabe-sources.md` — 1 reference
- Modify: `dashboard/PHASE_5_INLINE_VALIDATION.md` — 3 references

**Step 1: Update all living docs**

Replace all `northcloud.biz` → `northcloud.one` in each file.

**Step 2: Commit**

```bash
git add ARCHITECTURE.md search/README.md search-frontend/README.md publisher/docs/DEPLOYMENT.md docs/STREETCODE_RUNBOOK.md docs/RECLASSIFY_AFTER_CLASSIFIER_FIXES.md docs/ops-add-anishinaabe-sources.md dashboard/PHASE_5_INLINE_VALIDATION.md
git commit -m "docs: update service docs and runbooks domain to northcloud.one"
```

---

### Task 11: Claude Memory

**Files:**
- Modify: `/home/fsd42/.claude/projects/-home-fsd42-dev-north-cloud/memory/MEMORY.md`

**Step 1: Update MEMORY.md**

Replace all `northcloud.biz` → `northcloud.one`.

**Step 2: No commit needed** (memory files are outside the repo)

---

### Task 12: Verification

**Step 1: Search for any remaining references**

```bash
grep -r "northcloud\.biz" --include="*.yml" --include="*.yaml" --include="*.json" --include="*.sh" --include="*.conf" --include="*.vue" --include="*.ts" --include="*.go" --include="*.env*" --include="*.md" . | grep -v "docs/plans/2026-"
```

Expected: No output (all non-historical references replaced).

**Step 2: Verify docker compose configs parse correctly**

```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml config --quiet
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml config --quiet
```

Expected: Exit code 0, no errors.
