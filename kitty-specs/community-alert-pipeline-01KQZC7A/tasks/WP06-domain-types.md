---
work_package_id: WP06
title: Domain Types
dependencies:
- WP05
requirement_refs:
- FR-002
- FR-003
- FR-015
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T023
- T024
- T025
- T026
- T027
phase: B
agent: "claude:opus:reviewer:reviewer"
shell_pid: "226866"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: alert-crawler/internal/domain/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- alert-crawler/internal/domain/**
priority: P1
tags: []
---

# WP06 — Domain Types

## Objective

Define the canonical Go types for the `community_alert` envelope, hazard sub-types, source configuration, and lifecycle event payload. The Go types must round-trip to JSON conforming to `contracts/community-alert.schema.json` and `contracts/lifecycle-event.schema.json`.

L0 layer (no internal imports beyond stdlib + minimal external like `time`).

## Context

- Spec §3 (FR-002, FR-003, FR-015), §7.1 Key Entities (Alert)
- Plan §3 Architecture, §Component Design
- Contracts: `contracts/community-alert.schema.json`, `contracts/lifecycle-event.schema.json`
- Data model: `data-model.md` §1–§4

## Branch Strategy

Standard. Lane worktree from `main`. Depends on WP05 (scaffold must exist before adding internal packages).

## Subtasks

### T023 — Create `internal/domain/alert.go`

**Purpose**: Define the `Alert` envelope struct matching the JSON schema. Includes JSON struct tags for serialization, validation tags where applicable.

**Steps**:
1. Create `alert-crawler/internal/domain/alert.go`.
2. Define types:
   ```go
   package domain

   import "time"

   type Severity string
   const (
       SeverityInfo     Severity = "info"
       SeverityLow      Severity = "low"
       SeverityMedium   Severity = "medium"
       SeverityHigh     Severity = "high"
       SeverityCritical Severity = "critical"
   )

   type Category string
   const (
       CategoryHarmReduction Category = "harm_reduction"
       // future: water, evacuation, missing_person, ...
   )

   type LifecycleState string
   const (
       LifecycleActive    LifecycleState = "active"
       LifecycleRescinded LifecycleState = "rescinded"
   )

   type ParseQuality string
   const (
       ParseClean    ParseQuality = "clean"
       ParseDegraded ParseQuality = "degraded"
       ParseFailed   ParseQuality = "failed"
   )

   type Alert struct {
       ID              string             `json:"id"`
       Category        Category           `json:"category"`
       Severity        Severity           `json:"severity"`
       Scope           []string           `json:"scope"`
       IssuedAt        time.Time          `json:"issued_at"`
       ExpiresAt       *time.Time         `json:"expires_at,omitempty"`
       LifecycleState  LifecycleState     `json:"lifecycle_state"`
       RescindedAt     *time.Time         `json:"rescinded_at,omitempty"`
       Title           string             `json:"title"`
       Summary         string             `json:"summary"`
       Hazard          Hazard             `json:"hazard"`
       Guidance        []string           `json:"guidance,omitempty"`
       Sources         []SourceAttribution `json:"sources"`
       RevisionHistory []Revision         `json:"revision_history,omitempty"`
       ParseQuality    ParseQuality       `json:"parse_quality"`
       CrawledAt       time.Time          `json:"crawled_at"`
       LastUpdatedAt   time.Time          `json:"last_updated_at"`
   }

   type SourceAttribution struct {
       SourceID         string   `json:"source_id"`
       SourceName       string   `json:"source_name"`
       URL              string   `json:"url"`
       AttributionText  string   `json:"attribution_text,omitempty"`
       MediaLinks       []string `json:"media_links,omitempty"`
   }

   type Revision struct {
       RevisionAt     time.Time `json:"revision_at"`
       RevisionKind   string    `json:"revision_kind"` // created|updated|rescinded|parse_degraded|parse_recovered
       ChangeSummary  string    `json:"change_summary,omitempty"`
       ChangedFields  []string  `json:"changed_fields,omitempty"`
   }
   ```
3. Add a `Validate() error` method on `Alert` that checks: non-empty `id`, valid `category`, valid `severity`, non-empty `scope` slice, `issued_at` non-zero, valid `lifecycle_state`, valid `parse_quality`, non-empty `title`/`summary`, at least one source.
4. Constants are exported; tests in T027 verify enum coverage.

**Files**:
- `alert-crawler/internal/domain/alert.go` (new, ~120 lines).

**Validation**:
- Compiles cleanly. `gofmt`/`go vet` pass.
- `Validate()` rejects malformed alerts.

**Edge Cases**:
- Pointer types for nullable fields (`*time.Time`) match the JSON Schema's `["string", "null"]`.

### T024 — Create `internal/domain/hazard.go`

**Purpose**: Define the `Hazard` discriminated union (v1 only `HarmReductionHazard`).

**Steps**:
1. Create `alert-crawler/internal/domain/hazard.go`.
2. Define types:
   ```go
   type HazardType string
   const (
       HazardOpioidSupply    HazardType = "opioid_supply"
       HazardStimulantSupply HazardType = "stimulant_supply"
       HazardBenzoSupply     HazardType = "benzo_supply"
       HazardOther           HazardType = "other"
   )

   // Hazard is the wrapper carrying category-specific data.
   // For v1, only HarmReductionHazard is implemented; future categories will
   // expand this to a discriminated union.
   type Hazard struct {
       HarmReduction *HarmReductionHazard `json:"-"` // populated by parser
       // Raw JSON form is flattened directly into Alert.hazard via custom MarshalJSON.
   }

   type HarmReductionHazard struct {
       HazardType         HazardType   `json:"hazard_type"`
       Substances         []string     `json:"substances"`
       Composition        []Substance  `json:"composition,omitempty"`
       VisualDescription  string       `json:"visual_description,omitempty"`
       LabSource          string       `json:"lab_source,omitempty"`
       ConfirmationDate   *time.Time   `json:"confirmation_date,omitempty"` // date-only ideally
   }

   type Substance struct {
       Name               string  `json:"name"`
       Percentage         float64 `json:"percentage,omitempty"`
       IsActiveIngredient bool    `json:"is_active_ingredient,omitempty"`
       Note               string  `json:"note,omitempty"`
   }
   ```
3. Implement `MarshalJSON` and `UnmarshalJSON` on `Hazard` so the JSON wire format flattens to the harm-reduction shape directly (matching the schema's `oneOf` structure).
4. Validation: a `Hazard` with no inner pointer is invalid. For v1 an Alert must have `HarmReduction != nil`.

**Files**:
- `alert-crawler/internal/domain/hazard.go` (new, ~120 lines).

**Validation**:
- JSON round-trip: an alert with `category: harm_reduction` marshals/unmarshals correctly through the discriminated union.

### T025 — Create `internal/domain/source.go`

**Purpose**: Configuration-time entity describing one upstream source. Used by config and runner.

**Steps**:
1. Create `alert-crawler/internal/domain/source.go`:
   ```go
   type AcquisitionStrategy string
   const (
       AcquisitionRSS  AcquisitionStrategy = "rss"
       AcquisitionAtom AcquisitionStrategy = "atom"
       AcquisitionJSON AcquisitionStrategy = "json"
       AcquisitionHTML AcquisitionStrategy = "html"
   )

   type AlertSource struct {
       ID                   string              `yaml:"id" json:"id"`
       Name                 string              `yaml:"name" json:"name"`
       FeedURL              string              `yaml:"feed_url" env:"FEED_URL" json:"feed_url"`
       AcquisitionStrategy  AcquisitionStrategy `yaml:"acquisition_strategy" env:"ACQUISITION_STRATEGY" json:"acquisition_strategy"`
       PollInterval         time.Duration       `yaml:"poll_interval" env:"POLL_INTERVAL" json:"poll_interval"`
       DefaultCategory      Category            `yaml:"default_category" env:"DEFAULT_CATEGORY" json:"default_category"`
       DefaultScope         []string            `yaml:"default_scope" env:"DEFAULT_SCOPE" envSeparator:"," json:"default_scope"`
       DefaultExpiry        time.Duration       `yaml:"default_expiry" env:"DEFAULT_EXPIRY" json:"default_expiry"`
       Enabled              bool                `yaml:"enabled" env:"ENABLED" json:"enabled"`
   }
   ```
2. Implement validation: PollInterval in [30m, 60m] (per FR-001).

**Files**:
- `alert-crawler/internal/domain/source.go` (new, ~50 lines).

### T026 — Create `internal/domain/lifecycle_event.go`

**Purpose**: Redis pub/sub payload type matching `contracts/lifecycle-event.schema.json`.

**Steps**:
1. Create `alert-crawler/internal/domain/lifecycle_event.go`:
   ```go
   type EventType string
   const (
       EventCreated   EventType = "created"
       EventUpdated   EventType = "updated"
       EventRescinded EventType = "rescinded"
   )

   type LifecycleEvent struct {
       EventType EventType `json:"event_type"`
       EventAt   time.Time `json:"event_at"`
       AlertID   string    `json:"alert_id"`
       Category  Category  `json:"category"`
       Severity  Severity  `json:"severity"`
       Scope     []string  `json:"scope"`
       Payload   Alert     `json:"payload"`
   }
   ```
2. Add a constructor: `NewLifecycleEvent(eventType EventType, alert Alert) LifecycleEvent` that copies convenience fields and stamps `EventAt = time.Now().UTC()`.

**Files**:
- `alert-crawler/internal/domain/lifecycle_event.go` (new, ~40 lines).

### T027 — Unit tests for envelope round-trip

**Purpose**: Verify Go ↔ JSON conformance against the contracts.

**Steps**:
1. Create `alert-crawler/internal/domain/alert_test.go` and `hazard_test.go` and `lifecycle_event_test.go`.
2. Tests:
   - **Round-trip**: marshal an `Alert` to JSON, unmarshal back, assert `reflect.DeepEqual`.
   - **Validation**: `Alert{}.Validate()` returns errors covering all required fields.
   - **Enum coverage**: every constant for `Severity`, `Category`, `LifecycleState`, `ParseQuality`, `EventType`, `HazardType`, `AcquisitionStrategy` is exercised.
   - **JSON shape**: the marshaled JSON for a fixture alert matches a golden file (committed under `alert-crawler/internal/domain/testdata/golden_alert.json`).
3. Use `t.Helper()` for fixture builders.
4. Coverage ≥80% on `internal/domain/`.

**Files**:
- `alert-crawler/internal/domain/alert_test.go` (new)
- `alert-crawler/internal/domain/hazard_test.go` (new)
- `alert-crawler/internal/domain/lifecycle_event_test.go` (new)
- `alert-crawler/internal/domain/testdata/golden_alert.json` (new)

**Validation**:
- `task test:alert-crawler` passes for the domain package.

## Definition of Done

- All five subtasks complete.
- Types match the JSON Schemas in `contracts/`.
- Round-trip tests pass against golden fixtures.
- Coverage ≥80% on `internal/domain/`.
- Lint and vet clean.

## Risks

- **Schema drift**: if `community-alert.schema.json` changes after this WP, the Go types and tests must follow. Mitigation: golden file makes drift visible in CI.
- **Discriminated hazard union**: Go doesn't have native discriminated unions; the `Hazard` struct's manual `MarshalJSON`/`UnmarshalJSON` is the only correct way to flatten the JSON shape. Get this right.

## Reviewer Guidance

- Verify field names and JSON tags match the schema exactly.
- Verify nullable fields use pointer types (`*time.Time`).
- Verify `Validate()` is comprehensive.
- Verify the golden file is realistic (an actual harm-reduction alert).

## Implementation Command

```bash
spec-kitty agent action implement WP06 --agent <name>
```

Depends on WP05.

## Activity Log

- 2026-05-06T22:12:49Z – claude:sonnet:implementer:implementer – shell_pid=221680 – Started implementation via action command
- 2026-05-06T22:18:16Z – claude:sonnet:implementer:implementer – shell_pid=221680 – Domain types match JSON schemas; round-trip + golden tests pass; 97.6% coverage; lint + vet clean
- 2026-05-06T22:18:51Z – claude:opus:reviewer:reviewer – shell_pid=226866 – Started review via action command
