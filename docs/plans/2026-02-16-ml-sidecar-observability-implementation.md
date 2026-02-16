# ML Sidecar Observability Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add comprehensive structured logging from the classifier for every ML sidecar call, then build an enhanced Grafana dashboard that surfaces performance, classification insights, and error forensics.

**Architecture:** Classifier-side structured logging via `infralogger`. Each sidecar call emits an Info log line with content context, decision details, latency, and response metadata. The Grafana dashboard queries Loki for these structured fields. No new infrastructure needed.

**Tech Stack:** Go 1.25+ (classifier), Loki/LogQL (Grafana queries), JSON dashboard provisioning

**Design Doc:** `docs/plans/2026-02-16-ml-sidecar-observability-design.md`

---

### Task 1: Add Decision Context Fields to Domain Types

**Files:**
- Modify: `classifier/internal/domain/classification.go:54-108`

**Step 1: Add fields to all 5 result types**

Add these 4 fields to `CrimeResult`, `MiningResult`, `CoforgeResult`, `EntertainmentResult`, `AnishinaabeResult`:

```go
// In CrimeResult (after ReviewRequired field, line 108):
DecisionPath     string  `json:"decision_path,omitempty"`
MLConfidenceRaw  float64 `json:"ml_confidence_raw,omitempty"`
RuleTriggered    string  `json:"rule_triggered,omitempty"`
ProcessingTimeMs int64   `json:"processing_time_ms,omitempty"`
```

For `CrimeResult` specifically, the JSON tags must match the existing pattern (it already uses `street_crime_relevance` for Relevance). The new fields use standard snake_case.

For `MiningResult`, `CoforgeResult`, `EntertainmentResult`, `AnishinaabeResult`: same 4 fields, same JSON tags.

**Step 2: Run tests to verify no breakage**

Run: `cd classifier && go test ./internal/domain/... -v`
Expected: PASS (no logic change, only additive fields)

Run: `cd classifier && go test ./... -count=1 2>&1 | tail -5`
Expected: all tests pass (additive fields don't break existing serialization)

**Step 3: Run linter**

Run: `cd classifier && golangci-lint run ./internal/domain/...`
Expected: PASS

**Step 4: Commit**

```bash
git add classifier/internal/domain/classification.go
git commit -m "feat(classifier): add decision context fields to ML result domain types"
```

---

### Task 2: Add Latency + Response Size Tracking to Transport Layer

**Files:**
- Modify: `classifier/internal/mltransport/transport.go`
- Create: `classifier/internal/mltransport/transport_test.go`

**Step 1: Write tests for enhanced DoClassify**

Create `classifier/internal/mltransport/transport_test.go`:

```go
package mltransport_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/mltransport"
)

type testResponse struct {
	Result string `json:"result"`
}

func TestDoClassify_ReturnsLatencyAndSize(t *testing.T) {
	t.Helper()

	resp := testResponse{Result: "ok"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	req := &mltransport.ClassifyRequest{Title: "test", Body: "body"}
	var got testResponse
	latencyMs, sizeBytes, err := mltransport.DoClassify(context.Background(), server.URL, req, &got)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if latencyMs < 0 {
		t.Errorf("expected non-negative latency, got %d", latencyMs)
	}
	if sizeBytes <= 0 {
		t.Errorf("expected positive response size, got %d", sizeBytes)
	}
	if got.Result != "ok" {
		t.Errorf("expected result 'ok', got %q", got.Result)
	}
}

func TestDoClassify_ErrorReturnsLatency(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	req := &mltransport.ClassifyRequest{Title: "test", Body: "body"}
	var got testResponse
	latencyMs, _, err := mltransport.DoClassify(context.Background(), server.URL, req, &got)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if latencyMs < 0 {
		t.Errorf("expected non-negative latency even on error, got %d", latencyMs)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd classifier && go test ./internal/mltransport/... -v`
Expected: FAIL (DoClassify signature doesn't match yet)

**Step 3: Update DoClassify to return latency and response size**

Replace the current `DoClassify` function in `classifier/internal/mltransport/transport.go`:

```go
// DoClassify sends POST /classify to baseURL with req, decoding the response into respPtr.
// Returns the HTTP round-trip latency in milliseconds, the response body size in bytes, and any error.
// respPtr must be a pointer to a struct that matches the ML service response.
func DoClassify(ctx context.Context, baseURL string, req *ClassifyRequest, respPtr any) (latencyMs int64, responseSizeBytes int, err error) {
	body, marshalErr := json.Marshal(req)
	if marshalErr != nil {
		return 0, 0, fmt.Errorf("marshal request: %w", marshalErr)
	}

	httpReq, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/classify", bytes.NewReader(body))
	if reqErr != nil {
		return 0, 0, fmt.Errorf("create request: %w", reqErr)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: defaultTimeout}

	start := time.Now()
	resp, doErr := client.Do(httpReq)
	latencyMs = time.Since(start).Milliseconds()
	if doErr != nil {
		return latencyMs, 0, fmt.Errorf("http request: %w", doErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return latencyMs, 0, fmt.Errorf("ml service returned %d", resp.StatusCode)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return latencyMs, 0, fmt.Errorf("read response: %w", readErr)
	}
	responseSizeBytes = len(respBody)

	if unmarshalErr := json.Unmarshal(respBody, respPtr); unmarshalErr != nil {
		return latencyMs, responseSizeBytes, fmt.Errorf("decode response: %w", unmarshalErr)
	}

	return latencyMs, responseSizeBytes, nil
}
```

Add `"io"` to imports.

**Step 4: Run transport tests to verify they pass**

Run: `cd classifier && go test ./internal/mltransport/... -v`
Expected: PASS

**Step 5: Update all 5 ML client callers**

Each ML client calls `mltransport.DoClassify()` and currently ignores the return. Now it returns `(latencyMs, responseSizeBytes, error)`. Update each:

**`classifier/internal/mlclient/client.go`** - Update `Classify()`:
Find: `if err := mltransport.DoClassify(ctx, c.baseURL, req, &result); err != nil {`
Replace with: `if _, _, err := mltransport.DoClassify(ctx, c.baseURL, req, &result); err != nil {`

**`classifier/internal/miningmlclient/client.go`** - Same pattern.

**`classifier/internal/coforgemlclient/client.go`** - Same pattern.

**`classifier/internal/entertainmentmlclient/client.go`** - Same pattern.

**`classifier/internal/anishinaabemlclient/client.go`** - Same pattern.

**Step 6: Run full test suite**

Run: `cd classifier && go test ./... -count=1 2>&1 | tail -10`
Expected: all tests pass

**Step 7: Run linter**

Run: `cd classifier && golangci-lint run`
Expected: PASS

**Step 8: Commit**

```bash
git add classifier/internal/mltransport/transport.go classifier/internal/mltransport/transport_test.go \
  classifier/internal/mlclient/client.go classifier/internal/miningmlclient/client.go \
  classifier/internal/coforgemlclient/client.go classifier/internal/entertainmentmlclient/client.go \
  classifier/internal/anishinaabemlclient/client.go
git commit -m "feat(classifier): add latency and response size tracking to ML transport"
```

---

### Task 3: Populate Decision Context in Crime Classifier

**Files:**
- Modify: `classifier/internal/classifier/crime.go:47-60,104-167`
- Modify: `classifier/internal/classifier/classifier.go:406-417` (convertCrimeResult)
- Modify: `classifier/internal/classifier/crime_test.go`

The crime classifier is unique: it has a LOCAL `CrimeResult` (crime.go:47-60) that already contains `RuleRelevance`, `RuleConfidence`, `MLRelevance`, `MLConfidence` fields. These are dropped by `convertCrimeResult`. We need to:
1. Add `DecisionPath`, `ProcessingTimeMs` to the local CrimeResult
2. Record `DecisionPath` in `applyDecisionLogic`
3. Record `ProcessingTimeMs` from the ML response
4. Update `convertCrimeResult` to copy all new fields to domain type
5. Derive `RuleTriggered` from the existing RuleRelevance (crime rules don't have named patterns, so use rule relevance as proxy)

**Step 1: Update crime_test.go to verify decision context**

Add to `classifier/internal/classifier/crime_test.go` (after existing tests):

```go
func TestCrimeClassifier_DecisionContext_BothAgree(t *testing.T) {
	t.Helper()

	mlMock := &mockMLClient{
		response: &mlclient.ClassifyResponse{
			Relevance:           "core_street_crime",
			RelevanceConfidence: 0.85,
			CrimeTypes:          []string{"violent_crime"},
			Location:            "local_canada",
			ProcessingTimeMs:    42,
		},
	}

	sc := NewCrimeClassifier(mlMock, &mockLogger{}, true)
	raw := &domain.RawContent{
		ID:      "test-dc-1",
		Title:   "Man charged with murder after stabbing",
		RawText: "Police arrested a suspect in downtown.",
	}

	result, err := sc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DecisionPath != "both_agree" {
		t.Errorf("expected decision_path 'both_agree', got %q", result.DecisionPath)
	}
	if result.MLConfidence == 0 {
		t.Error("expected non-zero MLConfidence")
	}
	if result.ProcessingTimeMs != 42 {
		t.Errorf("expected processing_time_ms 42, got %d", result.ProcessingTimeMs)
	}
}

func TestCrimeClassifier_DecisionContext_RulesOnly(t *testing.T) {
	t.Helper()

	sc := NewCrimeClassifier(nil, &mockLogger{}, true)
	raw := &domain.RawContent{
		ID:      "test-dc-2",
		Title:   "Man charged with murder after stabbing",
		RawText: "Police arrested a suspect.",
	}

	result, err := sc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DecisionPath != "rules_only" {
		t.Errorf("expected decision_path 'rules_only', got %q", result.DecisionPath)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd classifier && go test ./internal/classifier/ -run TestCrimeClassifier_DecisionContext -v`
Expected: FAIL (DecisionPath field doesn't exist yet on local CrimeResult)

**Step 3: Add DecisionPath and ProcessingTimeMs to local CrimeResult**

In `crime.go`, add to the local CrimeResult struct (after line 59):

```go
DecisionPath     string `json:"decision_path,omitempty"`
ProcessingTimeMs int64  `json:"processing_time_ms,omitempty"`
```

**Step 4: Record DecisionPath in applyDecisionLogic**

In `crime.go:applyDecisionLogic`, add `result.DecisionPath = "..."` in each switch case:

- Case both agree (line 138-142): `result.DecisionPath = "both_agree"`
- Case rule core + ML not_crime (line 144-149): `result.DecisionPath = "rule_override"`
- Case rule core + ML unavailable (line 151-155): `result.DecisionPath = "rules_only"`
- Case ML override (line 157-161): `result.DecisionPath = "ml_override"`
- Default (line 163-166): `result.DecisionPath = "default"`

**Step 5: Record ProcessingTimeMs in mergeResults**

In `crime.go:mergeResults`, inside the `if ml != nil` block (after line 114):

```go
result.ProcessingTimeMs = ml.ProcessingTimeMs
```

**Step 6: Update convertCrimeResult to copy decision context fields**

In `classifier.go:convertCrimeResult` (line 406-417), add the new fields:

```go
func convertCrimeResult(sc *CrimeResult) *domain.CrimeResult {
	return &domain.CrimeResult{
		Relevance:           sc.Relevance,
		SubLabel:            sc.SubLabel,
		CrimeTypes:          sc.CrimeTypes,
		LocationSpecificity: sc.LocationSpecificity,
		FinalConfidence:     sc.FinalConfidence,
		HomepageEligible:    sc.HomepageEligible,
		CategoryPages:       sc.CategoryPages,
		ReviewRequired:      sc.ReviewRequired,
		DecisionPath:        sc.DecisionPath,
		MLConfidenceRaw:     sc.MLConfidence,
		RuleTriggered:       sc.RuleRelevance,
		ProcessingTimeMs:    sc.ProcessingTimeMs,
	}
}
```

Note: `MLConfidenceRaw` maps from the local `MLConfidence` field (raw ML score before decision matrix). `RuleTriggered` maps from `RuleRelevance` (the rule result - e.g., "core_street_crime" or "not_crime" - since crime rules don't have named patterns).

**Step 7: Run tests**

Run: `cd classifier && go test ./internal/classifier/ -run TestCrimeClassifier -v`
Expected: PASS

Run: `cd classifier && go test ./... -count=1 2>&1 | tail -5`
Expected: all pass

**Step 8: Run linter**

Run: `cd classifier && golangci-lint run ./internal/classifier/...`
Expected: PASS

**Step 9: Commit**

```bash
git add classifier/internal/classifier/crime.go classifier/internal/classifier/classifier.go \
  classifier/internal/classifier/crime_test.go
git commit -m "feat(classifier): populate decision context fields in crime hybrid classifier"
```

---

### Task 4: Populate Decision Context in Mining Classifier

**Files:**
- Modify: `classifier/internal/classifier/mining.go:65-117`
- Modify: `classifier/internal/classifier/mining_test.go`

Mining returns `*domain.MiningResult` directly (no local type conversion), so we populate the domain type fields directly.

**Step 1: Add decision context test**

Add to `classifier/internal/classifier/mining_test.go`:

```go
func TestMiningClassifier_DecisionContext_BothAgree(t *testing.T) {
	t.Helper()

	mlMock := &mockMiningMLClient{
		response: &miningmlclient.ClassifyResponse{
			Relevance:           "core_mining",
			RelevanceConfidence: 0.88,
			MiningStage:         "exploration",
			Commodities:         []string{"gold"},
			Location:            "local_canada",
			ProcessingTimeMs:    35,
			ModelVersion:        "v1",
		},
	}

	mc := NewMiningClassifier(mlMock, &mockLogger{}, true)
	raw := &domain.RawContent{
		ID:      "test-mc-1",
		Title:   "Gold mining drill results",
		RawText: "New exploration drilling at site.",
	}

	result, err := mc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DecisionPath != "both_agree" {
		t.Errorf("expected decision_path 'both_agree', got %q", result.DecisionPath)
	}
	if result.MLConfidenceRaw != 0.88 {
		t.Errorf("expected ml_confidence_raw 0.88, got %f", result.MLConfidenceRaw)
	}
	if result.ProcessingTimeMs != 35 {
		t.Errorf("expected processing_time_ms 35, got %d", result.ProcessingTimeMs)
	}
}
```

**Step 2: Run to verify failure**

Run: `cd classifier && go test ./internal/classifier/ -run TestMiningClassifier_DecisionContext -v`
Expected: FAIL

**Step 3: Populate fields in mining.go**

In `mergeResults` (line 65-81), inside `if ml != nil` block, add:

```go
result.MLConfidenceRaw = ml.RelevanceConfidence
result.ProcessingTimeMs = ml.ProcessingTimeMs
```

Also set `result.RuleTriggered = rule.relevance` after the result initialization.

In `applyDecisionLogic` (line 86-117), add `result.DecisionPath = "..."` in each switch case:
- Both core (line 88-91): `result.DecisionPath = "both_agree"`
- Rule core + ML not (line 93-96): `result.DecisionPath = "rule_override"`
- Rule core + ML unavailable (line 98-101): `result.DecisionPath = "rules_only"`
- ML override (line 103-106): `result.DecisionPath = "ml_override"`
- Peripheral + ML core (line 108-111): `result.DecisionPath = "ml_upgrade"`
- Default (line 113-116): `result.DecisionPath = "default"`

**Step 4: Run tests**

Run: `cd classifier && go test ./internal/classifier/ -run TestMiningClassifier -v`
Expected: PASS

**Step 5: Run linter**

Run: `cd classifier && golangci-lint run ./internal/classifier/mining.go`
Expected: PASS

**Step 6: Commit**

```bash
git add classifier/internal/classifier/mining.go classifier/internal/classifier/mining_test.go
git commit -m "feat(classifier): populate decision context fields in mining hybrid classifier"
```

---

### Task 5: Populate Decision Context in Coforge Classifier

**Files:**
- Modify: `classifier/internal/classifier/coforge.go:59-118`
- Modify: `classifier/internal/classifier/coforge_test.go`

Same pattern as mining. Coforge returns `*domain.CoforgeResult` directly.

**Step 1: Add decision context test**

Add to `classifier/internal/classifier/coforge_test.go` (check test file for existing mock pattern):

```go
func TestCoforgeClassifier_DecisionContext_BothAgree(t *testing.T) {
	t.Helper()

	mlMock := &mockCoforgeMLClient{
		response: &coforgemlclient.ClassifyResponse{
			Relevance:           "core_coforge",
			RelevanceConfidence: 0.82,
			Audience:            "developers",
			AudienceConfidence:  0.9,
			Topics:              []string{"ai"},
			Industries:          []string{"tech"},
			ProcessingTimeMs:    28,
			ModelVersion:        "v1",
		},
	}

	cc := NewCoforgeClassifier(mlMock, &mockLogger{}, true)
	raw := &domain.RawContent{
		ID:      "test-cf-1",
		Title:   "Startup launches new AI developer SDK",
		RawText: "The open source tool targets developers.",
	}

	result, err := cc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DecisionPath != "both_agree" {
		t.Errorf("expected decision_path 'both_agree', got %q", result.DecisionPath)
	}
	if result.MLConfidenceRaw != 0.82 {
		t.Errorf("expected ml_confidence_raw 0.82, got %f", result.MLConfidenceRaw)
	}
	if result.ProcessingTimeMs != 28 {
		t.Errorf("expected processing_time_ms 28, got %d", result.ProcessingTimeMs)
	}
}
```

**Step 2: Run to verify failure**

Run: `cd classifier && go test ./internal/classifier/ -run TestCoforgeClassifier_DecisionContext -v`
Expected: FAIL

**Step 3: Populate fields in coforge.go**

In `mergeResults` (line 59-76): add `result.RuleTriggered = rule.relevance` and inside `if ml != nil`: `result.MLConfidenceRaw = ml.RelevanceConfidence` and `result.ProcessingTimeMs = ml.ProcessingTimeMs`.

In `applyDecisionLogic` (line 81-118): add `result.DecisionPath` in each case:
- Both core: `"both_agree"`
- Rule core + ML not: `"rule_override"`
- Rule core + ML unavailable: `"rules_only"`
- ML override: `"ml_override"`
- Peripheral + ML core: `"ml_upgrade"`
- Default: `"default"`

**Step 4: Run tests + linter**

Run: `cd classifier && go test ./internal/classifier/ -run TestCoforgeClassifier -v`
Expected: PASS

Run: `cd classifier && golangci-lint run ./internal/classifier/coforge.go`

**Step 5: Commit**

```bash
git add classifier/internal/classifier/coforge.go classifier/internal/classifier/coforge_test.go
git commit -m "feat(classifier): populate decision context fields in coforge hybrid classifier"
```

---

### Task 6: Populate Decision Context in Entertainment Classifier

**Files:**
- Modify: `classifier/internal/classifier/entertainment.go:59-119`
- Create: `classifier/internal/classifier/entertainment_test.go` (missing - check if exists, create if not)

Same pattern as mining/coforge. Returns `*domain.EntertainmentResult` directly.

**Step 1: Check for existing test file**

Check `classifier/internal/classifier/entertainment_test.go` - if it exists, add tests. If not, create with mock pattern matching other test files.

**Step 2: Add decision context test**

```go
func TestEntertainmentClassifier_DecisionContext_BothAgree(t *testing.T) {
	t.Helper()

	mlMock := &mockEntertainmentMLClient{
		response: &entertainmentmlclient.ClassifyResponse{
			Relevance:           "core_entertainment",
			RelevanceConfidence: 0.80,
			Categories:          []string{"film"},
			ProcessingTimeMs:    22,
			ModelVersion:        "v1",
		},
	}

	ec := NewEntertainmentClassifier(mlMock, &mockLogger{}, true)
	raw := &domain.RawContent{
		ID:      "test-ent-1",
		Title:   "New Marvel movie breaks box office records",
		RawText: "The film premiered this weekend.",
	}

	result, err := ec.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DecisionPath != "both_agree" {
		t.Errorf("expected decision_path 'both_agree', got %q", result.DecisionPath)
	}
	if result.ProcessingTimeMs != 22 {
		t.Errorf("expected processing_time_ms 22, got %d", result.ProcessingTimeMs)
	}
}
```

**Step 3: Populate fields in entertainment.go**

In `mergeResults` (line 59-75): add `result.RuleTriggered = rule.relevance` and inside `if ml != nil`: `result.MLConfidenceRaw = ml.RelevanceConfidence` and `result.ProcessingTimeMs = ml.ProcessingTimeMs`.

In `applyDecisionLogic` (line 82-119): add `result.DecisionPath` in each case (same labels as mining).

**Step 4: Run tests + linter, commit**

```bash
git add classifier/internal/classifier/entertainment.go classifier/internal/classifier/entertainment_test.go
git commit -m "feat(classifier): populate decision context fields in entertainment hybrid classifier"
```

---

### Task 7: Populate Decision Context in Anishinaabe Classifier

**Files:**
- Modify: `classifier/internal/classifier/anishinaabe.go:59-118`
- Create: `classifier/internal/classifier/anishinaabe_test.go` (missing - check if exists, create if not)

Same pattern as mining/entertainment. Returns `*domain.AnishinaabeResult` directly.

**Step 1-4: Same pattern as Task 6**

Follow identical steps as entertainment. The decision matrix has the same structure (noted by `//nolint:dupl` comment).

**Step 5: Commit**

```bash
git add classifier/internal/classifier/anishinaabe.go classifier/internal/classifier/anishinaabe_test.go
git commit -m "feat(classifier): populate decision context fields in anishinaabe hybrid classifier"
```

---

### Task 8: Add Structured Logging Helpers

**Files:**
- Create: `classifier/internal/classifier/observability.go`
- Create: `classifier/internal/classifier/observability_test.go`

**Step 1: Write tests for helper functions**

Create `classifier/internal/classifier/observability_test.go`:

```go
package classifier

import "testing"

func TestTruncateWords(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		input    string
		n        int
		expected string
	}{
		{"short title", "Hello world", 10, "Hello world"},
		{"exact limit", "one two three", 3, "one two three"},
		{"truncate", "one two three four five six", 3, "one two three..."},
		{"empty", "", 10, ""},
		{"single word", "Hello", 1, "Hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			got := truncateWords(tt.input, tt.n)
			if got != tt.expected {
				t.Errorf("truncateWords(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.expected)
			}
		})
	}
}

func TestClassifyErrorType(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		errMsg   string
		expected string
	}{
		{"timeout", "http request: context deadline exceeded", "timeout"},
		{"5xx", "ml service returned 503", "5xx"},
		{"4xx", "ml service returned 400", "4xx"},
		{"connection", "http request: dial tcp: connection refused", "connection"},
		{"decode", "decode response: unexpected EOF", "decode"},
		{"unknown", "something weird happened", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			got := classifyErrorType(tt.errMsg)
			if got != tt.expected {
				t.Errorf("classifyErrorType(%q) = %q, want %q", tt.errMsg, got, tt.expected)
			}
		})
	}
}
```

**Step 2: Run to verify failure**

Run: `cd classifier && go test ./internal/classifier/ -run "TestTruncateWords|TestClassifyErrorType" -v`
Expected: FAIL

**Step 3: Implement helpers**

Create `classifier/internal/classifier/observability.go`:

```go
package classifier

import (
	"strings"
)

const titleExcerptWordLimit = 10

// truncateWords returns the first n words of s, appending "..." if truncated.
func truncateWords(s string, n int) string {
	words := strings.Fields(s)
	if len(words) <= n {
		return s
	}
	return strings.Join(words[:n], " ") + "..."
}

// classifyErrorType categorizes an ML sidecar error message into a type for dashboard filtering.
func classifyErrorType(errMsg string) string {
	lower := strings.ToLower(errMsg)
	switch {
	case strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "timeout"):
		return "timeout"
	case strings.Contains(lower, "returned 5"):
		return "5xx"
	case strings.Contains(lower, "returned 4"):
		return "4xx"
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "dial tcp") ||
		strings.Contains(lower, "no such host"):
		return "connection"
	case strings.Contains(lower, "decode") || strings.Contains(lower, "unmarshal") ||
		strings.Contains(lower, "eof"):
		return "decode"
	default:
		return "unknown"
	}
}
```

**Step 4: Run tests**

Run: `cd classifier && go test ./internal/classifier/ -run "TestTruncateWords|TestClassifyErrorType" -v`
Expected: PASS

**Step 5: Run linter**

Run: `cd classifier && golangci-lint run ./internal/classifier/observability.go`
Expected: PASS

**Step 6: Commit**

```bash
git add classifier/internal/classifier/observability.go classifier/internal/classifier/observability_test.go
git commit -m "feat(classifier): add observability helper functions for ML sidecar logging"
```

---

### Task 9: Wire Structured Logging into runOptionalClassifiers

**Files:**
- Modify: `classifier/internal/classifier/classifier.go:108-109,200-316`

This is the core wiring task. We modify `classifyOptionalForPublishable` to pass `contentType` down, and modify `runOptionalClassifiers` to:
1. Wrap each sidecar call with `time.Now()` for latency
2. Emit structured Info/Warn logs after each call

**Step 1: Update classifyOptionalForPublishable to pass contentType**

The function at line 206 already receives `contentType` as a parameter. It calls `runOptionalClassifiers(ctx, raw)` at line 234. Update to pass contentType:

```go
return c.runOptionalClassifiers(ctx, raw, contentType)
```

Also update `runLocationOnly` and `runCrimeOnly` if they should log too (optional - they handle edge cases).

**Step 2: Update runOptionalClassifiers signature**

Change line 240 from:
```go
func (c *Classifier) runOptionalClassifiers(
	ctx context.Context, raw *domain.RawContent,
```
To:
```go
func (c *Classifier) runOptionalClassifiers(
	ctx context.Context, raw *domain.RawContent, contentType string,
```

**Step 3: Add structured logging for each sidecar call**

For each sidecar in `runOptionalClassifiers`, wrap with timing and emit log. Example for crime:

```go
var crimeResult *domain.CrimeResult
if c.crime != nil {
	crimeStart := time.Now()
	scResult, scErr := c.crime.Classify(ctx, raw)
	crimeLatencyMs := time.Since(crimeStart).Milliseconds()
	if scErr != nil {
		c.logSidecarError("crime-ml", raw, contentType, scErr, crimeLatencyMs)
	} else if scResult != nil {
		crimeResult = convertCrimeResult(scResult)
		c.logSidecarSuccess("crime-ml", raw, contentType, crimeResult.Relevance,
			crimeResult.FinalConfidence, crimeResult.MLConfidenceRaw, crimeResult.RuleTriggered,
			crimeResult.DecisionPath, crimeLatencyMs, crimeResult.ProcessingTimeMs,
			crimeResult.ModelVersion)
	}
}
```

Repeat the same pattern for mining, coforge, entertainment, anishinaabe (but not location - it's internal, not an ML sidecar).

**Step 4: Add logSidecarSuccess and logSidecarError methods**

Add to `observability.go`:

```go
// logSidecarSuccess emits a structured Info log for a successful ML sidecar classification.
func (c *Classifier) logSidecarSuccess(
	sidecar string, raw *domain.RawContent, contentType string,
	relevance string, confidence, mlConfRaw float64,
	ruleTriggered, decisionPath string,
	latencyMs, processingTimeMs int64, modelVersion string,
) {
	c.logger.Info("ML sidecar classification complete",
		infralogger.String("sidecar", sidecar),
		infralogger.String("content_id", raw.ID),
		infralogger.String("content_type", contentType),
		infralogger.String("source", raw.SourceName),
		infralogger.String("title_excerpt", truncateWords(raw.Title, titleExcerptWordLimit)),
		infralogger.String("relevance", relevance),
		infralogger.Float64("confidence", confidence),
		infralogger.Float64("ml_confidence_raw", mlConfRaw),
		infralogger.String("rule_triggered", ruleTriggered),
		infralogger.String("decision_path", decisionPath),
		infralogger.Int64("latency_ms", latencyMs),
		infralogger.Int64("processing_time_ms", processingTimeMs),
		infralogger.String("model_version", modelVersion),
		infralogger.String("outcome", "success"),
	)
}

// logSidecarError emits a structured Warn log for a failed ML sidecar classification.
func (c *Classifier) logSidecarError(
	sidecar string, raw *domain.RawContent, contentType string,
	err error, latencyMs int64,
) {
	c.logger.Warn("ML sidecar classification failed",
		infralogger.String("sidecar", sidecar),
		infralogger.String("content_id", raw.ID),
		infralogger.String("content_type", contentType),
		infralogger.String("source", raw.SourceName),
		infralogger.String("title_excerpt", truncateWords(raw.Title, titleExcerptWordLimit)),
		infralogger.String("outcome", "error"),
		infralogger.String("error_type", classifyErrorType(err.Error())),
		infralogger.String("error_detail", err.Error()),
		infralogger.Int64("latency_ms", latencyMs),
	)
}
```

Add required import: `infralogger "github.com/north-cloud/infrastructure/logger"` to observability.go.

**Step 5: Extract ModelVersion from each result type for the success log**

For mining: `miningResult.ModelVersion`
For coforge: `coforgeResult.ModelVersion`
For entertainment: `entertainmentResult.ModelVersion`
For anishinaabe: `anishinaabeResult.ModelVersion`

Crime's ModelVersion isn't on the local CrimeResult - it's empty. Check if the domain CrimeResult has ModelVersion... No, it doesn't have one per the struct. For crime, pass `""` for modelVersion, or we could add it. For now, pass empty string.

**Step 6: Run full test suite**

Run: `cd classifier && go test ./... -count=1 2>&1 | tail -10`
Expected: all pass

**Step 7: Run linter**

Run: `cd classifier && golangci-lint run`
Expected: PASS. Watch for:
- `funlen` on runOptionalClassifiers (it's already annotated with nolint:gocognit)
- If it exceeds line limits, the logging is in helper methods so the main function stays reasonable

**Step 8: Commit**

```bash
git add classifier/internal/classifier/classifier.go classifier/internal/classifier/observability.go
git commit -m "feat(classifier): wire structured logging for all ML sidecar classifications"
```

---

### Task 10: Build and Deploy Enhanced Grafana Dashboard

**Files:**
- Modify: `infrastructure/grafana/provisioning/dashboards/north-cloud-ml-sidecars.json`

**Step 1: Read the existing dashboard JSON**

Read: `infrastructure/grafana/provisioning/dashboards/north-cloud-ml-sidecars.json`
Understand the existing panel IDs, grid positions, and datasource UIDs.

**Step 2: Build the new dashboard JSON**

Replace the existing dashboard with 6 rows of panels. Use the Loki datasource UID from the existing file (should be `"loki"`).

Key LogQL patterns for each panel type:

**Stat - Sidecar Status:**
```
count_over_time({service="classifier"} | json | msg="ML sidecar classification complete" | sidecar="crime-ml" [5m])
```

**Time Series - P95 Latency:**
```
quantile_over_time(0.95, {service="classifier"} | json | msg=`ML sidecar classification complete` | sidecar=`$sidecar` | unwrap latency_ms [$__interval]) by (sidecar)
```

**Pie Chart - Relevance Distribution:**
```
sum by (relevance) (count_over_time({service="classifier"} | json | msg=`ML sidecar classification complete` | sidecar="crime-ml" [$__range]))
```

**Bar Chart - Decision Path:**
```
sum by (decision_path) (count_over_time({service="classifier"} | json | msg=`ML sidecar classification complete` [$__range]))
```

**Table - Recent Errors:**
```
{service="classifier"} | json | msg=`ML sidecar classification failed` | line_format `{{.sidecar}} | {{.source}} | {{.title_excerpt}} | {{.error_type}} | {{.error_detail}}`
```

**Time Series - Confidence Drift:**
```
avg_over_time({service="classifier"} | json | msg=`ML sidecar classification complete` | unwrap confidence [1h]) by (sidecar)
```

The dashboard should use a template variable `$sidecar` with values: `crime-ml`, `mining-ml`, `coforge-ml`, `entertainment-ml`, `anishinaabe-ml`.

Keep the existing dashboard UID (`north-cloud-ml-sidecars`) and navigation links.

**Step 3: Validate JSON**

Run: `python3 -c "import json; json.load(open('infrastructure/grafana/provisioning/dashboards/north-cloud-ml-sidecars.json'))"`
Expected: No errors

**Step 4: Commit**

```bash
git add infrastructure/grafana/provisioning/dashboards/north-cloud-ml-sidecars.json
git commit -m "feat(grafana): enhance ML Sidecars dashboard with performance, insights, and error panels"
```

---

### Task 11: Integration Test - Deploy and Verify

**Step 1: Build classifier**

Run: `task build:classifier` or `cd classifier && go build -o bin/classifier .`
Expected: builds without error

**Step 2: Run full linter**

Run: `task lint:classifier`
Expected: PASS

**Step 3: Run full test suite with coverage**

Run: `task test:cover:classifier`
Expected: all tests pass

**Step 4: Rebuild and restart classifier in dev**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build classifier`

**Step 5: Verify structured logs appear**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f classifier 2>&1 | grep "ML sidecar classification" | head -5`
Expected: JSON log lines with sidecar, content_id, relevance, decision_path, latency_ms, etc.

**Step 6: Restart Grafana to pick up dashboard changes**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml --profile observability restart grafana`

**Step 7: Verify dashboard loads**

Open Grafana at `https://northcloud.biz/grafana` (or `localhost:3000`), navigate to ML Sidecars dashboard. Verify panels populate with data.

**Step 8: Final commit (if any fixups needed)**

```bash
git commit -m "fix(classifier): address integration test findings"
```
