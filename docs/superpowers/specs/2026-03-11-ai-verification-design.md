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
| Anthropic client | Extract to infrastructure/provider/anthropic | Follows dependency rule; reused by ai-observer |

## Schema Changes

Two new columns on both `people` and `band_offices` tables via a numbered migration file in `source-manager/internal/database/migrations/`:

**Up migration** (`NNNNNN_add_verification_ai_columns.up.sql`):

```sql
ALTER TABLE people ADD COLUMN verification_confidence REAL;
ALTER TABLE people ADD COLUMN verification_issues JSONB DEFAULT '[]';

ALTER TABLE band_offices ADD COLUMN verification_confidence REAL;
ALTER TABLE band_offices ADD COLUMN verification_issues JSONB DEFAULT '[]';
```

**Down migration** (`NNNNNN_add_verification_ai_columns.down.sql`):

```sql
ALTER TABLE people DROP COLUMN IF EXISTS verification_confidence;
ALTER TABLE people DROP COLUMN IF EXISTS verification_issues;

ALTER TABLE band_offices DROP COLUMN IF EXISTS verification_confidence;
ALTER TABLE band_offices DROP COLUMN IF EXISTS verification_issues;
```

- `verification_confidence`: 0.0-1.0 score from the LLM. NULL means not yet evaluated.
- `verification_issues`: JSON array of issues found by the LLM.

Issue shape:

```json
{"field": "phone", "issue": "format doesn't match province", "severity": "warning"}
```

Severity levels: `error` (likely garbage), `warning` (suspicious), `info` (minor concern).

Existing `verified` + `verified_at` fields handle final state. No new tables needed.

### Scan Function Updates

Adding these columns requires updating:

- `personColumns` and `bandOfficeColumns` constants to include the new fields
- `scanPerson()` and `scanBandOffice()` functions to scan the new columns
- All INSERT/UPDATE queries in `PersonRepository` and `BandOfficeRepository` to include the new columns
- Model structs (see Model Extensions below)

## Verification Worker

### Lifecycle

- Starts in source-manager bootstrap after database + server setup
- Runs on a configurable ticker (default: 5 min)
- Disabled by default: `VERIFICATION_AI_ENABLED=false`
- Graceful shutdown: bootstrap creates a cancellable context and cancels it when `server.Run()` returns, which stops the worker goroutine

### Each Tick

1. Query records: `WHERE verified = false AND verification_confidence IS NULL LIMIT $batch_size`
2. For each record, call Claude API with structured prompt (500ms delay between calls to avoid rate limits)
3. Parse response, write `verification_confidence` + `verification_issues` to the record
4. Apply thresholds:
   - `>= 0.95`: set `verified=true, verified_at=now()` (auto-verify)
   - `< 0.30`: direct DELETE (skip the lookup+check in RejectPerson/RejectBandOffice since we just queried with `WHERE verified=false`)
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
| `ANTHROPIC_MODEL` | `claude-haiku-4-5-20251001` | Model for verification (Haiku for cost efficiency) |

### Human Review Queue Interaction

The existing `ListPending` endpoint (from #274) queries `WHERE verified = false`. After AI scoring:

- Records with `verification_confidence IS NULL` are awaiting AI evaluation
- Records with `0.30 <= verification_confidence < 0.95` are AI-scored and awaiting human review
- The `ListPending` response already includes `verification_confidence` and `verification_issues` fields on each record, so the dashboard can display the AI's reasoning

No changes to `ListPending` query logic needed — it correctly returns all unverified records regardless of AI scoring status.

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

BandOffice example:

```json
{
  "record_type": "band_office",
  "community_name": "Fort William First Nation",
  "province": "Ontario",
  "address_line1": "90 Anemki Drive",
  "city": "Thunder Bay",
  "postal_code": "P7J 1L3",
  "phone": "807-623-9543",
  "fax": "807-623-5190",
  "email": "reception@fwfn.com",
  "toll_free": "1-800-555-0123",
  "office_hours": "Mon-Fri 8:30am-4:30pm",
  "source_url": "https://fwfn.com/contact"
}
```

Note: `community_name` and `province` are populated via a JOIN on the `communities` table when building the prompt. The worker query joins `people/band_offices` with `communities` to include this context.

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
│   └── prompt.go       # Prompt templates + response parsing
├── config/
│   └── config.go       # Extended with Verification section
└── repository/
    └── verification.go # Extended with new query methods

infrastructure/provider/anthropic/
├── client.go           # Anthropic SDK wrapper (extracted from ai-observer)
└── client_test.go
```

The Anthropic client is extracted to `infrastructure/provider/anthropic/` to follow the dependency rule ("services import only from `infrastructure/`"). Both source-manager and ai-observer import from this shared package. The ai-observer's existing `internal/provider/anthropic/` is replaced with an import of the shared package.

### Config Extension

```go
type Verification struct {
    AIEnabled           bool          `yaml:"ai_enabled" env:"VERIFICATION_AI_ENABLED"`
    Interval            time.Duration `yaml:"interval" env:"VERIFICATION_INTERVAL"`
    BatchSize           int           `yaml:"batch_size" env:"VERIFICATION_BATCH_SIZE"`
    AutoVerifyThreshold float64       `yaml:"auto_verify_threshold" env:"VERIFICATION_AUTO_VERIFY_THRESHOLD"`
    AutoRejectThreshold float64       `yaml:"auto_reject_threshold" env:"VERIFICATION_AUTO_REJECT_THRESHOLD"`
    AnthropicAPIKey     string        `yaml:"anthropic_api_key" env:"ANTHROPIC_API_KEY"`
    AnthropicModel      string        `yaml:"anthropic_model" env:"ANTHROPIC_MODEL"`
}
```

Defaults applied in `setDefaults()` following existing config pattern:

```go
func setDefaults() {
    // ... existing defaults ...
    viper.SetDefault("verification.ai_enabled", false)
    viper.SetDefault("verification.interval", "5m")
    viper.SetDefault("verification.batch_size", 10)
    viper.SetDefault("verification.auto_verify_threshold", 0.95)
    viper.SetDefault("verification.auto_reject_threshold", 0.30)
    viper.SetDefault("verification.anthropic_model", "claude-haiku-4-5-20251001")
}
```

### Bootstrap Integration

```go
if cfg.Verification.AIEnabled {
    verifyClient := anthropic.New(cfg.Verification.AnthropicAPIKey, cfg.Verification.AnthropicModel)
    worker := aiverify.NewWorker(verificationRepo, verifyClient, cfg.Verification, log)

    ctx, cancel := context.WithCancel(context.Background())
    go worker.Run(ctx)

    // After server.Run() returns (blocking), cancel the worker
    defer cancel()
}
```

### New Repository Methods

- `ListUnverifiedUnscoredPeople(ctx, limit)` — `SELECT ... FROM people JOIN communities ... WHERE verified=false AND verification_confidence IS NULL`
- `ListUnverifiedUnscoredBandOffices(ctx, limit)` — same pattern for band_offices
- `UpdatePersonVerificationResult(ctx, id, confidence, issues)` — writes AI results
- `UpdateBandOfficeVerificationResult(ctx, id, confidence, issues)` — writes AI results
- `AutoRejectPerson(ctx, id)` — direct `DELETE FROM people WHERE id = $1`
- `AutoRejectBandOffice(ctx, id)` — direct `DELETE FROM band_offices WHERE id = $1`

Separate methods per entity type follow the existing pattern (e.g., `VerifyPerson`/`VerifyBandOffice`).

### Model Extensions

```go
VerificationConfidence *float64        `json:"verification_confidence,omitempty"`
VerificationIssues     json.RawMessage `json:"verification_issues,omitempty"`
```

Added to both `Person` and `BandOffice` structs.

### New Dependency

`anthropic-sdk-go` added to `infrastructure/go.mod` (shared package).

## Error Handling

- **LLM call fails** (network, rate limit, malformed response): log warning, skip record, retry next tick. No record mutation on failure. Retry logic mirrors ai-observer: exponential backoff (250ms, 500ms, 1s) with jitter on HTTP 429.
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
- **infrastructure/provider/anthropic/client.go**: response validation, error handling (mock HTTP)
- **Integration**: full flow with mock Anthropic response, DB state assertions
- No real LLM calls in CI — all tests use mocked responses

## Future (v2+)

- Source page re-fetch comparison (option c from design session)
- Confidence threshold tuning based on production telemetry
- Dashboard UI for reviewing AI-flagged issues
- Batch re-verification when scraper re-runs
