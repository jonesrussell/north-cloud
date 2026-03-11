# AI-Assisted Verification of Scraped Leadership/Contact Data

**Issue**: #298
**Date**: 2026-03-11
**Status**: Draft
**Depends on**: #273 (scraper), #274 (verification queue)

---

## Summary

Add an AI verification step inside source-manager that evaluates scraped Person and BandOffice records for plausibility and cross-field consistency before they enter the verification queue. High-confidence records are auto-verified; low-confidence records are auto-rejected; everything in between is queued for human review.

## Decision Log

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Where to host | source-manager | Owns Person/BandOffice data and verification lifecycle |
| Verification scope (v1) | Name/role plausibility + cross-field consistency | 80-90% of bad extractions caught without page re-fetch |
| Thresholds | Auto-verify >= 0.95, auto-reject < 0.30 | Conservative; loosen after production telemetry |
| Worker architecture | Background goroutine | Simplest path; no new service/container needed |
| LLM provider | Anthropic Claude (via anthropic-sdk-go) | Matches ai-observer pattern; Go-native SDK |

## Schema Changes

Two new columns on both `people` and `band_offices` tables:

```sql
ALTER TABLE people ADD COLUMN verification_confidence REAL;
ALTER TABLE people ADD COLUMN verification_issues JSONB DEFAULT '[]';

ALTER TABLE band_offices ADD COLUMN verification_confidence REAL;
ALTER TABLE band_offices ADD COLUMN verification_issues JSONB DEFAULT '[]';
```

- `verification_confidence`: 0.0-1.0 score from the LLM. NULL means not yet evaluated.
- `verification_issues`: JSON array of issues found by the LLM.

Issue shape:

```json
{"field": "phone", "issue": "format doesn't match province", "severity": "warning"}
```

Severity levels: `error` (likely garbage), `warning` (suspicious), `info` (minor concern).

Existing `verified` + `verified_at` fields handle final state. No new tables needed.

## Verification Worker

### Lifecycle

- Starts in source-manager bootstrap after database + server setup
- Runs on a configurable ticker (default: 5 min)
- Disabled by default: `VERIFICATION_AI_ENABLED=false`
- Graceful shutdown via context cancellation

### Each Tick

1. Query records: `WHERE verified = false AND verification_confidence IS NULL LIMIT $batch_size`
2. For each record, call Claude API with structured prompt
3. Parse response, write `verification_confidence` + `verification_issues` to the record
4. Apply thresholds:
   - `>= 0.95`: set `verified=true, verified_at=now()` (auto-verify)
   - `< 0.30`: delete the record (auto-reject)
   - `0.30-0.95`: no action; stays in queue with confidence + issues for human review

### Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `VERIFICATION_AI_ENABLED` | `false` | Enable AI verification worker |
| `VERIFICATION_INTERVAL` | `5m` | Poll interval |
| `VERIFICATION_BATCH_SIZE` | `10` | Records per tick |
| `VERIFICATION_AUTO_VERIFY_THRESHOLD` | `0.95` | Auto-verify above this |
| `VERIFICATION_AUTO_REJECT_THRESHOLD` | `0.30` | Auto-reject below this |
| `ANTHROPIC_API_KEY` | (none) | Required when enabled |

## LLM Prompt Design

Single prompt template for both Person and BandOffice records.

### System Prompt

```
You are a data quality verifier for First Nations community leadership and contact records scraped from official websites. Your job is to evaluate whether extracted data is plausible and internally consistent.

Evaluate:
1. Name plausibility — Is this a real human name, or scraper noise (navigation text, template fragments, "Click Here", "Vacant", "TBD")?
2. Role plausibility — Is the role a recognized leadership/staff title (Chief, Councillor, Band Manager, Director, Elder, etc.)?
3. Cross-field consistency — Does phone area code match province? Does email domain relate to the community? Does address match expected region?

Return JSON only.
```

### User Prompt (templated per record)

Person example:

```json
{
  "record_type": "person",
  "name": "John Smith",
  "role": "Chief",
  "email": "jsmith@community.ca",
  "phone": "807-555-1234",
  "community_name": "Fort William First Nation",
  "province": "Ontario",
  "source_url": "https://fwfn.com/council"
}
```

BandOffice records include address, phone, fax, email, toll-free, and office hours.

### Expected Response (JSON schema enforced)

```json
{
  "confidence": 0.92,
  "issues": [
    {
      "field": "email",
      "issue": "Generic domain, not community-specific",
      "severity": "info"
    }
  ]
}
```

The LLM's `confidence` value drives threshold logic in Go. Token cost estimate: ~500 tokens per record. At 10 records/tick, 5-min interval, ~144K tokens/day max.

## Package Structure

```
source-manager/internal/
├── aiverify/
│   ├── worker.go       # Background worker (ticker, batch fetch, threshold logic)
│   ├── prompt.go       # Prompt templates + response parsing
│   └── client.go       # Anthropic SDK wrapper
├── config/
│   └── config.go       # Extended with Verification section
└── repository/
    └── verification.go # Extended with new query methods
```

### Config Extension

```go
type Verification struct {
    AIEnabled           bool          `env:"VERIFICATION_AI_ENABLED" default:"false"`
    Interval            time.Duration `env:"VERIFICATION_INTERVAL" default:"5m"`
    BatchSize           int           `env:"VERIFICATION_BATCH_SIZE" default:"10"`
    AutoVerifyThreshold float64       `env:"VERIFICATION_AUTO_VERIFY_THRESHOLD" default:"0.95"`
    AutoRejectThreshold float64       `env:"VERIFICATION_AUTO_REJECT_THRESHOLD" default:"0.30"`
    AnthropicAPIKey     string        `env:"ANTHROPIC_API_KEY"`
}
```

### Bootstrap Integration

```go
if cfg.Verification.AIEnabled {
    verifyClient := aiverify.NewClient(cfg.Verification.AnthropicAPIKey)
    worker := aiverify.NewWorker(verificationRepo, verifyClient, cfg.Verification, log)
    go worker.Run(ctx)
}
```

### New Repository Methods

- `ListUnverifiedUnscored(ctx, limit)` — `WHERE verified=false AND verification_confidence IS NULL`
- `UpdateVerificationResult(ctx, entityType, id, confidence, issues)` — writes AI results back

### Model Extensions

```go
VerificationConfidence *float64        `json:"verification_confidence,omitempty"`
VerificationIssues     json.RawMessage `json:"verification_issues,omitempty"`
```

### New Dependency

`anthropic-sdk-go` added to source-manager's `go.mod`.

## Error Handling

- **LLM call fails** (network, rate limit, malformed response): log warning, skip record, retry next tick. No record mutation on failure.
- **Invalid JSON response**: log error with raw response, skip record. Confidence stays NULL.
- **Database write fails**: log error, skip. Record re-fetched next tick.
- **All errors are non-fatal** — worker continues processing remaining records in the batch.

## Observability

Structured logging on every action:

- `verification.auto_verified` — id, type, confidence
- `verification.auto_rejected` — id, type, confidence, issues
- `verification.queued` — id, type, confidence, issue count
- `verification.error` — id, type, error message
- `verification.tick` — batch size, processed count, duration

Existing Grafana/Loki stack picks these up via JSON structured logs. No new dashboards in v1.

## Testing

- **prompt.go**: template rendering, response parsing, edge cases (empty name, missing fields)
- **worker.go**: threshold logic with mock client (verify auto-verify/reject/queue paths)
- **client.go**: response validation, error handling (mock HTTP)
- **Integration**: full flow with mock Anthropic response, DB state assertions
- No real LLM calls in CI — all tests use mocked responses

## Future (v2+)

- Source page re-fetch comparison (option c from design session)
- Confidence threshold tuning based on production telemetry
- Dashboard UI for reviewing AI-flagged issues
- Batch re-verification when scraper re-runs
