# Enrichment deployment

The enrichment service is a Go HTTP API built from `enrichment/cmd`. It exposes
`GET /health` and accepts enrichment jobs at `POST /api/v1/enrich`. Jobs are
acknowledged with `202 Accepted`, processed asynchronously, and delivered to the
per-request callback URL with the per-request API key.

## Host contract

The production host contract is systemd plus an environment file:

- Binary: `/usr/local/bin/enrichment`
- Unit: `/etc/systemd/system/enrichment.service`
- Environment: `/etc/enrichment/env`
- User/group: `enrichment:enrichment`
- Health endpoint: `http://127.0.0.1:8095/health`

`enrichment/deploy/` contains the unit and env template that Ansible should
consume. The service intentionally does not define Waaseyaa callback settings as
host environment. Callback URL and API key are external request inputs.

## Required Ansible companion work

Do the companion work in `northcloud-ansible`; do not edit Waaseyaa from this
mission. The Ansible PR should:

1. Build or copy the `enrichment` binary to `/usr/local/bin/enrichment`.
2. Install `enrichment/deploy/enrichment.service`.
3. Create `/etc/enrichment/env` from `enrichment/deploy/env.example` when
   missing.
4. Ensure the `enrichment` user and group exist.
5. Enable and start `enrichment.service`.
6. Add a health check against `http://127.0.0.1:8095/health`.

## Local validation

```bash
cd enrichment
task build
task test
task test:race
task lint
task vuln
```

`task lint` requires `golangci-lint`. `task vuln` requires `govulncheck`.
Production smoke testing additionally requires systemd and the Ansible install
path, so it is host-side validation rather than a Windows-local check.

## Rollback

The enrichment service is stateless. Rollback is a binary and unit rollback:

```bash
sudo systemctl stop enrichment.service
sudo install -m 0755 /path/to/previous/enrichment /usr/local/bin/enrichment
sudo systemctl daemon-reload
sudo systemctl start enrichment.service
curl -fsS http://127.0.0.1:8095/health
```

Failed callback delivery does not persist local state. Callers can resubmit the
same `signal_id` if the downstream callback receiver needs a replay.

