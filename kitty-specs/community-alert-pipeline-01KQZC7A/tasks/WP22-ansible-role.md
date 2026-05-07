---
work_package_id: WP22
title: Ansible Role (Timer + Service Templates)
dependencies:
- WP05
- WP20
requirement_refs:
- C-007
- C-010
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T094
- T095
- T096
- T097
phase: C
agent: "claude:sonnet:implementer:implementer"
shell_pid: "523843"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: ../northcloud-ansible/roles/north-cloud/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- ../northcloud-ansible/roles/north-cloud/templates/alert-crawler.timer.j2
- ../northcloud-ansible/roles/north-cloud/templates/alert-crawler.service.j2
- ../northcloud-ansible/roles/north-cloud/defaults/main.yml
- ../northcloud-ansible/roles/north-cloud/tasks/alert-crawler.yml
priority: P1
tags: []
---

# WP22 — Ansible Role (Timer + Service Templates)

## Objective

Add `alert-crawler.timer.j2` and `alert-crawler.service.j2` to the existing `north-cloud` Ansible role. Parameterize cadence via `nc_alert_crawler_schedule`. Critically, fix the uid mismatch on the data directory (PR-004): host `deploy_user` is uid 1001 but container user is uid 1000.

This is a **cross-repo work package** operating on `../northcloud-ansible/`.

## Context

- Plan §Phased Build Sequence Phase C.4
- Research R-002 (signal-crawler ansible role pattern)
- Risk PR-004 (uid mismatch)
- Repo CLAUDE.md ("Non-root container volume ownership" gotcha)

## Branch Strategy

Cross-repo. The agent must commit changes inside `../northcloud-ansible/` (separate git repo). A separate PR is opened against that repo's `main`.

## Subtasks

### T094 — Create `alert-crawler.timer.j2`

**Purpose**: systemd timer unit template parameterized with `nc_alert_crawler_schedule`.

**Steps**:
1. Read `../northcloud-ansible/roles/north-cloud/templates/signal-crawler.timer.j2` for the canonical pattern.
2. Create `../northcloud-ansible/roles/north-cloud/templates/alert-crawler.timer.j2`:
   ```ini
   [Unit]
   Description=alert-crawler timer (community alert pipeline)
   After=docker.service

   [Timer]
   OnCalendar={{ nc_alert_crawler_schedule | default('*-*-* *:30:00 UTC') }}
   Persistent=true
   RandomizedDelaySec=120
   Unit=alert-crawler.service

   [Install]
   WantedBy=timers.target
   ```
3. The default `*-*-* *:30:00 UTC` runs hourly at half-past. `RandomizedDelaySec=120` spreads load across the hour boundary.
4. `nc_alert_crawler_schedule` is the operator-tunable variable (T096).

**Files**:
- `../northcloud-ansible/roles/north-cloud/templates/alert-crawler.timer.j2` (new, ~12 lines).

### T095 — Create `alert-crawler.service.j2`

**Purpose**: systemd service unit. `Type=oneshot`, runs `docker compose ... run --rm alert-crawler`.

**Steps**:
1. Read `../northcloud-ansible/roles/north-cloud/templates/signal-crawler.service.j2` for the canonical pattern.
2. Create `../northcloud-ansible/roles/north-cloud/templates/alert-crawler.service.j2`:
   ```ini
   [Unit]
   Description=alert-crawler oneshot run
   After=docker.service
   Requires=docker.service

   [Service]
   Type=oneshot
   WorkingDirectory={{ nc_install_dir }}
   EnvironmentFile={{ nc_install_dir }}/.env
   ExecStartPre=/usr/bin/docker compose -f docker-compose.base.yml -f docker-compose.prod.yml pull alert-crawler
   ExecStart=/usr/bin/docker compose -f docker-compose.base.yml -f docker-compose.prod.yml run --rm alert-crawler
   StandardOutput=journal
   StandardError=journal

   [Install]
   WantedBy=multi-user.target
   ```
3. `nc_install_dir` is an existing role variable (typically `/home/deployer/north-cloud`).

**Files**:
- `../northcloud-ansible/roles/north-cloud/templates/alert-crawler.service.j2` (new, ~16 lines).

### T096 — Add `nc_alert_crawler_schedule` default

**Purpose**: Parameterize cadence.

**Steps**:
1. Edit `../northcloud-ansible/roles/north-cloud/defaults/main.yml`:
   ```yaml
   # alert-crawler timer cadence (systemd OnCalendar format).
   # Default runs hourly at half-past with a 2-min randomized delay.
   # Cadence is bounded by the spec at 30-60 min (FR-001).
   nc_alert_crawler_schedule: "*-*-* *:30:00 UTC"

   # Path on the host where alert-crawler's persistent SQLite state lives.
   # Volume-mounted into the container at /app/data.
   # Owner MUST be uid 1000 (container user), NOT deploy_user (uid 1001).
   nc_alert_crawler_data_path: "{{ nc_install_dir }}/alert-crawler/data"
   ```
2. Add a tasks/alert-crawler.yml or extend tasks/main.yml to:
   - Create the data directory with `owner: "1000"`, `group: "1000"`, `mode: "0750"`.
   - Template the timer and service units to `/etc/systemd/system/`.
   - `daemon_reload`.
   - Enable and start the timer.

**Files**:
- `../northcloud-ansible/roles/north-cloud/defaults/main.yml` (modified, +~10 lines).
- `../northcloud-ansible/roles/north-cloud/tasks/alert-crawler.yml` (new, ~50 lines) OR extend `tasks/main.yml`.

### T097 — Data dir creation with owner: "1000" (PR-004 mitigation)

**Purpose**: Avoid the uid mismatch that causes "unable to open database file" failures (per repo CLAUDE.md "Non-root container volume ownership" gotcha).

**Steps**:
1. In the Ansible task added in T096:
   ```yaml
   - name: Create alert-crawler data directory
     ansible.builtin.file:
       path: "{{ nc_alert_crawler_data_path }}"
       state: directory
       owner: "1000"   # CRITICAL: container user uid, NOT deploy_user (uid 1001).
       group: "1000"
       mode: "0750"
     become: true
   ```
2. Document in the task block comment why `owner: "1000"` is required (link to repo CLAUDE.md gotcha section).
3. Test on a staging host: deploy, then `ls -la /home/deployer/north-cloud/alert-crawler/data` should show uid 1000.

**Files**:
- `../northcloud-ansible/roles/north-cloud/tasks/alert-crawler.yml` (already from T096).

**Validation**:
- `ansible-playbook --check -i inventory site.yml` clean (dry-run).
- On staging: `systemctl status alert-crawler.timer` shows the timer active; first invocation succeeds and creates the SQLite file.

## Definition of Done

- Timer template parameterized via `nc_alert_crawler_schedule`.
- Service template `Type=oneshot` running `docker compose run --rm`.
- Default variable in `defaults/main.yml`.
- Data dir created with `owner: "1000"`.
- Ansible playbook applies cleanly on staging.

## Risks

- **PR-004 PR-004**: addressed directly via T097.
- **Cross-repo coordination**: this WP commits in `northcloud-ansible`. Mark the WP done after the PR merges in that repo.
- **Production deployment timing**: the systemd unit only activates at the next OnCalendar tick after deploy. First production poll may be up to 1h delayed; this is expected.

## Reviewer Guidance

- Verify `owner: "1000"` is set explicitly.
- Verify the timer cadence default is `*-*-* *:30:00 UTC` (hourly at :30).
- Verify the service unit references `docker compose run --rm` (matches signal-crawler).
- Verify `nc_install_dir` is used (not hardcoded paths).

## Implementation Command

```bash
spec-kitty agent action implement WP22 --agent <name>
```

Depends on WP05, WP20.

## Activity Log

- 2026-05-07T13:36:12Z – claude:sonnet:implementer:implementer – shell_pid=523843 – Started implementation via action command
- 2026-05-07T13:37:30Z – claude:sonnet:implementer:implementer – shell_pid=523843 – Ready for review: northcloud-ansible commit ee575c2 adds timer/service templates, defaults, and uid-1000 data-dir task for alert-crawler
