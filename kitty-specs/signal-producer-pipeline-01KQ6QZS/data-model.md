# Data Model: Signal Producer Pipeline

**Mission**: `signal-producer-pipeline-01KQ6QZS`

The producer is stateless on its own — it has no database. The only persistent
state is the checkpoint file. The "data model" describes the in-memory and
on-the-wire shapes the producer consumes and produces.

## Checkpoint (persistent)

Stored at `/var/lib/signal-producer/checkpoint.json` (configurable). File mode
0640. JSON shape:

```json
{
  "last_successful_run": "2026-04-27T05:30:00Z",
  "last_batch_size": 23
}
```

| Field                 | Type                | Required | Notes                                                                                                                |
| --------------------- | ------------------- | -------- | -------------------------------------------------------------------------------------------------------------------- |
| `last_successful_run` | RFC3339 UTC string  | yes      | Timestamp at which the last fully successful run completed. Used as the lower bound for the next ES `crawled_at` query (after subtracting the lookback buffer). |
| `last_batch_size`     | non-negative int    | yes      | Number of signals delivered in the last successful run. For observability only.                                       |

**State transitions** (only one):

```
checkpoint(t0) → checkpoint(t1)
  precondition: every batch in the run delivered with HTTP 2xx
  postcondition: file rewritten atomically (write tmp → fsync → rename)
```

Partial-run failures leave the checkpoint at `t0`. A corrupt file is logged at
WARN and treated as missing (24-hour cold-start lookback per FR-004).

**Validation rules**:

1. `last_successful_run` parses as RFC3339 UTC. If parse fails → treat file as corrupt.
2. `last_batch_size` is non-negative.
3. The file is written atomically. The producer never writes to the canonical
   path directly; it writes to `<path>.tmp.<pid>`, fsyncs, then renames.

## ESHit (input)

A document returned by the ES query against `*_classified_content`. The mapper
consumes only the fields below; other fields are passed through into
`Signal.payload` for Waaseyaa-side use.

### Required for both content types

| Field           | Type   | Notes                                                                                          |
| --------------- | ------ | ---------------------------------------------------------------------------------------------- |
| `_id`           | string | ES document ID. Becomes part of `Signal.external_id` after prefixing.                          |
| `title`         | string | Becomes `Signal.label`.                                                                        |
| `quality_score` | int    | Becomes `Signal.strength`. Filter threshold ≥ 40 applied in the ES query.                      |
| `url`           | string | Becomes `Signal.source_url`.                                                                   |
| `crawled_at`    | RFC3339 UTC | Used in the ES range query and in checkpoint advancement decisions.                       |
| `content_type`  | enum {`rfp`, `need_signal`} | Determines which type-specific subfield map applies.                          |

### Type-specific subfields

For `content_type = rfp`:

| Field                       | Type     | Notes                                                                       |
| --------------------------- | -------- | --------------------------------------------------------------------------- |
| `rfp.organization_name`     | string   | Optional; missing → empty string. Becomes `Signal.organization_name`.       |
| `rfp.province`              | string   | Optional. Becomes `Signal.province`.                                        |
| `rfp.categories`            | []string | Optional. First GSIN category becomes `Signal.sector`.                      |
| `rfp.closing_date`          | RFC3339 UTC string, optional | Becomes `Signal.expires_at` if present.                  |

For `content_type = need_signal`:

| Field                              | Type   | Notes                                              |
| ---------------------------------- | ------ | -------------------------------------------------- |
| `need_signal.organization_name`    | string | Optional. Becomes `Signal.organization_name`.      |
| `need_signal.province`             | string | Optional. Becomes `Signal.province`.               |
| `need_signal.sector`               | string | Optional. Becomes `Signal.sector`.                 |
| `need_signal.signal_type`          | string | Required. Becomes `Signal.signal_type`.            |

### Validation rules

1. Missing required field at top level (`_id`, `title`, `quality_score`, `url`, `crawled_at`, `content_type`) → mapper returns error for that hit; producer logs and skips, increments `skipped` counter.
2. `content_type` outside `{rfp, need_signal}` → mapper returns error (the ES query should prevent this; defensive check).
3. For `need_signal`, missing `signal_type` → mapper returns error.
4. Missing optional fields → empty string in the mapped Signal (FR-008).

## Signal (output, on-the-wire)

The Waaseyaa wire format. JSON shape per FR-006:

```json
{
  "signal_type": "rfp",
  "external_id": "nc-rfp-AbC123",
  "source": "north-cloud",
  "source_url": "https://canadabuys.canada.ca/...",
  "label": "Bridge construction services",
  "strength": 78,
  "organization_name": "Government of Canada",
  "sector": "Construction",
  "province": "ON",
  "expires_at": "2026-05-15T17:00:00Z",
  "payload": { ... full ES hit ... }
}
```

| Field               | Type      | Required | Mapping rule                                                                                                                                 |
| ------------------- | --------- | -------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| `signal_type`       | string    | yes      | `"rfp"` for RFP hits; `need_signal.signal_type` value for need_signal hits.                                                                 |
| `external_id`       | string    | yes      | `nc-rfp-{_id}` for RFP, `nc-sig-{_id}` for need_signal. Prefix prevents collisions (FR-007). Waaseyaa-side dedup keys off this.            |
| `source`            | string    | yes      | Always `"north-cloud"` (FR-006).                                                                                                            |
| `source_url`        | string    | yes      | From ES `url`.                                                                                                                              |
| `label`             | string    | yes      | From ES `title`.                                                                                                                            |
| `strength`          | int       | yes      | From ES `quality_score`.                                                                                                                    |
| `organization_name` | string    | yes      | From type-specific subfield. Empty string if missing.                                                                                       |
| `sector`            | string    | yes      | From type-specific subfield. RFP: first GSIN category. need_signal: `need_signal.sector`. Empty string if missing.                          |
| `province`          | string    | yes      | From type-specific subfield. Empty string if missing.                                                                                       |
| `expires_at`        | RFC3339   | optional | RFP only, from `rfp.closing_date`. Omit field if absent.                                                                                    |
| `payload`           | object    | yes      | The full original ES hit. Allows Waaseyaa to access fields the producer doesn't surface today without requiring a producer change tomorrow. |

## IngestResult (input from Waaseyaa response)

| Field           | Type | Notes                                                                |
| --------------- | ---- | -------------------------------------------------------------------- |
| `ingested`      | int  | Signals accepted and stored.                                         |
| `skipped`       | int  | Signals seen but skipped by Waaseyaa (typically dedup hits).         |
| `leads_created` | int  | Signals that resulted in new lead creation on the Waaseyaa side.    |
| `leads_matched` | int  | Signals matched to an existing lead.                                 |
| `unmatched`     | int  | Signals stored but with no lead correlation.                         |

The producer logs all five fields per batch and per run summary. They do not
affect checkpoint behavior — any HTTP 2xx response counts as delivery.

## Config (load-time)

Loaded from `signal-producer/config.yml` plus environment overrides via
`infrastructure/config/`. Shape:

```yaml
waaseyaa:
  url: "${WAASEYAA_URL}"
  api_key: "${WAASEYAA_API_KEY}"
  batch_size: 50
  min_quality_score: 40

elasticsearch:
  url: "${ES_URL}"
  indexes: ["*_classified_content"]

schedule:
  lookback_buffer: "5m"

checkpoint:
  file: "/var/lib/signal-producer/checkpoint.json"
```

### Validation rules at load

1. `waaseyaa.url` must parse as a valid URL with `https` scheme in production. Dev allows `http://localhost`.
2. `waaseyaa.api_key` non-empty (otherwise producer exits with a clear error, not a 401 from Waaseyaa).
3. `waaseyaa.batch_size` in `[1, 500]` (sanity bounds).
4. `waaseyaa.min_quality_score` in `[0, 100]`.
5. `elasticsearch.url` parses as a valid URL.
6. `schedule.lookback_buffer` parses via `time.ParseDuration`.
7. `checkpoint.file` parent directory must exist and be writable (verified at startup).

Validation failures cause the binary to exit non-zero before any ES call. Don't
let a misconfigured run masquerade as a "no signals to send" success.
