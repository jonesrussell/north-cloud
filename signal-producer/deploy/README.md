# signal-producer deploy

The unit and timer files in this directory are the source of truth for
the production install, but they are NOT applied by this repo's
`scripts/deploy.sh`. They are consumed by Ansible.

## Deployment is Ansible-managed

Production install of signal-producer is handled by the `north-cloud`
role in the [`jonesrussell/northcloud-ansible`](https://github.com/jonesrussell/northcloud-ansible)
repository. Specifically, Ansible owns:

- Placing the binary at `/usr/local/bin/signal-producer`.
- Installing `signal-producer.service` and `signal-producer.timer` into
  `/etc/systemd/system/`.
- Creating the dedicated `signal-producer` system user and group.
- Provisioning `/etc/signal-producer/env` (mode `0600`) from
  `env.example` and populating `WAASEYAA_API_KEY` from the secrets
  store.
- Running `systemctl daemon-reload` and `systemctl enable --now
  signal-producer.timer`.

The unit files (`signal-producer.service`, `signal-producer.timer`),
`env.example`, and this `README.md` in this directory are the canonical
source of truth. Ansible copies them verbatim — edits here are how you
change production behaviour, but only after a corresponding Ansible run.

This repo's `scripts/deploy.sh` only registers signal-producer in the
deploy health-check skip list (it has no HTTP endpoint). It does NOT
install or modify any host-level systemd state.

### First-deploy gating

Before the first north-cloud deploy that includes signal-producer
reaches production, a corresponding `northcloud-ansible` PR must land
and the playbook must run on the target host. Otherwise the binary
shipped in the deploy tarball is never linked into `/usr/local/bin`,
the systemd unit is never installed, and the timer is never enabled —
the producer will silently no-op.

## Files

- `signal-producer.service` — systemd `Type=oneshot` unit; runs as the
  `signal-producer` system user; reads env from `/etc/signal-producer/env`;
  state lives in `/var/lib/signal-producer/` (managed by `StateDirectory=`).
- `signal-producer.timer` — fires every 15 minutes (`OnCalendar=*:0/15`)
  with boot-time catch-up (`Persistent=true`).
- `env.example` — template Ansible uses to seed `/etc/signal-producer/env`
  on first install. Redeploys never overwrite an existing env file.
  `WAASEYAA_API_KEY` must be populated from 1Password by Ansible (or by
  hand on first bring-up).

## Post-deploy verification ritual

The commands below assume Ansible has already run on the target host.
If `systemctl status signal-producer.timer` reports `Loaded:
not-found`, the Ansible playbook has not been applied yet — fix that
before troubleshooting north-cloud.

Run on the VPS after a deploy that includes signal-producer changes:

```bash
sudo systemctl status signal-producer.timer
sudo systemctl list-timers | grep signal-producer
sudo journalctl -u signal-producer --since "20 minutes ago"
```

Expect the first run within 15 minutes; success is a `run_summary` log
line. If the first run fails because the env file is unpopulated,
populate it from 1Password and trigger a manual run:

```bash
sudo systemctl start signal-producer.service
```

For triage recipes (source-down, force-rewind, key rotation), see
[`docs/RUNBOOK.md` § Signal Producer](../../docs/RUNBOOK.md#signal-producer).
