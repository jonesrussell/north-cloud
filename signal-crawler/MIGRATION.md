# signal-crawler — successor direction

> Read alongside [`docs/specs/lead-pipeline.md`](../docs/specs/lead-pipeline.md).

## Status

**signal-crawler is in maintenance mode.** It continues to run during the transition, but it is not the place where new adapter work lands.

## Successor

The successor is **signal-producer** — the binary specified by the Lead Intelligence Integration milestone (issue #592). signal-producer posts to Waaseyaa (`${WAASEYAA_URL}/api/signals`) using the same wire format this service uses today, with checkpoint persistence and an enrichment-callback pathway that this service does not have.

New adapters land in signal-producer. Procurement sources do not go to either service — they extend the `PortalParser` interface in `rfp-ingestor` instead.

See `docs/specs/lead-pipeline.md` for the producer catalogue, the shared signal schema, the threshold and attribution contracts, and the per-producer dedup keys.

## What still runs here

The following adapters keep running in signal-crawler during the transition. Treat them as examples rather than canonical data sources; none of them should be copied or promoted into signal-producer without a separate decision.

| Adapter | Why it stays here | Next step |
|---|---|---|
| `adapter/hn` (HN general) | ICP-mismatched — not part of the prospect engine's target segments | No promotion. Retire when signal-producer replaces the adapter harness. |
| `adapter/jobs/hnhiring` | ICP-mismatched | No promotion. |
| `adapter/jobs/wwr` (WeWorkRemotely) | ICP-mismatched | No promotion. |
| `adapter/jobs/remoteok` | ICP-mismatched | No promotion. |
| `adapter/jobs/gcjobs` | Marginal for senior-engineering-gap detection; currently blocked by source-side IP filtering | No promotion. |
| `adapter/jobs/workbc` | Marginal | No promotion. |
| `adapter/funding/otf` | On-ICP but single-source; broader funding coverage lands in signal-producer | Reimplement in signal-producer when funding adapters land there. |

## What to do when modifying this service

1. Bug fixes and minor adapter tweaks are fine in-place.
2. New adapters — do not add here. Open an issue under the Lead Intelligence Integration milestone and land them in signal-producer.
3. Changes to the wire format, threshold gate, attribution rules, or dedup key must reference `docs/specs/lead-pipeline.md` and may need a coordinated change in signal-producer and Waaseyaa.
4. When signal-producer fully covers the canonical adapters, this service is retired — not on a calendar, but on coverage parity.

## References

- Spec: `docs/specs/lead-pipeline.md`
- Successor: issue #592 (signal-producer binary) and the surrounding Lead Intelligence Integration milestone
- Prospect engine overview: `docs/prospect-engine-plan.md`
