---
work_package_id: WP06
title: Production Deployment
dependencies:
- WP05
requirement_refs:
- FR-017
- FR-018
- FR-019
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T026
- T027
- T028
- T029
- T030
agent: "claude:opus-4.7:implementer:implementer"
shell_pid: "26404"
history:
- event: created
  at: '2026-04-27T05:55:00Z'
  by: /spec-kitty.tasks
authoritative_surface: signal-producer/deploy/
execution_mode: code_change
mission_id: 01KQ6QZSNBQF3VW515AKM0SQ46
mission_slug: signal-producer-pipeline-01KQ6QZS
owned_files:
- signal-producer/deploy/signal-producer.service
- signal-producer/deploy/signal-producer.timer
- signal-producer/deploy/env.example
- signal-producer/deploy/README.md
- scripts/deploy.sh
- docs/RUNBOOK.md
- .github/workflows/deploy.yml
tags: []
---

# WP06 — Production Deployment

## Objective

Ship the signal-producer to the production VPS via the existing deploy pipeline. Install the systemd unit and timer; verify a successful first run via journald. After this WP merges and CI deploys, signals start flowing to Waaseyaa every 15 minutes.

## Context

Read first:
- [spec.md](../spec.md) FR-017 (timer), FR-018 (deployment + checkpoint location), FR-019 (source-down alert), C-007 (no docker-compose for the producer), C-010 (file mode 0640).
- [plan.md](../plan.md) "Deployment" row of the Charter Check, plus deploy risks in the premortem.
- [research.md](../research.md) D7 (source-down via WARN log + journald grep), D8 (`StateDirectory=` + dedicated user).
- [quickstart.md](../quickstart.md) — operator sections (running locally, journald, force-rewind, deploy).
- Root `CLAUDE.md` "Production Deployment" + "Oneshot Docker services" + "New Go service checklist".
- `scripts/deploy.sh` and `.github/workflows/deploy.yml` — the existing pipeline you are extending.

Charter constraints: C-005 (conventional commit, no `--no-verify`), C-009 (drift + layers + ports green).

## Branch Strategy

Planning base: `main`. Merge target: `main`. Lane workspace from `lanes.json`. Sequential — depends on WP05 (a runnable binary must exist). After this WP merges and CI deploys, the operator verifies a real run on the VPS.

## Subtask Guidance

### T026 — `signal-producer.service` unit

**Purpose**: One-shot systemd unit that runs the binary as a dedicated user with a managed state directory.

**Steps**:

1. Create `signal-producer/deploy/signal-producer.service`:
   ```ini
   [Unit]
   Description=North Cloud Signal Producer
   After=network-online.target
   Wants=network-online.target

   [Service]
   Type=oneshot
   User=signal-producer
   Group=signal-producer
   StateDirectory=signal-producer
   StateDirectoryMode=0750
   EnvironmentFile=/etc/signal-producer/env
   ExecStart=/usr/local/bin/signal-producer
   StandardOutput=journal
   StandardError=journal
   SyslogIdentifier=signal-producer
   # Defensive timeouts so a stuck run doesn't block the next timer fire.
   TimeoutStartSec=10m

   [Install]
   WantedBy=multi-user.target
   ```
2. Decisions captured in this unit:
   - `Type=oneshot` (research D8) — no daemon, no overlapping fires.
   - `StateDirectory=signal-producer` creates `/var/lib/signal-producer/` with the right ownership automatically (D8).
   - `EnvironmentFile=` reads from `/etc/signal-producer/env` (root:root 0600).
   - `User=`/`Group=` create implicit isolation. The deploy script (T028) creates the user.
3. Create `signal-producer/deploy/env.example` documenting the env file:
   ```
   # Place at /etc/signal-producer/env, owned root:root, mode 0600.
   WAASEYAA_URL=https://northops.ca
   WAASEYAA_API_KEY=replace-me-from-1password
   ES_URL=http://localhost:9200
   ```

**Files**: `signal-producer/deploy/signal-producer.service`, `signal-producer/deploy/env.example`.

**Validation**:

- [ ] `systemd-analyze verify signal-producer/deploy/signal-producer.service` passes (run on a Linux VM if not on the dev box).
- [ ] Unit references the correct binary path and env file.

### T027 — `signal-producer.timer` unit

**Purpose**: Schedule the unit every 15 minutes, with catch-up on missed boots.

**Steps**:

1. Create `signal-producer/deploy/signal-producer.timer`:
   ```ini
   [Unit]
   Description=North Cloud Signal Producer Timer

   [Timer]
   OnCalendar=*:0/15
   Persistent=true
   AccuracySec=30s

   [Install]
   WantedBy=timers.target
   ```
2. Defaults explained in a comment block above:
   - `OnCalendar=*:0/15` fires at HH:00, HH:15, HH:30, HH:45 (FR-017).
   - `Persistent=true` runs a catch-up on boot if the timer was missed (recovery scenario in spec).
   - `AccuracySec=30s` keeps the timer drift bounded; the producer is fast enough that this is generous.
   - Default systemd behavior already prevents starting a second instance while the first is active (because `Type=oneshot`).

**Files**: `signal-producer/deploy/signal-producer.timer`.

**Validation**:

- [ ] `systemd-analyze verify signal-producer/deploy/signal-producer.timer` passes.
- [ ] `systemctl list-timers` (after install) shows the timer with the next fire time at the next 15-minute boundary.

### T028 — Deploy script and GH Actions updates

**Purpose**: Get the binary onto the VPS, install the unit + timer, and enable.

**Steps**:

1. Edit `scripts/deploy.sh`:
   - Add a section that, when the deploy artifact contains `signal-producer/`, performs:
     ```bash
     # Create user/group if absent (idempotent via getent).
     getent group signal-producer >/dev/null || groupadd --system signal-producer
     getent passwd signal-producer >/dev/null || useradd --system --gid signal-producer --shell /usr/sbin/nologin --no-create-home signal-producer

     # Copy binary, unit files, env example.
     install -m 0755 -o root -g root signal-producer/signal-producer /usr/local/bin/signal-producer
     install -m 0644 -o root -g root signal-producer/deploy/signal-producer.service /etc/systemd/system/signal-producer.service
     install -m 0644 -o root -g root signal-producer/deploy/signal-producer.timer /etc/systemd/system/signal-producer.timer

     # First-time env file setup (do not overwrite existing).
     if [ ! -f /etc/signal-producer/env ]; then
       install -d -m 0750 -o root -g root /etc/signal-producer
       install -m 0600 -o root -g root signal-producer/deploy/env.example /etc/signal-producer/env
       echo "WARNING: /etc/signal-producer/env populated from example; rotate secrets!"
     fi

     systemctl daemon-reload
     systemctl enable --now signal-producer.timer
     ```
2. Add `signal-producer` to the deploy health-check **skip list** (root `CLAUDE.md` notes oneshot services are skipped) — this is a `Type=oneshot` unit and isn't subject to docker-compose-style health checks.
3. Edit `.github/workflows/deploy.yml`:
   - The CI builds the `signal-producer` binary as part of the tarball (the existing pipeline picks up `signal-producer/` because T003 added it to `GO_SERVICES`). Verify the binary path is `signal-producer/signal-producer` and that the deploy script picks it up.
   - Add `signal-producer/deploy/*.service` and `signal-producer/deploy/*.timer` to the file paths included in the deploy tarball.
4. Coordinate with the existing nginx/Caddy config — no change needed (the producer doesn't expose HTTP).

**Files**: `scripts/deploy.sh`, `.github/workflows/deploy.yml`.

**Validation**:

- [ ] Re-running deploy is idempotent (user/group/file install commands all use `--system` flags or guard with `getent`).
- [ ] First-time deploy creates `/etc/signal-producer/env` from the example with mode 0600 and a warning log.
- [ ] Subsequent deploys do NOT overwrite the env file.
- [ ] Health-check skip list updated.

### T029 — RUNBOOK source-down + force-rewind

**Purpose**: Operational documentation per research D7 and the FR-019 requirement that the alert be operator-actionable.

**Steps**:

1. Edit `docs/RUNBOOK.md`. Add a new section `## Signal Producer`:
   - **First-run smoke test** (post-deploy, exact commands).
   - **Source-down triage** (the journald grep recipes from `quickstart.md`, fleshed out with example outputs).
   - **Force a checkpoint rewind** (the recipe from `quickstart.md`, with safety notes about Waaseyaa-side dedup as the backstop against duplicate leads).
   - **Failed-run triage** (status, journald, common error patterns).
   - **Rotating the API key** (stop timer → update env file → systemctl daemon-reload → start timer → verify next run).
2. Cross-link from `signal-producer/CLAUDE.md` to the new RUNBOOK section.

**Files**: `docs/RUNBOOK.md`.

**Validation**:

- [ ] Section exists, ≤ 200 lines, includes all five subsections above.
- [ ] Commands are paste-able (no placeholders the operator must guess).

### T030 — Health-check skip-list and first-run smoke test

**Purpose**: Final operational hookup. Tell the deploy pipeline this isn't a long-running service, and document the post-deploy verification ritual.

**Steps**:

1. In `scripts/deploy.sh` (already edited in T028), confirm `signal-producer` is on the oneshot/skip list. Cross-reference root `CLAUDE.md` "Oneshot Docker services" entry — though signal-producer is host-systemd not docker-compose, the same principle (skip the http health check) applies.
2. Add a brief section to `signal-producer/deploy/README.md` documenting:
   ```markdown
   # signal-producer deploy

   Deploys via the standard CI pipeline. After deploy, verify on the VPS:

   ```bash
   sudo systemctl status signal-producer.timer
   sudo systemctl list-timers | grep signal-producer
   sudo journalctl -u signal-producer --since "20 minutes ago"
   ```

   Expect the first run within 15 minutes; success is a `run_summary` log line.
   ```

**Files**: `signal-producer/deploy/README.md`.

**Validation**:

- [ ] README exists; ≤ 50 lines.
- [ ] Skip-list entry exists in `scripts/deploy.sh`.
- [ ] Post-deploy verification ritual documented.

## Definition of Done

- [ ] All five subtasks complete with their validation checklists ticked.
- [ ] `systemd-analyze verify` passes on both unit files.
- [ ] `task drift:check`, `task layers:check`, `task ports:check` all green.
- [ ] No files modified outside `owned_files` (root CLAUDE.md is intentionally NOT touched here; if a doc update is needed, file a follow-up).
- [ ] Conventional commit; lefthook hooks pass.

## Reviewer Guidance

1. **`Type=oneshot` and timer hygiene**: confirm both unit files. Reject if `Type=simple` or if the timer overlaps via parallel fires.
2. **`StateDirectory=`**, not manual `chown`: the unit MUST use `StateDirectory=signal-producer` for `/var/lib/signal-producer/`. Reject manual `mkdir`/`chown` in the deploy script — that path is fragile.
3. **Env file safety**: deploy script MUST NOT overwrite an existing `/etc/signal-producer/env`. The first-time installation writes from the example with a clear "rotate secrets" warning. Reject if a redeploy stomps on a populated env file.
4. **Idempotent user/group creation**: `getent` guards before `groupadd`/`useradd`. Reject if the script crashes on a re-run because the user already exists.
5. **Health-check skip list updated**: confirm `signal-producer` appears in whichever list `deploy.sh` uses to skip post-deploy HTTP health checks.
6. **No HTTP endpoint exposed**: confirm the producer binary doesn't open a listening port. Reject if any `net.Listen` call appears.
7. **Runbook is paste-able**: try the source-down grep on a fictional journald output. If the operator has to invent any value, the recipe is broken.

## Risks and Mitigations

| Risk                                                                                                | Mitigation                                                                                                                                                            |
| --------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| First-time deploy: `signal-producer` user does not exist on the VPS.                                | `useradd --system` in the deploy script. Idempotent.                                                                                                                  |
| First-time deploy: `/etc/signal-producer/env` empty causes the binary to exit with "API_KEY missing". | Acceptable. The deploy log warns the operator to populate the file. The next timer fire fails until populated; producer fails fast (T023 validates).                   |
| `systemctl daemon-reload` not picked up.                                                            | Explicit call in the deploy script after copying unit files.                                                                                                          |
| Timer overlap if a previous run is stuck.                                                           | `Type=oneshot` + `TimeoutStartSec=10m` prevents indefinite stuck runs. Systemd refuses to start a second copy while the first is active.                              |
| Operator forgets to rotate the API key after first deploy.                                          | First-deploy log line shouts "WARNING: rotate secrets!"; runbook documents the rotation procedure.                                                                    |
| Drift check rejects new doc paths.                                                                  | `task drift:check` was already validated by WP01 (which added the spec section). Confirm one more time after WP06's edits.                                            |

## Post-Merge Operator Verification (manual, outside the WP)

After this WP merges and CI deploys, perform on the VPS:

```bash
ssh prod
sudo systemctl status signal-producer.timer
sudo systemctl list-timers | grep signal-producer
# Wait up to 15 min for the first fire, then:
sudo journalctl -u signal-producer --since "20 minutes ago" | grep run_summary
```

Expect at least one `run_summary` line within the first 30 minutes after deploy. If the first run failed because the env file is unpopulated, populate it from 1Password and trigger a manual run:

```bash
sudo systemctl start signal-producer.service
```

This verification is the mission's exit criterion (Success Criterion 1).

## Implementation Command

```bash
spec-kitty agent action implement WP06 --agent <agent-name> --mission signal-producer-pipeline-01KQ6QZS
```

## Activity Log

- 2026-04-27T07:24:10Z – claude:opus-4.7:implementer:implementer – shell_pid=22524 – Started implementation via action command
- 2026-04-27T07:28:42Z – claude:opus-4.7:implementer:implementer – shell_pid=22524 – Production deployment ready for review
- 2026-04-27T07:30:39Z – claude:opus-4.7:reviewer:reviewer – shell_pid=34024 – Started review via action command
- 2026-04-27T07:31:22Z – claude:opus-4.7:reviewer:reviewer – shell_pid=34024 – Moved to planned
- 2026-04-27T07:31:32Z – claude:opus-4.7:implementer:implementer – shell_pid=26404 – Started implementation via action command
- 2026-04-27T07:33:55Z – claude:opus-4.7:implementer:implementer – shell_pid=26404 – Cycle 2: systemd install moved to Ansible scope
