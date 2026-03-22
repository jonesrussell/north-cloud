# Deploy User Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move north-cloud production from `/opt/north-cloud/` (jones) to `/home/deployer/north-cloud/` (deployer), aligning with the Ansible convention used by all other apps.

**Architecture:** Update all hardcoded `/opt/north-cloud` paths to `/home/deployer/north-cloud` across deploy scripts, docker-compose, manage-ips, and docs. Set up deployer SSH access. Use Ansible to provision the new directory. Symlink old path during confidence period.

**Tech Stack:** Ansible, GitHub Actions, Docker Compose, Bash, SSH

**Spec:** `docs/superpowers/specs/2026-03-21-deployer-migration-design.md`

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `scripts/deploy.sh:3,26` | Modify | Change `DEPLOY_DIR` constant + comment |
| `.github/workflows/deploy.yml:207,233,256` | Modify | Update 3 hardcoded `cd` paths |
| `docker-compose.prod.yml:129-130,697` | Modify | Update squid bind-mount paths |
| `scripts/manage-ips.sh:32-35,42` | Modify | Update default path constants + doc comment |
| `CLAUDE.md:182` | Modify | Update production path reference |
| `DOCKER.md` | Verify | Pre-verified: no `/opt/north-cloud` references |
| `docs/RUNBOOK.md:55-56,78-79,89,97,126-127,179-180,188-189` | Modify | Update all SSH + path examples |
| `northcloud-ansible: roles/north-cloud/defaults/main.yml:5` | Modify | Change `north_cloud_path` |
| `northcloud-ansible: roles/north-cloud/tasks/main.yml` | Modify | Add deployer SSH key task |

---

### Task 1: Update deploy.sh DEPLOY_DIR

**Files:**
- Modify: `scripts/deploy.sh:3,26`

- [ ] **Step 1: Update the comment on line 3**

```bash
# Before:
# This script should be placed at /opt/north-cloud/deploy.sh on the production server
# After:
# This script should be placed at /home/deployer/north-cloud/deploy.sh on the production server
```

- [ ] **Step 2: Update the DEPLOY_DIR constant on line 26**

```bash
# Before:
DEPLOY_DIR="/opt/north-cloud"
# After:
DEPLOY_DIR="/home/deployer/north-cloud"
```

- [ ] **Step 3: Verify no other /opt/north-cloud references in the file**

Run: `grep -n '/opt/north-cloud' scripts/deploy.sh`
Expected: No matches

- [ ] **Step 4: Commit**

```bash
git add scripts/deploy.sh
git commit -m "chore(deploy): update DEPLOY_DIR to /home/deployer/north-cloud"
```

---

### Task 2: Update GitHub Actions deploy workflow

**Files:**
- Modify: `.github/workflows/deploy.yml:207,233,256`

- [ ] **Step 1: Update all 3 hardcoded paths**

Replace all occurrences of `cd /opt/north-cloud` with `cd /home/deployer/north-cloud`:

Line 207 (sync files step):
```yaml
          ssh -p "${SSH_PORT}" "${DEPLOY_USER}@${DEPLOY_HOST}" '
            cd /home/deployer/north-cloud
```

Line 233 (image tag manifest step):
```yaml
            cd /home/deployer/north-cloud
            touch image-tags.env
```

Line 256 (deploy services step):
```yaml
          ssh -p "${SSH_PORT}" "${DEPLOY_USER}@${DEPLOY_HOST}" "
            cd /home/deployer/north-cloud
```

- [ ] **Step 2: Verify no remaining /opt references**

Run: `grep -n '/opt/north-cloud' .github/workflows/deploy.yml`
Expected: No matches

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/deploy.yml
git commit -m "ci(deploy): update deploy paths to /home/deployer/north-cloud"
```

---

### Task 3: Update docker-compose.prod.yml squid bind-mounts

**Files:**
- Modify: `docker-compose.prod.yml:129-130,697`

- [ ] **Step 1: Update squid container volumes**

Lines 129-130:
```yaml
    volumes:
      - /home/deployer/north-cloud/squid/squid.conf:/etc/squid/squid.conf:ro
      - /home/deployer/north-cloud/squid/logs:/var/log/squid
```

- [ ] **Step 2: Update alloy squid logs mount**

Line 697:
```yaml
      - /home/deployer/north-cloud/squid/logs:/mnt/squid-logs:ro
```

- [ ] **Step 3: Verify no remaining /opt references**

Run: `grep -n '/opt/north-cloud' docker-compose.prod.yml`
Expected: No matches

- [ ] **Step 4: Commit**

```bash
git add docker-compose.prod.yml
git commit -m "chore(compose): update squid bind-mount paths for deployer migration"
```

---

### Task 4: Update manage-ips.sh default paths

**Files:**
- Modify: `scripts/manage-ips.sh:32-35,42`

- [ ] **Step 1: Update all default path constants**

Also check for any `/opt/north-cloud` in header comments and update those too.

Lines 32-35:
```bash
INVENTORY_FILE="${INVENTORY_FILE:-/home/deployer/north-cloud/proxy-ips.conf}"
SQUID_CONF="${SQUID_CONF:-/home/deployer/north-cloud/squid/squid.conf}"
SQUID_LOG_DIR="${SQUID_LOG_DIR:-/home/deployer/north-cloud/squid/logs}"
```

Line 42:
```bash
COMPOSE_DIR="${COMPOSE_DIR:-/home/deployer/north-cloud}"
```

- [ ] **Step 2: Verify no remaining /opt references**

Run: `grep -n '/opt/north-cloud' scripts/manage-ips.sh`
Expected: No matches

- [ ] **Step 3: Commit**

```bash
git add scripts/manage-ips.sh
git commit -m "chore(scripts): update manage-ips.sh paths for deployer migration"
```

---

### Task 5: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md:182`

- [ ] **Step 1: Update production path reference**

Line 182:
```markdown
- Production (`/home/deployer/north-cloud`) is **NOT a git repo** — do not use `git pull`
```

- [ ] **Step 2: Search for any other /opt/north-cloud references in CLAUDE.md**

Run: `grep -n '/opt/north-cloud' CLAUDE.md`
Expected: No matches

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md production path to deployer home"
```

---

### Task 6: Update RUNBOOK.md

**Files:**
- Modify: `docs/RUNBOOK.md` (multiple lines)

- [ ] **Step 1: Replace all /opt/north-cloud references**

Use replace-all to change `/opt/north-cloud` to `/home/deployer/north-cloud` throughout the file.

Also replace `ssh jones@northcloud.one` with `ssh deployer@northcloud.one` throughout (the runbook should use the deploy user, not jones).

- [ ] **Step 2: Verify no remaining /opt references**

Run: `grep -n '/opt/north-cloud' docs/RUNBOOK.md`
Expected: No matches

Run: `grep -n 'ssh jones@' docs/RUNBOOK.md`
Expected: No matches

- [ ] **Step 3: Commit**

```bash
git add docs/RUNBOOK.md
git commit -m "docs: update RUNBOOK.md paths and SSH user for deployer migration"
```

---

### Task 7: Update Ansible north_cloud_path

**Files:**
- Modify: `/home/jones/dev/northcloud-ansible/roles/north-cloud/defaults/main.yml:5`

- [ ] **Step 1: Update the path default**

Line 5:
```yaml
north_cloud_path: /home/deployer/north-cloud
```

- [ ] **Step 2: Commit (in northcloud-ansible repo)**

```bash
cd /home/jones/dev/northcloud-ansible
git add roles/north-cloud/defaults/main.yml
git commit -m "chore(north-cloud): update north_cloud_path to deployer home"
```

---

### Task 8: Add deployer SSH key setup to Ansible

**Files:**
- Modify: `/home/jones/dev/northcloud-ansible/roles/north-cloud/tasks/main.yml`

- [ ] **Step 1: Add SSH key task after "Create north-cloud directory"**

Add after the directory creation task:

```yaml
- name: Ensure deployer has SSH authorized key for GH Actions
  ansible.posix.authorized_key:
    user: "{{ deploy_user }}"
    key: "{{ vault_nc_deploy_ssh_public_key }}"
    state: present
  when: vault_nc_deploy_ssh_public_key is defined
```

- [ ] **Step 2: Commit (in northcloud-ansible repo)**

```bash
cd /home/jones/dev/northcloud-ansible
git add roles/north-cloud/tasks/main.yml
git commit -m "feat(north-cloud): add deployer SSH key provisioning task"
```

---

### Task 9: Production cutover (manual steps)

These steps are executed on the production server, not in code.

- [ ] **Step 1: Pre-flight checks**

```bash
ssh jones@northcloud.one
# Verify .env doesn't set COMPOSE_PROJECT_NAME
grep COMPOSE_PROJECT_NAME /opt/north-cloud/.env
# Should return nothing

# Verify docker project name
cd /opt/north-cloud
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml config | head -1
# Should show: name: north-cloud
```

- [ ] **Step 2: Generate SSH keypair for deployer (if not reusing jones key)**

```bash
ssh jones@northcloud.one
sudo -u deployer ssh-keygen -t ed25519 -C "deployer@northcloud.one" -f /home/deployer/.ssh/id_ed25519 -N ""
sudo cat /home/deployer/.ssh/id_ed25519.pub
# Copy the public key — add to vault as vault_nc_deploy_ssh_public_key
# Copy the private key — set as GH Actions DEPLOY_SSH_KEY secret
```

Or copy jones' authorized_keys to deployer:
```bash
sudo cp /home/jones/.ssh/authorized_keys /home/deployer/.ssh/authorized_keys
sudo chown deployer:deployer /home/deployer/.ssh/authorized_keys
```

- [ ] **Step 3: Run Ansible to create new directory + template .env**

```bash
cd /home/jones/dev/northcloud-ansible
ansible-playbook playbooks/site.yml --tags north-cloud --diff
```

- [ ] **Step 4: Copy stateful files to new path**

```bash
ssh jones@northcloud.one
sudo rsync -a /opt/north-cloud/{Caddyfile,proxy-ips.conf,image-tags.env,backups,data,deploy.sh,scripts,squid} /home/deployer/north-cloud/
sudo chown -R deployer:deployer /home/deployer/north-cloud/
```

- [ ] **Step 5: Migrate crontab**

```bash
ssh jones@northcloud.one
crontab -l  # Copy the backup line
sudo crontab -u deployer -e  # Add it with updated paths: /home/deployer/north-cloud/...
crontab -r  # Remove jones crontab (only if no other entries)
```

- [ ] **Step 6: Test SSH as deployer**

```bash
ssh deployer@northcloud.one whoami
# Should output: deployer
ssh deployer@northcloud.one "ls /home/deployer/north-cloud/.env"
# Should succeed
```

- [ ] **Step 7: Stop services at old path**

```bash
ssh jones@northcloud.one
cd /opt/north-cloud
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml down
```

- [ ] **Step 8: Create symlink for safety**

```bash
ssh jones@northcloud.one
sudo mv /opt/north-cloud /opt/north-cloud.old
sudo ln -sfn /home/deployer/north-cloud /opt/north-cloud
```

- [ ] **Step 9: Update GH Actions secrets**

> **CRITICAL: Steps 9 and 10 are an atomic pair. Do NOT push code (step 10) before updating secrets (step 9). If code is pushed while DEPLOY_USER is still `jones`, GH Actions will SSH as jones but `cd /home/deployer/north-cloud` will fail because that path isn't accessible to jones. This breaks the deploy.**

In GitHub repo settings (jonesrussell/north-cloud → Settings → Secrets):
- `DEPLOY_USER`: change to `deployer`
- `DEPLOY_SSH_KEY`: set to deployer's private key (or keep jones' if key was copied)

Verify the secret change took effect:
```bash
# Check via gh CLI
gh secret list -R jonesrussell/north-cloud | grep DEPLOY
```

- [ ] **Step 10: Push code changes and deploy**

Only after confirming Step 9 secrets are updated:
```bash
cd /home/jones/dev/north-cloud
git push origin main
# GH Actions auto-deploys as deployer to /home/deployer/north-cloud
```

- [ ] **Step 11: Health check**

```bash
ssh deployer@northcloud.one
cd /home/deployer/north-cloud
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml ps
# All services should be Up

# Spot-check key services:
curl -s http://localhost:8040/health  # auth
curl -s http://localhost:8080/health  # crawler
curl -s http://localhost:8090/health  # search
```

---

### Task 10: Cleanup (after 7-day confidence period)

- [ ] **Step 1: Verify everything has been stable for 7 days**

Check GH Actions deploy history — all deploys since cutover should succeed.

- [ ] **Step 2: Remove symlink and old directory**

```bash
ssh jones@northcloud.one  # jones still has SSH access for admin
sudo rm /opt/north-cloud  # remove symlink
sudo rm -rf /opt/north-cloud.old  # remove old files
```

- [ ] **Step 3: Commit any remaining doc updates**

If any stray references were found during the confidence period, fix them now.
