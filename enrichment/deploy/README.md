# enrichment deploy

The files in this directory are the production source of truth for the
enrichment service host install. They are not applied by this repository's
Docker deploy flow yet; the intended consumer is the `north-cloud` role in
the sibling `northcloud-ansible` repository.

## Deployment is Ansible-managed

Production install should be handled by `northcloud-ansible`, not by edits in
Waaseyaa and not by manually copying files during a release. The companion
Ansible change should own:

- Placing the compiled binary at `/usr/local/bin/enrichment`.
- Installing `enrichment.service` into `/etc/systemd/system/`.
- Creating the dedicated `enrichment` system user and group.
- Provisioning `/etc/enrichment/env` from `env.example`.
- Running `systemctl daemon-reload` and `systemctl enable --now
  enrichment.service`.

Callback URL and API key values are not host-level environment variables for
this service. They are supplied on each `POST /api/v1/enrich` request and are
forwarded only to that request's callback.

## Files

- `enrichment.service` - systemd `Type=simple` unit; runs as the
  `enrichment` system user and reads runtime config from `/etc/enrichment/env`.
- `env.example` - template for `/etc/enrichment/env`; contains only service
  bind, timeout, and Elasticsearch settings.

## Runtime configuration

| Variable | Default | Notes |
| --- | --- | --- |
| `ENRICHMENT_HOST` | `0.0.0.0` | Bind host for the HTTP API. |
| `ENRICHMENT_PORT` | `8095` | Bind port for `/health` and `/api/v1/enrich`. |
| `ENRICHMENT_READ_TIMEOUT` | `5s` | `net/http` read timeout. |
| `ENRICHMENT_WRITE_TIMEOUT` | `10s` | `net/http` write timeout and callback request timeout. |
| `ENRICHMENT_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown deadline. |
| `ELASTICSEARCH_URL` | `http://localhost:9200` | Primary Elasticsearch URL. `ES_URL` is accepted as a fallback. |

## First-deploy gating

Before the first north-cloud deploy that includes enrichment reaches
production, a matching `northcloud-ansible` change must land and the playbook
must run on the target host. Otherwise the binary can exist in the repo release
without a systemd unit, env file, service user, or enabled listener.

## Post-deploy verification

Run on the production host after Ansible has installed the service:

```bash
sudo systemctl status enrichment.service
curl -fsS http://127.0.0.1:8095/health
sudo journalctl -u enrichment -n 100 --no-pager
```

Expected health response:

```json
{"status":"ok","service":"enrichment"}
```

To smoke-test request acceptance without depending on Waaseyaa live mode, use a
temporary callback endpoint controlled by the operator:

```bash
curl -fsS -X POST http://127.0.0.1:8095/api/v1/enrich \
  -H 'Content-Type: application/json' \
  -d '{
    "signal_id": "deploy-smoke",
    "tenant_id": "ops",
    "enrichers": ["company_intel"],
    "callback_url": "https://example.invalid/callback",
    "callback_api_key": "not-a-real-secret",
    "payload": {"organization_name": "North Cloud"}
  }'
```

A `202 Accepted` response confirms the HTTP contract and async queue handoff.
Callback delivery depends on the callback endpoint supplied in the request.

