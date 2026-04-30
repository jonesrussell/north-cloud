# Enrichment service deployment

The enrichment service deploys as a host-level systemd service managed by the
`north-cloud` role in `northcloud-ansible`.

Canonical artifacts live in:

- `enrichment/deploy/enrichment.service`
- `enrichment/deploy/env.example`
- `enrichment/deploy/README.md`
- `enrichment/docs/deployment.md`

The service listens on `ENRICHMENT_HOST:ENRICHMENT_PORT` and reads
Elasticsearch from `ELASTICSEARCH_URL` or `ES_URL`. Callback URL and callback
API key are intentionally request fields, not deployment configuration.

First production rollout is blocked on a companion `northcloud-ansible` change
that installs the binary, unit, env file, service user, and health check. No
Waaseyaa repository changes are required for this north-cloud mission.

