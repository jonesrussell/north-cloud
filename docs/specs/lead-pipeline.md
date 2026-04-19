# Lead / Signal Pipeline Spec

> Last verified: 2026-04-19 (umbrella for signal-crawler, classifier `need_signal`, publisher `/api/leads`, rfp-ingestor, and the Lead Intelligence successor architecture; MIGRATION.md in signal-crawler is already linked from §6)

## Overview

This spec is the single source of truth for the **lead and signal data plane** across North Cloud. It defines the vocabulary, the producer catalogue, the shared schema, the dedup contract, and the deprecation trajectory so that changes to any one path do not silently diverge from the others.

Read this first before editing:

- `signal-crawler/`
- `classifier/internal/classifier/need_signal_extractor.go`
- `classifier/internal/classifier/content_type_need_signal_heuristic.go`
- `publisher/internal/api/leads_export_handler.go`
- `publisher/internal/models/claudriel_lead.go`
- `rfp-ingestor/` (procurement signals — see also `docs/specs/rfp-ingestor.md`)
- any new component under `infrastructure/signal/`

The downstream prospect engine (ranker, briefing, feedback loop) is specified in `docs/prospect-engine-plan.md`. This spec defines the upstream data plane only.

---

## Vocabulary

| Term | Definition |
|---|---|
| **Signal** | A time-bound fact suggesting a prospect is open to an engagement — a funding announcement, a senior-engineering gap, an RFP publication, a tech-migration post, etc. Signals are the atomic input; they are not yet qualified prospects. |
| **Lead** | A signal (or cluster of signals) plus enrichment, optionally qualified by a human or ranker. Not all signals become leads. |
| **Need signal** | A signal **derived from classified_content by the classifier** (`NeedSignalExtractor`). Distinguished from externally sourced signals to make the path of origin unambiguous. |
| **Prospect** | A lead surfaced in a daily briefing, typed with ranker score and ICP segments. Defined in the prospect engine plan, not here. |
| **Producer** | A service or binary that writes signals into the data plane (signal-crawler, classifier, rfp-ingestor, the forthcoming signal-producer). |
| **Consumer** | Anything that reads signals — Waaseyaa (via HTTP POST), the publisher leads endpoint (for RFPs), the enrichment service, Claudriel. |
| **Canonical producer** | The producer a new adapter should land in. See §5. |

---

## File Map

```
signal-crawler/
  cmd/signal-crawler/main.go          # Oneshot binary, cron-scheduled
  internal/adapter/                   # adapter.Source implementations (hn, funding, jobs/*)
  internal/adapter/adapter.go         # Signal struct (matches Waaseyaa /api/signals)
  internal/scoring/scoring.go         # ScoreDirectAsk / ScoreStrongSignal / ScoreWeakSignal
  internal/dedup/                     # SQLite (`data/seen.db`)
  internal/northops/client.go         # POST ${NORTHOPS_URL}/api/signals
  MIGRATION.md                        # Successor direction (see §6)

classifier/internal/classifier/
  need_signal_extractor.go            # NeedSignalExtractor, SignalType* constants
  content_type_need_signal_heuristic.go  # 2-keyword / 0.80-confidence gate
  # NeedSignalResult is persisted in ES classified_content under `need_signal`

publisher/internal/
  models/claudriel_lead.go            # ClaudrielLead struct (RFP-shaped)
  api/leads_export_handler.go         # GET /api/leads → Claudriel's NorthCloudLeadFetcher
  database/claudriel_lead_repo.go     # ListClaudrielLeads

rfp-ingestor/                         # Procurement signals
  internal/parser/*.go                # PortalParser implementations (CanadaBuys, SEAO, …)
  # Bulk-indexes to ES rfp_classified_content; see docs/specs/rfp-ingestor.md

infrastructure/signal/                # Shared helpers used by both producers
  threshold.go                        # Unified accept/reject gate (#638 — landed)
  org_normalize.go                    # Normalize / FromEmail / FromURL / Resolve
                                      #   (#639 helper landed; producer wiring pending)
```

---

## Architecture — data flow

```
                           ┌─────────────────────────────────────┐
                           │   Waaseyaa (northops.ca)            │
                           │   POST /api/signals                 │
                           │   POST /api/signals/{id}/enrichments│
                           └────────────▲────────────▲───────────┘
                                        │            │
  ┌──────────────────┐   HTTPS          │            │        HTTPS
  │ signal-crawler   ├──────────────────┘            │    ┌──────────────────┐
  │ (oneshot, cron)  │  (X-Api-Key)                  └────┤ enrichment-svc   │
  │ adapter.Source[] │                                    │ (port 8095)       │
  │ SQLite dedup     │                                    │ enricher handlers │
  └────────┬─────────┘                                    └────────▲─────────┘
           │ (superseded by signal-producer — see §6)              │ HTTPS
           ▼                                                       │
  ┌──────────────────┐    HTTPS                                    │
  │ signal-producer  ├────────────────────────────────────────────┘
  │ (Lead Intel #592)│
  │ checkpoint state │
  └────────┬─────────┘
           │ reads from ES
           │
           ▼
  ┌──────────────────────────────────────────────┐
  │ Elasticsearch                                │
  │   raw_content                                │
  │   {source}_classified_content  ←── classifier│
  │   rfp_classified_content        ←── rfp-ing  │
  └────────▲────────────▲────────────▲───────────┘
           │            │            │
  ┌────────┴────┐ ┌─────┴──────┐ ┌──┴───────────┐
  │ crawler     │ │ classifier │ │ rfp-ingestor │
  │             │ │ need_signal│ │ (bypasses    │
  │             │ │ extractor  │ │  classifier) │
  └─────────────┘ └─────┬──────┘ └──────┬───────┘
                        │               │
                        │               └─▶ publisher Layer 11 → rfp:{…} channels
                        │
                        └─▶ (persisted in classified_content; consumed by prospect engine)

  ┌──────────────────┐    HTTP (Bearer)
  │ publisher        ├──────────────────▶ Claudriel NorthCloudLeadFetcher
  │ GET /api/leads   │  serves Postgres `claudriel_leads` (RFP-shaped bridge)
  └──────────────────┘
```

---

## Producer catalogue

| # | Producer | Canonical? | Origin | Sink | Dedup | Status |
|---|---|---|---|---|---|---|
| 1 | **signal-crawler** | No — **maintenance only** | External scrape adapters (hn, funding, jobs) | Waaseyaa HTTP POST `/api/signals` | SQLite `data/seen.db` per producer key | Superseded by signal-producer. New adapters land in signal-producer, not here. Existing adapters keep running during transition. |
| 2 | **classifier `need_signal`** | Yes | ES `raw_content` → classified_content | Persisted in ES `{source}_classified_content.need_signal` | None at this layer; dedup happens at crawl time | Canonical taxonomy owner for classification-derived signals. |
| 3 | **rfp-ingestor** | Yes | CanadaBuys CSV, SEAO Quebec JSON (extensible via `PortalParser`) | ES `rfp_classified_content` (bypasses classifier) | Per-source URL / document-id hash | Canonical for procurement. New procurement sources extend this, not a new crawler (per prospect-engine-plan §P3). |
| 4 | **signal-producer** (Lead Intel #592) | Yes — future default | ES hits → Waaseyaa HTTP POST | Waaseyaa `/api/signals`; enrichment callbacks to `/api/signals/{id}/enrichments` | Checkpoint persistence (#595) | In flight. Absorbs signal-crawler's role. |
| 5 | **publisher `GET /api/leads`** | N/A — **consumer, not producer** | Postgres `claudriel_leads` (RFP-shaped) | Claudriel `NorthCloudLeadFetcher` via Bearer auth | N/A — read-only bridge | Kept. Not a general lead API. Repurposing would break Claudriel. |

The phrase "a new lead source" should map to exactly one row above. If it does not, the decision is explicitly in scope for an amendment to this spec.

---

## Shared signal schema

Field-by-field contract. Producer columns mark where the field originates; "W" = Waaseyaa POST body, "ES" = classified_content document.

| Field | Type | Required | W (external producers) | ES (classifier) | Definition |
|---|---|---|---|---|---|
| `signal_type` | string enum | yes | yes | yes | `outdated_website`, `funding_win`, `funding_announcement`, `job_posting`, `senior_eng_gap` (extends job_posting), `new_program`, `tech_migration`, `ai_strategy_public`, `ocap_data_sovereignty`, `rfp_published`. Enum is extended in `classifier/internal/classifier/need_signal_extractor.go`; external producers must reference the same list. |
| `external_id` | string | yes | yes | `_id` on the ES doc | Stable identifier from the origin source (URL hash, document id). Used for dedup. |
| `source` | string | yes | yes | yes | Origin handle, e.g. `signal-crawler/funding`, `classifier/need_signal`, `rfp-ingestor/canadabuys`. |
| `label` | string | yes | yes | derived from content `title` | Human-readable summary of the signal. |
| `source_url` | string | yes | yes | yes | Canonical URL where the signal was observed. |
| `strength` | integer 0–100 | yes | yes | mapped from `need_signal.confidence × 100` | Producer-side score before enrichment. |
| `sector` | string | yes | yes | yes | Industry/sector tag. Populated via `#639` attribution work for external producers. |
| `province` | string (2-char) | recommended | yes | yes | Canadian province code; empty for international. |
| `organization_name` | string | recommended | yes | yes | Plain-text org name (see §Organization attribution). |
| `organization_type` | string enum | recommended | yes | yes | `for_profit`, `non_profit`, `government`, `indigenous_community`, `educational`, `unknown`. |
| `funding_status` | string enum | conditional | yes | yes | Required when `signal_type=funding_announcement` or `funding_win`. |
| `notes` | string | optional | yes | yes | Free-text observations. |
| `detected_at` | timestamp | yes | yes | yes | ISO-8601 UTC. |
| `icp.segments[]` | nested | optional | added by enrichment | added by `sector_alignment` classifier component | See prospect-engine-plan §3.4 and §8. Not part of the baseline schema. |

**Backward compatibility:** the Waaseyaa `Signal` struct in `signal-crawler/internal/adapter/adapter.go` is the authoritative wire format for external producers. Adding a required field is a breaking change and must land in coordinated commits across signal-crawler, the signal-producer (#592), and Waaseyaa's HTTP handler. Additive optional fields require a note in this spec and no coordinated deploy.

---

## Threshold and confidence contract

Both the signal-crawler scoring gate and the classifier `need_signal` heuristic must accept or reject the same content. Historical drift between the two is tracked in **#638**.

Unified rule (post-#638):

- A signal qualifies if **≥2 keyword matches** are observed in title + body **and** the derived confidence is **≥ 0.80**.
- The shared implementation lives in `infrastructure/signal/threshold.go`.
- `signal-crawler/internal/scoring/scoring.go` and `classifier/internal/classifier/content_type_need_signal_heuristic.go` both delegate to it.
- Any change to threshold or confidence is a spec change: update this section in the same PR, and add a parameterised unit test exercising the shared helper from both call sites.

Rationale for raising signal-crawler to match the classifier (not lowering): the classifier gate has been tuned against production content for months; signal-crawler's 1-keyword gate produces false positives we have already seen rejected downstream.

---

## Organization attribution contract

Organization attribution is the field most likely to drift across producers because each adapter extracts it differently. **#639** tracks the convergence.

Required behavior (post-#639):

1. **Explicit field** — if the source provides an organization name directly (job posting company field, funding press release entity), use it verbatim.
2. **Email-domain fallback** — if the signal contains a contact email, derive the org from the email's apex domain via `infrastructure/signal/org_normalize.go`.
3. **URL-apex fallback** — if neither exists, derive from the `source_url` apex domain.
4. **Never "unknown"** — an empty `organization_name` is a producer bug; fail the signal with a structured log, do not write an empty string.
5. **Normalization** — `Acme Corporation`, `Acme Corp`, and `acme-corp.com` resolve to the same canonical string for dedup and enrichment lookups.

Dry-run validation: the PR closing #639 must include a sample run against a production day's signals showing ≥80% populated `organization_name` and zero empty strings.

---

## Dedup key strategy

Dedup is **per-producer** because each producer sees the world through its own identifier space; a cross-producer merge happens downstream in Waaseyaa / the prospect engine, not here.

| Producer | Dedup key | Storage | TTL |
|---|---|---|---|
| signal-crawler | SHA-256 of `(adapter_name, external_id)` | SQLite `data/seen.db` | Permanent (append-only) |
| signal-producer (future) | checkpoint on ES `_seq_no` per index (#595) | Service-local KV | Advances; no TTL |
| classifier `need_signal` | ES `_id` (document id) | No explicit store; the ES mapping is the record | Index lifetime |
| rfp-ingestor | `(source, external_id)` hash in-index | ES `rfp_classified_content` | Index lifetime |
| publisher `/api/leads` | `claudriel_leads.id` UUID | Postgres | Row lifetime |

**Cross-producer merge** (downstream, not this spec): Waaseyaa matches signals from multiple producers on `(organization_name_normalized, signal_type, ±7-day window)`. When matched, the higher-strength signal wins and the others attach as evidence. That logic lives in Waaseyaa and is referenced from the prospect engine plan, not here.

---

## Deprecation / migration

See `signal-crawler/MIGRATION.md` for the successor direction (written under #641). In short:

1. **signal-crawler is in maintenance mode.** No new adapters land there.
2. **New adapters land in signal-producer** (Lead Intel #592) and post to Waaseyaa using the same wire format.
3. **ICP-mismatched adapters** (HN general, WeWorkRemotely, RemoteOK, HN Hiring — per prospect engine plan §4.3) keep running in signal-crawler as examples, are not promoted to signal-producer, and are documented in MIGRATION.md.
4. **Procurement sources do not migrate to signal-producer.** They extend rfp-ingestor (per prospect engine plan §P3).
5. **The transition is not dated.** signal-crawler is retired when the last canonical adapter has landed in signal-producer, not on a calendar.

---

## Cross-service boundaries

- **North Cloud owns** the signal schema, the producer catalogue, the threshold gate, the dedup keys, and classifier-side enrichments.
- **North Cloud does not own** the Waaseyaa-side ranker, feedback store, briefing UI, or prospect definition — those live in the prospect engine plan and in the Waaseyaa repo.
- **`ClaudrielLead` is a bridge, not a schema.** It ships the RFP-shaped contract Claudriel's fetcher expects; do not repurpose it as a generic lead model. If Claudriel's needs change, version the endpoint (`/api/leads/v2`) rather than breaking the v1 struct.

---

## Related docs

- `docs/prospect-engine-plan.md` — downstream intelligence layer (ranker, briefing, feedback)
- `docs/specs/rfp-ingestor.md` — procurement producer
- `docs/specs/classification.md` — classifier pipeline that hosts `need_signal`
- `docs/specs/content-routing.md` — publisher routing (Layer 11 rfp, and the `/api/leads` bridge)
- `signal-crawler/CLAUDE.md` — operator guide
- `signal-crawler/MIGRATION.md` — successor direction (#641)
- Lead Intelligence Integration milestone — #592 – #600

---

## Change log

| Date | Change | Author |
|---|---|---|
| 2026-04-18 | Initial draft (#640) | Claude / approved by Russell |
