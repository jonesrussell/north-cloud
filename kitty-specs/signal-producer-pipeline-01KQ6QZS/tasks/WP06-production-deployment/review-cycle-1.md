# Review Feedback — WP06 (cycle 1)

## Critical Issues (must fix)

**The systemd install logic belongs in Ansible, not in `scripts/deploy.sh`.** Per the convention documented in root `CLAUDE.md` under "Oneshot Docker services":

> ...manage the systemd timer via Ansible (northcloud-ansible repo, north-cloud role).

`signal-crawler` (the closest analog) follows this pattern. The current WP06 commit `eb4311e9` puts user creation, binary install, unit install, `daemon-reload`, and `enable --now` into `scripts/deploy.sh` — that violates the convention and creates a dual-install path (Ansible AND deploy.sh both managing the same systemd state) that will drift.

### What to remove from `scripts/deploy.sh`

Strip the entire "Step 3.6: Install host-level systemd services" block (the section you added). Specifically: the `getent`/`groupadd`/`useradd` guards, the `install -m 0755 ... /usr/local/bin/signal-producer`, the unit-file installs, the env-file first-time-setup, and the `systemctl daemon-reload && systemctl enable --now`. None of that should live in deploy.sh.

### What to KEEP in `scripts/deploy.sh`

The skip-list entry you added (the `signal-producer)` case after `signal-crawler)` that prints the "host systemd timer, no HTTP endpoint" message). That part is correct — deploy.sh DOES need to know to skip the HTTP health check for this oneshot service.

### What to KEEP in `signal-producer/deploy/`

The unit files (`signal-producer.service`, `signal-producer.timer`), `env.example`, and `README.md` are correct as the source of truth. Ansible will copy them from the deploy artifact.

### What to UPDATE in `signal-producer/deploy/README.md`

Add a section "## Deployment is Ansible-managed" documenting:

- Production install of the binary, systemd unit + timer, dedicated user, and env file is handled by the `north-cloud` role in the `jonesrussell/northcloud-ansible` repo.
- This repo's deploy.sh only handles the health-check skip; it does NOT install systemd state.
- A new mission/PR in `northcloud-ansible` is required for first-deploy enablement (a follow-up task has been filed in the orchestrator's session).
- The unit and timer files in this directory are the source of truth — Ansible copies them.

### What to update in `.github/workflows/deploy.yml`

The build step that produces a Linux amd64 host binary at `signal-producer/signal-producer` is correct — keep it. Ansible needs the binary to be present in the deploy tarball. But: confirm there is no longer any post-deploy `systemctl` invocation for signal-producer in the workflow (none was added by your cycle-1 commit, but double-check).

### `docs/RUNBOOK.md` Signal Producer section

Keep the section as-is, with one addition: under "First-run smoke test" or as a new bullet at the top, add a note that production install happens via Ansible (`northcloud-ansible` repo), and that the smoke-test commands assume a successful Ansible run.

## Should Fix

- **`signal-producer/deploy/README.md` post-deploy verification**: the current README's `systemctl status signal-producer.timer` recipe is fine, but it should explicitly point at "after running the Ansible playbook against prod" rather than "after running the CI deploy".

## Nice to Have

- A reference to the spawned follow-up task / PR in `northcloud-ansible` from the README (when it lands).
- A grep recipe in RUNBOOK to spot a misconfigured Ansible run (e.g., `systemctl status signal-producer.timer | grep -i loaded` should return `loaded`).

## Notes for the Re-implementation

- Cycle-2 commit should be relatively small: delete the install block from deploy.sh (keep skip-list), update README.md (add Ansible-managed section), small RUNBOOK touch-up.
- Do NOT touch the unit files, env.example, or workflow steps that build the binary.
- The companion `northcloud-ansible` PR is a separate mission outside this repo (already filed as a follow-up task by the orchestrator).
