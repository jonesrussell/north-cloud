# AI-Assisted Verification Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an AI verification worker to source-manager that scores scraped Person/BandOffice records for plausibility and consistency, auto-verifying high-confidence and auto-rejecting low-confidence records.

**Architecture:** Background goroutine in source-manager polls unscored records, sends each to Claude API for evaluation, writes confidence + issues back, and applies threshold logic. Anthropic SDK client extracted to `infrastructure/provider/anthropic/` for reuse by ai-observer.

**Tech Stack:** Go 1.26+, anthropic-sdk-go, PostgreSQL, Gin (existing)

**Spec:** `docs/superpowers/specs/2026-03-11-ai-verification-design.md`

> **Note:** The spec references migration path `source-manager/internal/database/migrations/` — the actual path is `source-manager/migrations/`. This plan uses the correct path.

---

## File Structure

### New Files
| File | Responsibility |
|------|---------------|
| `infrastructure/provider/anthropic/client.go` | Shared Anthropic SDK wrapper (extracted from ai-observer) |
| `infrastructure/provider/anthropic/client_test.go` | Client unit tests |
| `source-manager/internal/aiverify/worker.go` | Background worker: ticker, batch fetch, threshold logic |
| `source-manager/internal/aiverify/worker_test.go` | Worker unit tests with mock client |
| `source-manager/internal/aiverify/prompt.go` | Prompt templates + response parsing |
| `source-manager/internal/aiverify/prompt_test.go` | Prompt rendering + parsing tests |
| `source-manager/internal/aiverify/llm_verifier.go` | LLM-backed Verifier implementation |
| `source-manager/migrations/016_add_verification_fields.up.sql` | Add verification_confidence + verification_issues columns |
| `source-manager/migrations/016_add_verification_fields.down.sql` | Drop those columns |

### Modified Files
| File | Change |
|------|--------|
| `infrastructure/go.mod` | Add anthropic-sdk-go dependency |
| `source-manager/go.mod` | (vendor sync after infrastructure change) |
| `source-manager/internal/models/person.go:25` | Add VerificationConfidence, VerificationIssues fields |
| `source-manager/internal/models/band_office.go:32` | Add VerificationConfidence, VerificationIssues fields |
| `source-manager/internal/repository/person.go:22-51` | Update personColumns, scanPerson, Create, Update |
| `source-manager/internal/repository/band_office.go:16-189` | Update bandOfficeColumns, scanBandOffice, Create, Update, Upsert |
| `source-manager/internal/repository/verification.go` | Add ListUnverifiedUnscored*, UpdateVerificationResult*, AutoReject* methods |
| `source-manager/internal/config/config.go:22-143` | Add Verification config struct + setDefaults |
| `source-manager/internal/bootstrap/app.go:52-66` | Start verification worker goroutine before server.Run() |
| `ai-observer/internal/provider/anthropic/client.go` | Replace with import of infrastructure/provider/anthropic |

> **Type note:** The spec says `VerificationIssues` should be `json.RawMessage`. This plan uses `*string` instead because PostgreSQL JSONB scans cleanly to `*string` and avoids nullable-RawMessage edge cases. The JSON content is identical — it's just stored as a string in Go. The field serializes to JSON identically.

---

## Chunk 1: Database Schema + Model Updates

### Task 1: Add migration files

**Files:**
- Create: `source-manager/migrations/016_add_verification_fields.up.sql`
- Create: `source-manager/migrations/016_add_verification_fields.down.sql`

- [ ] **Step 1: Write up migration**

```sql
-- 016_add_verification_fields.up.sql
ALTER TABLE people ADD COLUMN verification_confidence REAL;
ALTER TABLE people ADD COLUMN verification_issues JSONB DEFAULT '[]';

ALTER TABLE band_offices ADD COLUMN verification_confidence REAL;
ALTER TABLE band_offices ADD COLUMN verification_issues JSONB DEFAULT '[]';
```

- [ ] **Step 2: Write down migration**

```sql
-- 016_add_verification_fields.down.sql
ALTER TABLE people DROP COLUMN IF EXISTS verification_confidence;
ALTER TABLE people DROP COLUMN IF EXISTS verification_issues;

ALTER TABLE band_offices DROP COLUMN IF EXISTS verification_confidence;
ALTER TABLE band_offices DROP COLUMN IF EXISTS verification_issues;
```

- [ ] **Step 3: Commit**

```bash
git add source-manager/migrations/016_add_verification_fields.up.sql \
       source-manager/migrations/016_add_verification_fields.down.sql
git commit -m "feat(source-manager): add verification AI columns migration (#298)"
```

### Task 2: Update Person model

**Files:**
- Modify: `source-manager/internal/models/person.go:25`

- [ ] **Step 1: Add fields to Person struct**

After the `VerifiedAt` field (line 25), add:

```go
VerificationConfidence *float64 `db:"verification_confidence" json:"verification_confidence,omitempty"`
VerificationIssues     *string  `db:"verification_issues" json:"verification_issues,omitempty"`
```

- [ ] **Step 2: Commit**

```bash
git add source-manager/internal/models/person.go
git commit -m "feat(source-manager): add verification fields to Person model (#298)"
```

### Task 3: Update BandOffice model

**Files:**
- Modify: `source-manager/internal/models/band_office.go:32`

- [ ] **Step 1: Add fields to BandOffice struct**

After the `VerifiedAt` field (line 32), add:

```go
VerificationConfidence *float64 `db:"verification_confidence" json:"verification_confidence,omitempty"`
VerificationIssues     *string  `db:"verification_issues" json:"verification_issues,omitempty"`
```

- [ ] **Step 2: Commit**

```bash
git add source-manager/internal/models/band_office.go
git commit -m "feat(source-manager): add verification fields to BandOffice model (#298)"
```

### Task 4: Update PersonRepository scan + columns

**Files:**
- Modify: `source-manager/internal/repository/person.go:22-80`

- [ ] **Step 1: Update personColumns constant (line 22)**

Append `, verification_confidence, verification_issues` to the column list string.

- [ ] **Step 2: Update scanPerson function (line 40)**

Add `&p.VerificationConfidence, &p.VerificationIssues` to the `Scan()` call, matching column order.

- [ ] **Step 3: Update Create method**

Add `verification_confidence` and `verification_issues` as new positional parameters ($18, $19) in the INSERT query and pass `p.VerificationConfidence, p.VerificationIssues` as values.

- [ ] **Step 4: Update Update method**

Add the two new columns to the SET clause and parameter list.

- [ ] **Step 5: Run tests**

```bash
cd source-manager && GOWORK=off go test ./internal/repository/ -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add source-manager/internal/repository/person.go
git commit -m "feat(source-manager): update PersonRepository for verification fields (#298)"
```

### Task 5: Update BandOfficeRepository scan + columns

**Files:**
- Modify: `source-manager/internal/repository/band_office.go:16-189`

- [ ] **Step 1: Update bandOfficeColumns constant (line 16)**

Append `, verification_confidence, verification_issues` to the column list string.

- [ ] **Step 2: Update scanBandOffice function (line 35)**

Add `&bo.VerificationConfidence, &bo.VerificationIssues` to the `Scan()` call.

- [ ] **Step 3: Update Create method**

Add placeholders ($19, $20) and values for the new columns.

- [ ] **Step 4: Update Update method**

Add the two new columns to the SET clause.

- [ ] **Step 5: Update Upsert method**

Add the new columns to both the INSERT and ON CONFLICT UPDATE clauses.

- [ ] **Step 6: Run tests**

```bash
cd source-manager && GOWORK=off go test ./internal/repository/ -v
```

Expected: PASS

- [ ] **Step 7: Run full test suite + lint**

```bash
cd source-manager && GOWORK=off go test ./... && GOWORK=off golangci-lint run
```

Expected: All tests pass, 0 lint issues.

- [ ] **Step 8: Commit**

```bash
git add source-manager/internal/repository/band_office.go
git commit -m "feat(source-manager): update BandOfficeRepository for verification fields (#298)"
```

---

## Chunk 2: Anthropic Client Extraction

### Task 6: Extract Anthropic client to infrastructure

**Files:**
- Create: `infrastructure/provider/anthropic/client.go`
- Create: `infrastructure/provider/anthropic/client_test.go`
- Modify: `infrastructure/go.mod`

- [ ] **Step 1: Write the client interface test**

Create `infrastructure/provider/anthropic/client_test.go`:

```go
package anthropic_test

import (
    "testing"

    "github.com/jonesrussell/north-cloud/infrastructure/provider/anthropic"
)

func TestNewClient(t *testing.T) {
    client := anthropic.New("test-key", "claude-haiku-4-5-20251001")
    if client == nil {
        t.Fatal("expected non-nil client")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd infrastructure && GOWORK=off go test ./provider/anthropic/ -v
```

Expected: FAIL — package doesn't exist yet.

- [ ] **Step 3: Write the client**

Create `infrastructure/provider/anthropic/client.go`. Mirror the ai-observer's client pattern, including `MaxTokens` in the request to match the ai-observer's existing API:

```go
package anthropic

import (
    "context"
    "fmt"
    "math/rand/v2"
    "time"

    anthropicsdk "github.com/anthropics/anthropic-sdk-go"
    "github.com/anthropics/anthropic-sdk-go/option"
)

const (
    maxRetries       = 3
    baseRetryWait    = 250 * time.Millisecond
    defaultMaxTokens = 1024
)

// GenerateRequest holds the prompt data for a Claude API call.
type GenerateRequest struct {
    SystemPrompt string
    UserPrompt   string
    MaxTokens    int64 // 0 uses defaultMaxTokens
}

// GenerateResponse holds the parsed response from Claude.
type GenerateResponse struct {
    Content string
}

// Client wraps the Anthropic SDK for Claude API calls.
type Client struct {
    inner anthropicsdk.Client
    model anthropicsdk.Model
}

// New creates a new Anthropic client.
func New(apiKey, model string) *Client {
    return &Client{
        inner: anthropicsdk.NewClient(option.WithAPIKey(apiKey)),
        model: anthropicsdk.Model(model),
    }
}

// Generate sends a prompt to Claude and returns the response text.
func (c *Client) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
    maxTokens := req.MaxTokens
    if maxTokens <= 0 {
        maxTokens = defaultMaxTokens
    }

    params := anthropicsdk.MessageNewParams{
        Model:     c.model,
        MaxTokens: maxTokens,
        System: []anthropicsdk.TextBlockParam{
            anthropicsdk.NewTextBlock(req.SystemPrompt),
        },
        Messages: []anthropicsdk.MessageParam{
            anthropicsdk.NewUserMessage(
                anthropicsdk.NewTextBlock(req.UserPrompt),
            ),
        },
    }

    var lastErr error
    for attempt := range maxRetries {
        resp, err := c.inner.Messages.New(ctx, params)
        if err == nil {
            if len(resp.Content) == 0 {
                return nil, fmt.Errorf("empty response from Claude")
            }
            return &GenerateResponse{Content: resp.Content[0].Text}, nil
        }

        lastErr = err
        wait := baseRetryWait * time.Duration(1<<attempt)
        jitter := time.Duration(rand.Int64N(int64(wait / 2)))
        time.Sleep(wait + jitter)
    }

    return nil, fmt.Errorf("claude API failed after %d retries: %w", maxRetries, lastErr)
}
```

- [ ] **Step 4: Add anthropic-sdk-go to infrastructure/go.mod**

```bash
cd infrastructure && GOWORK=off go get github.com/anthropics/anthropic-sdk-go && GOWORK=off go mod tidy
```

- [ ] **Step 5: Run test to verify it passes**

```bash
cd infrastructure && GOWORK=off go test ./provider/anthropic/ -v
```

Expected: PASS

- [ ] **Step 6: Run lint**

```bash
cd infrastructure && GOWORK=off golangci-lint run ./provider/anthropic/
```

Expected: 0 issues

- [ ] **Step 7: Commit**

```bash
git add infrastructure/provider/anthropic/ infrastructure/go.mod infrastructure/go.sum
git commit -m "feat(infrastructure): extract shared Anthropic client (#298)"
```

### Task 7: Update ai-observer to use shared client

**Files:**
- Modify: `ai-observer/internal/provider/anthropic/client.go`
- Modify: `ai-observer/go.mod`

- [ ] **Step 1: Replace ai-observer's internal client with import**

Update `ai-observer/internal/provider/anthropic/client.go` to wrap the shared infrastructure client. The ai-observer's existing `provider.GenerateRequest` has `MaxTokens int` and `JSONSchema string` fields. Map these to the infrastructure client:
- Pass `MaxTokens` through directly (infrastructure client accepts `int64`)
- `JSONSchema` is used only by ai-observer's prompt builder — keep it on the ai-observer's request type, not the infrastructure client

- [ ] **Step 2: Run ai-observer tests**

```bash
cd ai-observer && GOWORK=off go test ./... -v
```

Expected: PASS

- [ ] **Step 3: Run lint**

```bash
cd ai-observer && GOWORK=off golangci-lint run
```

Expected: 0 issues

- [ ] **Step 4: Commit**

```bash
git add ai-observer/
git commit -m "refactor(ai-observer): use shared Anthropic client from infrastructure (#298)"
```

---

## Chunk 3: Prompt Design + Parsing

### Task 8: Write prompt template tests

**Files:**
- Create: `source-manager/internal/aiverify/prompt_test.go`

- [ ] **Step 1: Write tests for prompt rendering**

```go
package aiverify_test

import (
    "strings"
    "testing"

    "github.com/jonesrussell/north-cloud/source-manager/internal/aiverify"
)

func TestBuildPersonPrompt(t *testing.T) {
    input := aiverify.VerifyInput{
        RecordType:    "person",
        Name:          "John Smith",
        Role:          "Chief",
        Email:         "jsmith@fwfn.com",
        Phone:         "807-555-1234",
        CommunityName: "Fort William First Nation",
        Province:      "Ontario",
        SourceURL:     "https://fwfn.com/council",
    }

    prompt := aiverify.BuildUserPrompt(input)
    if prompt == "" {
        t.Fatal("expected non-empty prompt")
    }
    if !strings.Contains(prompt, "Fort William First Nation") {
        t.Error("prompt missing community name")
    }
    if !strings.Contains(prompt, "person") {
        t.Error("prompt missing record_type")
    }
}

func TestBuildBandOfficePrompt(t *testing.T) {
    input := aiverify.VerifyInput{
        RecordType:    "band_office",
        CommunityName: "Fort William First Nation",
        Province:      "Ontario",
        Phone:         "807-623-9543",
        Email:         "reception@fwfn.com",
        AddressLine1:  "90 Anemki Drive",
        City:          "Thunder Bay",
        PostalCode:    "P7J 1L3",
        SourceURL:     "https://fwfn.com/contact",
    }

    prompt := aiverify.BuildUserPrompt(input)
    if prompt == "" {
        t.Fatal("expected non-empty prompt")
    }
    if !strings.Contains(prompt, "band_office") {
        t.Error("prompt missing record_type")
    }
    if !strings.Contains(prompt, "90 Anemki Drive") {
        t.Error("prompt missing address")
    }
}

func TestParseVerifyResponse_Valid(t *testing.T) {
    raw := `{"confidence": 0.92, "issues": [{"field": "email", "issue": "Generic domain", "severity": "info"}]}`
    result, err := aiverify.ParseVerifyResponse(raw)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result.Confidence != 0.92 {
        t.Errorf("expected confidence 0.92, got %f", result.Confidence)
    }
    if len(result.Issues) != 1 {
        t.Errorf("expected 1 issue, got %d", len(result.Issues))
    }
}

func TestParseVerifyResponse_InvalidJSON(t *testing.T) {
    _, err := aiverify.ParseVerifyResponse("not json")
    if err == nil {
        t.Error("expected error for invalid JSON")
    }
}

func TestParseVerifyResponse_MissingConfidence(t *testing.T) {
    raw := `{"issues": []}`
    _, err := aiverify.ParseVerifyResponse(raw)
    if err == nil {
        t.Error("expected error for missing confidence")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd source-manager && GOWORK=off go test ./internal/aiverify/ -v
```

Expected: FAIL — package doesn't exist yet.

### Task 9: Implement prompt templates

**Files:**
- Create: `source-manager/internal/aiverify/prompt.go`

- [ ] **Step 1: Write the prompt package**

```go
package aiverify

import (
    "encoding/json"
    "errors"
    "fmt"
)

// SystemPrompt is the fixed system prompt for verification.
const SystemPrompt = `You are a data quality verifier for First Nations community leadership and contact records scraped from official websites. Your job is to evaluate whether extracted data is plausible and internally consistent.

Evaluate:
1. Name plausibility — Is this a real human name, or scraper noise (navigation text, template fragments, "Click Here", "Vacant", "TBD")?
2. Role plausibility — Is the role a recognized leadership/staff title (Chief, Councillor, Band Manager, Director, Elder, etc.)?
3. Cross-field consistency — Does phone area code match province? Does email domain relate to the community? Does address match expected region?

Return JSON only.`

// VerifyInput holds the record data to send to the LLM.
type VerifyInput struct {
    RecordType    string `json:"record_type"`
    Name          string `json:"name,omitempty"`
    Role          string `json:"role,omitempty"`
    Email         string `json:"email,omitempty"`
    Phone         string `json:"phone,omitempty"`
    CommunityName string `json:"community_name"`
    Province      string `json:"province,omitempty"`
    SourceURL     string `json:"source_url,omitempty"`
    AddressLine1  string `json:"address_line1,omitempty"`
    AddressLine2  string `json:"address_line2,omitempty"`
    City          string `json:"city,omitempty"`
    PostalCode    string `json:"postal_code,omitempty"`
    Fax           string `json:"fax,omitempty"`
    TollFree      string `json:"toll_free,omitempty"`
    OfficeHours   string `json:"office_hours,omitempty"`
}

// VerifyResult is the parsed LLM response.
type VerifyResult struct {
    Confidence float64       `json:"confidence"`
    Issues     []VerifyIssue `json:"issues"`
}

// VerifyIssue is a single issue found by the LLM.
type VerifyIssue struct {
    Field    string `json:"field"`
    Issue    string `json:"issue"`
    Severity string `json:"severity"` // "error", "warning", "info"
}

// BuildUserPrompt renders the user prompt JSON for a record.
func BuildUserPrompt(input VerifyInput) string {
    data, _ := json.MarshalIndent(input, "", "  ")
    return string(data)
}

// ParseVerifyResponse parses the LLM's JSON response.
func ParseVerifyResponse(raw string) (*VerifyResult, error) {
    var m map[string]interface{}
    if unmarshalErr := json.Unmarshal([]byte(raw), &m); unmarshalErr != nil {
        return nil, fmt.Errorf("parse verify response: %w", unmarshalErr)
    }
    if _, ok := m["confidence"]; !ok {
        return nil, errors.New("parse verify response: missing confidence field")
    }

    var result VerifyResult
    if err := json.Unmarshal([]byte(raw), &result); err != nil {
        return nil, fmt.Errorf("parse verify response: %w", err)
    }
    return &result, nil
}
```

- [ ] **Step 2: Run tests**

```bash
cd source-manager && GOWORK=off go test ./internal/aiverify/ -v
```

Expected: PASS

- [ ] **Step 3: Run lint**

```bash
cd source-manager && GOWORK=off golangci-lint run ./internal/aiverify/
```

Expected: 0 issues

- [ ] **Step 4: Commit**

```bash
git add source-manager/internal/aiverify/prompt.go source-manager/internal/aiverify/prompt_test.go
git commit -m "feat(source-manager): add AI verification prompt templates (#298)"
```

---

## Chunk 4: Verification Worker

### Task 10: Write worker tests

**Files:**
- Create: `source-manager/internal/aiverify/worker_test.go`

- [ ] **Step 1: Write tests for threshold logic**

```go
package aiverify_test

import (
    "testing"

    "github.com/jonesrussell/north-cloud/source-manager/internal/aiverify"
)

func TestClassifyAction_AutoVerify(t *testing.T) {
    action := aiverify.ClassifyAction(0.97, 0.95, 0.30)
    if action != aiverify.ActionAutoVerify {
        t.Errorf("expected AutoVerify, got %s", action)
    }
}

func TestClassifyAction_AutoReject(t *testing.T) {
    action := aiverify.ClassifyAction(0.15, 0.95, 0.30)
    if action != aiverify.ActionAutoReject {
        t.Errorf("expected AutoReject, got %s", action)
    }
}

func TestClassifyAction_QueueForReview(t *testing.T) {
    action := aiverify.ClassifyAction(0.60, 0.95, 0.30)
    if action != aiverify.ActionQueue {
        t.Errorf("expected Queue, got %s", action)
    }
}

func TestClassifyAction_ExactVerifyThreshold(t *testing.T) {
    action := aiverify.ClassifyAction(0.95, 0.95, 0.30)
    if action != aiverify.ActionAutoVerify {
        t.Errorf("expected AutoVerify at exact threshold, got %s", action)
    }
}

func TestClassifyAction_ExactRejectThreshold(t *testing.T) {
    action := aiverify.ClassifyAction(0.30, 0.95, 0.30)
    if action != aiverify.ActionQueue {
        t.Errorf("expected Queue at exact reject threshold, got %s", action)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd source-manager && GOWORK=off go test ./internal/aiverify/ -run TestClassify -v
```

Expected: FAIL — ClassifyAction not defined.

### Task 11: Implement worker

**Files:**
- Create: `source-manager/internal/aiverify/worker.go`

- [ ] **Step 1: Write the worker with Verifier interface and ClassifyAction**

```go
package aiverify

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// Action represents what to do with a verified record.
type Action string

const (
    ActionAutoVerify Action = "auto_verify"
    ActionAutoReject Action = "auto_reject"
    ActionQueue      Action = "queue"

    delayBetweenCalls = 500 * time.Millisecond
)

// Verifier abstracts the LLM call for testing.
type Verifier interface {
    Verify(ctx context.Context, input VerifyInput) (*VerifyResult, error)
}

// VerificationRecord holds a record fetched for verification.
type VerificationRecord struct {
    ID         string
    EntityType string // "person" or "band_office"
    Input      VerifyInput
}

// Repository abstracts DB operations for the worker.
type Repository interface {
    ListUnverifiedUnscoredPeople(ctx context.Context, limit int) ([]VerificationRecord, error)
    ListUnverifiedUnscoredBandOffices(ctx context.Context, limit int) ([]VerificationRecord, error)
    UpdatePersonVerificationResult(ctx context.Context, id string, confidence float64, issues string) error
    UpdateBandOfficeVerificationResult(ctx context.Context, id string, confidence float64, issues string) error
    VerifyPerson(ctx context.Context, id string) error
    VerifyBandOffice(ctx context.Context, id string) error
    AutoRejectPerson(ctx context.Context, id string) error
    AutoRejectBandOffice(ctx context.Context, id string) error
}
```

> **Note:** `VerifyPerson`/`VerifyBandOffice` are reused from the existing verification repository (PR #274). `AutoRejectPerson`/`AutoRejectBandOffice` are new — they do a direct DELETE without the lookup+verified-check that the existing `RejectPerson`/`RejectBandOffice` perform, since the worker already queried `WHERE verified=false`.

```go
// WorkerConfig holds verification worker settings.
type WorkerConfig struct {
    Interval            time.Duration
    BatchSize           int
    AutoVerifyThreshold float64
    AutoRejectThreshold float64
}

// Worker runs the AI verification loop.
type Worker struct {
    repo     Repository
    verifier Verifier
    config   WorkerConfig
    logger   infralogger.Logger
}

// NewWorker creates a new verification worker.
func NewWorker(repo Repository, verifier Verifier, cfg WorkerConfig, log infralogger.Logger) *Worker {
    return &Worker{
        repo:     repo,
        verifier: verifier,
        config:   cfg,
        logger:   log,
    }
}

// ClassifyAction determines the action based on confidence and thresholds.
func ClassifyAction(confidence, verifyThreshold, rejectThreshold float64) Action {
    if confidence >= verifyThreshold {
        return ActionAutoVerify
    }
    if confidence < rejectThreshold {
        return ActionAutoReject
    }
    return ActionQueue
}

// Run starts the verification ticker. Blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
    w.logger.Info("verification worker started",
        infralogger.String("interval", w.config.Interval.String()),
        infralogger.Int("batch_size", w.config.BatchSize),
    )

    ticker := time.NewTicker(w.config.Interval)
    defer ticker.Stop()

    // Run immediately on start, then on each tick
    w.tick(ctx)

    for {
        select {
        case <-ctx.Done():
            w.logger.Info("verification worker stopped")
            return
        case <-ticker.C:
            w.tick(ctx)
        }
    }
}

func (w *Worker) tick(ctx context.Context) {
    start := time.Now()

    people, err := w.repo.ListUnverifiedUnscoredPeople(ctx, w.config.BatchSize)
    if err != nil {
        w.logger.Error("verification: list unscored people", infralogger.Error(err))
        return
    }

    offices, officesErr := w.repo.ListUnverifiedUnscoredBandOffices(ctx, w.config.BatchSize)
    if officesErr != nil {
        w.logger.Error("verification: list unscored band offices", infralogger.Error(officesErr))
        return
    }

    records := append(people, offices...)
    processed := 0

    for i := range records {
        if ctx.Err() != nil {
            return
        }
        if i > 0 {
            time.Sleep(delayBetweenCalls)
        }
        w.processRecord(ctx, &records[i])
        processed++
    }

    w.logger.Info("verification.tick",
        infralogger.Int("batch_size", len(records)),
        infralogger.Int("processed", processed),
        infralogger.String("duration", time.Since(start).String()),
    )
}

func (w *Worker) processRecord(ctx context.Context, rec *VerificationRecord) {
    result, err := w.verifier.Verify(ctx, rec.Input)
    if err != nil {
        w.logger.Error("verification.error",
            infralogger.String("id", rec.ID),
            infralogger.String("type", rec.EntityType),
            infralogger.Error(err),
        )
        return
    }

    issuesJSON, marshalErr := json.Marshal(result.Issues)
    if marshalErr != nil {
        w.logger.Error("verification: marshal issues", infralogger.Error(marshalErr))
        return
    }

    if writeErr := w.updateResult(ctx, rec, result.Confidence, string(issuesJSON)); writeErr != nil {
        w.logger.Error("verification: update result", infralogger.Error(writeErr))
        return
    }

    action := ClassifyAction(result.Confidence, w.config.AutoVerifyThreshold, w.config.AutoRejectThreshold)
    w.applyAction(ctx, rec, action, result)
}

func (w *Worker) updateResult(ctx context.Context, rec *VerificationRecord, confidence float64, issues string) error {
    if rec.EntityType == "person" {
        return w.repo.UpdatePersonVerificationResult(ctx, rec.ID, confidence, issues)
    }
    return w.repo.UpdateBandOfficeVerificationResult(ctx, rec.ID, confidence, issues)
}

func (w *Worker) applyAction(ctx context.Context, rec *VerificationRecord, action Action, result *VerifyResult) {
    var err error

    switch action {
    case ActionAutoVerify:
        err = w.autoVerify(ctx, rec)
        if err == nil {
            w.logger.Info("verification.auto_verified",
                infralogger.String("id", rec.ID),
                infralogger.String("type", rec.EntityType),
                infralogger.Float64("confidence", result.Confidence),
            )
        }
    case ActionAutoReject:
        err = w.autoReject(ctx, rec)
        if err == nil {
            w.logger.Info("verification.auto_rejected",
                infralogger.String("id", rec.ID),
                infralogger.String("type", rec.EntityType),
                infralogger.Float64("confidence", result.Confidence),
            )
        }
    case ActionQueue:
        w.logger.Info("verification.queued",
            infralogger.String("id", rec.ID),
            infralogger.String("type", rec.EntityType),
            infralogger.Float64("confidence", result.Confidence),
            infralogger.Int("issue_count", len(result.Issues)),
        )
    }

    if err != nil {
        w.logger.Error(fmt.Sprintf("verification: %s failed", action),
            infralogger.String("id", rec.ID),
            infralogger.Error(err),
        )
    }
}

func (w *Worker) autoVerify(ctx context.Context, rec *VerificationRecord) error {
    if rec.EntityType == "person" {
        return w.repo.VerifyPerson(ctx, rec.ID)
    }
    return w.repo.VerifyBandOffice(ctx, rec.ID)
}

func (w *Worker) autoReject(ctx context.Context, rec *VerificationRecord) error {
    if rec.EntityType == "person" {
        return w.repo.AutoRejectPerson(ctx, rec.ID)
    }
    return w.repo.AutoRejectBandOffice(ctx, rec.ID)
}
```

- [ ] **Step 2: Run tests**

```bash
cd source-manager && GOWORK=off go test ./internal/aiverify/ -v
```

Expected: PASS

- [ ] **Step 3: Run lint**

```bash
cd source-manager && GOWORK=off golangci-lint run ./internal/aiverify/
```

Expected: 0 issues

- [ ] **Step 4: Commit**

```bash
git add source-manager/internal/aiverify/worker.go source-manager/internal/aiverify/worker_test.go
git commit -m "feat(source-manager): add AI verification worker with threshold logic (#298)"
```

### Task 12: Add LLM verifier implementation

**Files:**
- Create: `source-manager/internal/aiverify/llm_verifier.go`

- [ ] **Step 1: Write the LLM-backed Verifier**

```go
package aiverify

import (
    "context"

    "github.com/jonesrussell/north-cloud/infrastructure/provider/anthropic"
)

// LLMVerifier calls Claude to verify records.
type LLMVerifier struct {
    client *anthropic.Client
}

// NewLLMVerifier creates a new LLM-backed verifier.
func NewLLMVerifier(client *anthropic.Client) *LLMVerifier {
    return &LLMVerifier{client: client}
}

// Verify sends a record to Claude and parses the response.
func (v *LLMVerifier) Verify(ctx context.Context, input VerifyInput) (*VerifyResult, error) {
    resp, err := v.client.Generate(ctx, anthropic.GenerateRequest{
        SystemPrompt: SystemPrompt,
        UserPrompt:   BuildUserPrompt(input),
    })
    if err != nil {
        return nil, err
    }
    return ParseVerifyResponse(resp.Content)
}
```

- [ ] **Step 2: Run lint**

```bash
cd source-manager && GOWORK=off golangci-lint run ./internal/aiverify/
```

Expected: 0 issues

- [ ] **Step 3: Commit**

```bash
git add source-manager/internal/aiverify/llm_verifier.go
git commit -m "feat(source-manager): add LLM verifier implementation (#298)"
```

---

## Chunk 5: Repository Extensions + Config + Bootstrap

### Task 13: Add new repository methods

**Files:**
- Modify: `source-manager/internal/repository/verification.go`

- [ ] **Step 1: Add ListUnverifiedUnscored methods**

Add to `verification.go`. These JOIN with communities to get `community_name` and `province` for the LLM prompt:

```go
// ListUnverifiedUnscoredPeople returns people not yet evaluated by AI.
func (r *VerificationRepository) ListUnverifiedUnscoredPeople(
    ctx context.Context, limit int,
) ([]aiverify.VerificationRecord, error) {
    query := `SELECT p.id, p.name, p.role, p.email, p.phone, p.source_url,
              c.name AS community_name, c.province
        FROM people p
        JOIN communities c ON c.id = p.community_id
        WHERE p.verified = false AND p.verification_confidence IS NULL
        ORDER BY p.created_at ASC LIMIT $1`
    // ... scan rows, build VerificationRecord with VerifyInput populated
}
```

Similar method for band offices. Also add:

- `UpdatePersonVerificationResult(ctx, id, confidence, issues)` — `UPDATE people SET verification_confidence=$2, verification_issues=$3 WHERE id=$1`
- `UpdateBandOfficeVerificationResult(ctx, id, confidence, issues)` — same for band_offices
- `AutoRejectPerson(ctx, id)` — `DELETE FROM people WHERE id=$1` (direct DELETE, no lookup — deliberate optimization since the worker already queried `WHERE verified=false`)
- `AutoRejectBandOffice(ctx, id)` — `DELETE FROM band_offices WHERE id=$1`

> **Note:** `VerifyPerson`/`VerifyBandOffice` already exist from PR #274 — reuse them for auto-verify. Do NOT duplicate.

- [ ] **Step 2: Ensure VerificationRepository implements the worker's Repository interface**

Add a compile-time check:

```go
var _ aiverify.Repository = (*VerificationRepository)(nil)
```

- [ ] **Step 3: Run tests**

```bash
cd source-manager && GOWORK=off go test ./... -v
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add source-manager/internal/repository/verification.go
git commit -m "feat(source-manager): add AI verification repository methods (#298)"
```

### Task 14: Add config section

**Files:**
- Modify: `source-manager/internal/config/config.go`

- [ ] **Step 1: Add VerificationConfig struct**

After `AuthConfig` (line 61), add:

```go
type VerificationConfig struct {
    AIEnabled           bool          `env:"VERIFICATION_AI_ENABLED"           yaml:"ai_enabled"`
    Interval            time.Duration `env:"VERIFICATION_INTERVAL"             yaml:"interval"`
    BatchSize           int           `env:"VERIFICATION_BATCH_SIZE"           yaml:"batch_size"`
    AutoVerifyThreshold float64       `env:"VERIFICATION_AUTO_VERIFY_THRESHOLD" yaml:"auto_verify_threshold"`
    AutoRejectThreshold float64       `env:"VERIFICATION_AUTO_REJECT_THRESHOLD" yaml:"auto_reject_threshold"`
    AnthropicAPIKey     string        `env:"ANTHROPIC_API_KEY"                 yaml:"anthropic_api_key"`
    AnthropicModel      string        `env:"ANTHROPIC_MODEL"                   yaml:"anthropic_model"`
}
```

- [ ] **Step 2: Add Verification field to Config struct (line 27)**

```go
Verification VerificationConfig `yaml:"verification"`
```

- [ ] **Step 3: Add defaults in setDefaults() using direct assignment pattern**

At the end of `setDefaults()` (before closing brace, line 143):

```go
// Verification defaults (disabled by default)
if cfg.Verification.Interval == 0 {
    cfg.Verification.Interval = 5 * time.Minute
}
if cfg.Verification.BatchSize == 0 {
    cfg.Verification.BatchSize = 10
}
if cfg.Verification.AutoVerifyThreshold == 0 {
    cfg.Verification.AutoVerifyThreshold = 0.95
}
if cfg.Verification.AutoRejectThreshold == 0 {
    cfg.Verification.AutoRejectThreshold = 0.30
}
if cfg.Verification.AnthropicModel == "" {
    cfg.Verification.AnthropicModel = "claude-haiku-4-5-20251001"
}
// Note: cfg.Verification.AIEnabled defaults to false (feature flag)
```

- [ ] **Step 4: Run tests + lint**

```bash
cd source-manager && GOWORK=off go test ./... && GOWORK=off golangci-lint run
```

Expected: PASS, 0 issues

- [ ] **Step 5: Commit**

```bash
git add source-manager/internal/config/config.go
git commit -m "feat(source-manager): add verification AI config (#298)"
```

### Task 15: Wire worker into bootstrap

**Files:**
- Modify: `source-manager/internal/bootstrap/app.go`
- Modify: `source-manager/internal/bootstrap/server.go`

- [ ] **Step 1: Add verification worker startup to app.go**

The actual `Start()` function structure (lines 16-70) has `server.Run()` as a blocking call at line 63. Insert the worker between Phase 3 (event publisher) and Phase 4 (server.Run):

```go
// Phase 3: Setup event publisher (optional)
publisher := SetupEventPublisher(cfg, log)

// Phase 4: Setup HTTP server
server := SetupHTTPServer(cfg, db, publisher, log)

// Phase 4.5: Verification worker (optional, disabled by default)
var verifyCancel context.CancelFunc
if cfg.Verification.AIEnabled {
    verifyClient := anthropic.New(cfg.Verification.AnthropicAPIKey, cfg.Verification.AnthropicModel)
    verifier := aiverify.NewLLMVerifier(verifyClient)
    verificationRepo := repository.NewVerificationRepository(db.DB(), log)
    worker := aiverify.NewWorker(verificationRepo, verifier, aiverify.WorkerConfig{
        Interval:            cfg.Verification.Interval,
        BatchSize:           cfg.Verification.BatchSize,
        AutoVerifyThreshold: cfg.Verification.AutoVerifyThreshold,
        AutoRejectThreshold: cfg.Verification.AutoRejectThreshold,
    }, log)

    var verifyCtx context.Context
    verifyCtx, verifyCancel = context.WithCancel(context.Background())
    go worker.Run(verifyCtx)
    defer verifyCancel()
}

log.Info("Starting HTTP server", ...)

// Phase 5: Run server (blocks until shutdown)
if runErr := server.Run(); runErr != nil {
```

Note: `verificationRepo` is instantiated here in `app.go`, separate from the one in `server.go` (which is used for HTTP handlers). The worker needs its own instance since it runs independently.

Add imports: `context`, `anthropic`, `aiverify`, `repository`.

- [ ] **Step 2: Vendor sync**

```bash
cd source-manager && GOWORK=off go mod tidy && task vendor
```

- [ ] **Step 3: Run full test suite + lint**

```bash
cd source-manager && GOWORK=off go test ./... && GOWORK=off golangci-lint run
```

Expected: PASS, 0 issues

- [ ] **Step 4: Commit**

```bash
git add source-manager/internal/bootstrap/ source-manager/go.mod source-manager/go.sum source-manager/vendor/
git commit -m "feat(source-manager): wire AI verification worker into bootstrap (#298)"
```

---

## Chunk 6: Final Validation

### Task 16: Full integration test

- [ ] **Step 1: Run full test suite across affected services**

```bash
cd source-manager && GOWORK=off go test ./... -v
cd ai-observer && GOWORK=off go test ./... -v
cd infrastructure && GOWORK=off go test ./... -v
```

Expected: All PASS

- [ ] **Step 2: Run lint across affected services**

```bash
cd source-manager && GOWORK=off golangci-lint run
cd ai-observer && GOWORK=off golangci-lint run
cd infrastructure && GOWORK=off golangci-lint run
```

Expected: 0 issues each

- [ ] **Step 3: Verify build**

```bash
cd source-manager && GOWORK=off go build -o /dev/null ./...
```

Expected: Build succeeds

- [ ] **Step 4: Create PR**

```bash
git push -u origin claude/ai-verification-298
gh pr create --title "feat(source-manager): AI-assisted verification of scraped data" \
  --body "$(cat <<'EOF'
## Summary
- Background worker evaluates scraped Person/BandOffice records using Claude API
- Auto-verifies high-confidence (>= 0.95), auto-rejects low-confidence (< 0.30)
- Middle band (0.30-0.95) queued for human review with AI-provided issues
- Disabled by default (VERIFICATION_AI_ENABLED=false)
- Shared Anthropic client extracted to infrastructure/provider/anthropic/

## Test plan
- [ ] Unit tests for prompt rendering + response parsing
- [ ] Unit tests for threshold logic (auto-verify/reject/queue)
- [ ] Unit tests for Anthropic client
- [ ] Full test suite passes across source-manager, ai-observer, infrastructure
- [ ] Lint clean across all affected services
- [ ] Manual test: enable worker, verify it processes records correctly

Closes #298

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```
