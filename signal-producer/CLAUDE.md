# CLAUDE.md — signal-producer

> **Read first:** [`docs/specs/lead-pipeline.md`](../docs/specs/lead-pipeline.md)
> defines the shared signal schema, threshold contract, and dedup strategy across
> all producers. Mission spec:
> [`kitty-specs/signal-producer-pipeline-01KQ6QZS/spec.md`](../kitty-specs/signal-producer-pipeline-01KQ6QZS/spec.md).

## Role

Stateless one-shot binary that drains classified content from Elasticsearch
(`*_classified_content`, `quality_score >= 40`, `content_type in {rfp,
need_signal}`), maps each hit to the Waaseyaa `/api/signals` schema, and POSTs
the batch. The only persistent state is a small checkpoint file that records
the last successful `crawled_at` watermark.

This is the canonical successor to `signal-crawler/` for ES-derived signals.
External-source adapters (HN, funding, jobs) stay in `signal-crawler/` until
that service's MIGRATION.md retires them.

## Architecture

Three internal packages, downward-only dependency DAG:

```
internal/
  client/    Elasticsearch read client + Waaseyaa POST client
  mapper/    ESHit -> Signal translation (rfp + need_signal variants)
  producer/  Orchestrator: load checkpoint -> query ES -> map -> POST -> advance checkpoint
cmd/         Thin main: load config, wire dependencies, run one cycle, exit
```

`producer` imports `mapper` and `client`. `mapper` and `client` are independent
leaves. `cmd` imports `producer` only. See `.layers` for the enforced mapping.

## Development

```bash
task signal-producer:build          # Build binary
task signal-producer:test           # Run tests
task signal-producer:lint           # golangci-lint
task signal-producer:vuln           # govulncheck
```

## Deployment posture

One-shot Docker container managed by an Ansible-installed systemd timer (same
pattern as signal-crawler). No long-running process, no docker-compose service.
Checkpoint file lives on a host bind-mount at the path configured in
`checkpoint.file` (default `/var/lib/signal-producer/checkpoint.json`).

## Out of scope (do not add here)

- The Waaseyaa receiver (`POST /api/signals`) — owned by the Waaseyaa repo.
- Enrichment of signals after delivery — owned by the downstream enrichment
  service.
- External-source adapters (HN, funding, jobs scrapers) — stay in
  `signal-crawler/` per its MIGRATION.md.

The runbook entry will land alongside WP06.
