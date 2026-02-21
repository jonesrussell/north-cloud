# Fix PR #91 Review Issues Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all critical, important, and suggestion-level issues identified in the PR #91 review for the classifier declarative routing feature.

**Architecture:** All changes are confined to the classifier service. No API or schema changes. Changes fall into three categories: observability/logging correctness, startup validation hardening, and test coverage for the new routing dispatch.

**Tech Stack:** Go 1.26+, golangci-lint, `task` taskfile runner

---

## Task 1: Harden startup validation — unknown sidecar names

**Files:**
- Modify: `classifier/internal/classifier/classifier.go:72-80`
- Modify: `classifier/internal/classifier/classifier_routing_test.go`

**Step 1: Add unknown-name check to startup loop**

In `NewClassifier`, change the existing loop at lines 72-80 from checking only `known && !enabled` to also check `!known`:

```go
for routeKey, names := range routingTable {
    for _, name := range names {
        switch enabled, known := sidecarEnabled[name]; {
        case !known:
            logger.Warn("Routing table references unknown sidecar name; it will always be ignored",
                infralogger.String("routing_key", routeKey),
                infralogger.String("sidecar_name", name),
            )
        case known && !enabled:
            logger.Warn("Routing table references disabled sidecar classifier",
                infralogger.String("routing_key", routeKey),
                infralogger.String("sidecar_name", name),
            )
        }
    }
}
```

**Step 2: Add recording logger to test file**

Add `recordingLogger` type in `classifier_routing_test.go` that captures Warn calls:

```go
type recordingLogger struct {
    mockLogger
    warns []string
}

func (r *recordingLogger) Warn(msg string, _ ...infralogger.Field) {
    r.warns = append(r.warns, msg)
}

func (r *recordingLogger) With(_ ...infralogger.Field) infralogger.Logger { return r }
```

**Step 3: Add test for unknown sidecar warning**

```go
func TestNewClassifier_UnknownSidecarInRoutingTable_Warns(t *testing.T) {
    rec := &recordingLogger{}
    cfg := Config{
        RoutingTable: map[string][]string{
            "article": {"crime", "typo_sidecar"},
        },
    }
    NewClassifier(rec, []domain.ClassificationRule{}, testhelpers.NewMockSourceReputationDB(), cfg)
    if len(rec.warns) == 0 {
        t.Fatal("expected a Warn for unknown sidecar name, got none")
    }
    found := false
    for _, w := range rec.warns {
        if strings.Contains(w, "unknown sidecar name") {
            found = true
            break
        }
    }
    if !found {
        t.Errorf("expected warn containing 'unknown sidecar name', got: %v", rec.warns)
    }
}
```

**Step 4: Add test for disabled-sidecar warning (and verify no warn for enabled)**

```go
func TestNewClassifier_DisabledSidecarInRoutingTable_Warns(t *testing.T) {
    rec := &recordingLogger{}
    cfg := Config{
        CrimeClassifier: nil, // disabled
        RoutingTable: map[string][]string{
            "article": {"crime"},
        },
    }
    NewClassifier(rec, []domain.ClassificationRule{}, testhelpers.NewMockSourceReputationDB(), cfg)
    found := false
    for _, w := range rec.warns {
        if strings.Contains(w, "disabled sidecar") {
            found = true
            break
        }
    }
    if !found {
        t.Errorf("expected warn containing 'disabled sidecar', got: %v", rec.warns)
    }
}
```

**Step 5: Run tests**
```bash
cd classifier && go test ./internal/classifier/... -run TestNewClassifier -v
```

---

## Task 2: Fix logSidecarNilResult severity

**Files:**
- Modify: `classifier/internal/classifier/observability.go:86-92`

**Step 1: Change Warn to Error and add outcome field**

```go
func (c *Classifier) logSidecarNilResult(sidecar, contentID string, latencyMs int64) {
    c.logger.Error("ML sidecar returned nil result without error",
        infralogger.String("sidecar", sidecar),
        infralogger.String("content_id", contentID),
        infralogger.Int64("latency_ms", latencyMs),
        infralogger.String("outcome", "nil_result"),
    )
}
```

**Step 2: Run tests**
```bash
cd classifier && go test ./internal/classifier/... -v
```

---

## Task 3: Fix runLocationOptional placeholder log

**Files:**
- Modify: `classifier/internal/classifier/classifier.go:433-435`

**Step 1: Replace logSidecarSuccess call with location-specific log**

Replace:
```go
c.logSidecarSuccess("location", raw, "",
    "", 0, 0, "", "", latencyMs, 0, "")
```

With:
```go
c.logger.Info("Location classification complete",
    infralogger.String("sidecar", "location"),
    infralogger.String("content_id", raw.ID),
    infralogger.String("source", raw.SourceName),
    infralogger.String("title_excerpt", truncateWords(raw.Title, titleExcerptWordLimit)),
    infralogger.String("specificity", locResult.Specificity),
    infralogger.String("city", locResult.City),
    infralogger.String("province", locResult.Province),
    infralogger.String("country", locResult.Country),
    infralogger.Float64("confidence", locResult.Confidence),
    infralogger.Int64("latency_ms", latencyMs),
    infralogger.String("outcome", "success"),
)
```

**Step 2: Run tests**
```bash
cd classifier && go test ./internal/classifier/... -v
```

---

## Task 4: Raise ResolveSidecars subtype fallback from Debug to Warn

**Files:**
- Modify: `classifier/internal/classifier/classifier.go:109-112`

**Step 1: Change Debug to Warn with updated message**

```go
c.logger.Warn("No routing entry for article subtype; falling back to article route — all sidecars will run",
    infralogger.String("content_subtype", subtype),
    infralogger.String("fallback_key", "article"),
)
```

---

## Task 5: Export SetDefaults from config package and fix processor fallback

**Files:**
- Modify: `classifier/internal/config/config.go`
- Modify: `classifier/cmd/processor/processor.go:60-78`

**Step 1: Export SetDefaults in config.go**

Add after `setDefaults`:
```go
// SetDefaults applies all defaults to cfg. Call this when constructing a Config without Load.
func SetDefaults(cfg *Config) {
    setDefaults(cfg)
}
```

**Step 2: Add defaultProcessorConcurrency constant to processor.go**

```go
defaultProcessorConcurrency = 5
```

**Step 3: Fix LoadConfig fallback to call SetDefaults**

Replace:
```go
cfg = &config.Config{}
if cfg.Service.PollInterval == 0 {
    cfg.Service.PollInterval = defaultPollInterval
}
if cfg.Service.BatchSize == 0 {
    cfg.Service.BatchSize = 100
}
if cfg.Service.Concurrency == 0 {
    cfg.Service.Concurrency = 5
}
```

With:
```go
cfg = &config.Config{}
config.SetDefaults(cfg)
cfg.Service.Concurrency = defaultProcessorConcurrency // processor uses lower concurrency than HTTP service
```

**Step 4: Run tests**
```bash
cd classifier && go test ./... -v
```

---

## Task 6: Add rules-only mode warning to all processor classifier factories

**Files:**
- Modify: `classifier/cmd/processor/processor.go:217-299`

**Step 1: Update all 5 factories**

Apply the bootstrap pattern (warn when enabled but URL empty) to:
- `createCrimeClassifier`
- `createMiningClassifier`
- `createCoforgeClassifier`
- `createEntertainmentClassifier`
- `createAnishinaabeClassifier`

Pattern for each:
```go
if cfg.Classification.Crime.MLServiceURL != "" {
    mlClient = mlclient.NewClient(cfg.Classification.Crime.MLServiceURL)
    log.Info("Crime classifier enabled for processor",
        infralogger.String("ml_service_url", cfg.Classification.Crime.MLServiceURL))
} else {
    log.Warn("Crime classifier enabled for processor but ML service URL is empty; running in rules-only mode",
        infralogger.String("ml_service_url", ""))
}
```

---

## Task 7: SidecarRegistry YAML warning

**Files:**
- Modify: `classifier/internal/config/config.go`
- Modify: `classifier/internal/bootstrap/classifier.go`
- Modify: `classifier/cmd/processor/processor.go`

**Step 1: Add SidecarRegistryFromYAML flag to ClassificationConfig**

```go
// SidecarRegistryFromYAML is true when sidecar_registry was set explicitly in the YAML config.
// It has no effect on runtime behavior (SidecarRegistry is not yet consumed) but triggers a
// startup warning so operators know the field is inoperative.
SidecarRegistryFromYAML bool `yaml:"-"` // not loaded from YAML; set by setClassificationDefaults
```

**Step 2: Set the flag in setClassificationDefaults**

```go
if c.SidecarRegistry != nil {
    c.SidecarRegistryFromYAML = true
} else {
    c.SidecarRegistry = getDefaultSidecarRegistry(c)
}
```

**Step 3: Warn in bootstrap NewHTTPComponents**

After the call to `SetupDatabase` (first thing that needs logger), add:
```go
if cfg.Classification.SidecarRegistryFromYAML {
    logger.Warn("classification.sidecar_registry is set in config but is not yet consumed; " +
        "use the named fields (crime.enabled, mining.enabled, etc.) to control sidecar behaviour")
}
```

**Step 4: Warn in processor createClassifierConfig**

At the top of `createClassifierConfig`:
```go
if cfg.Classification.SidecarRegistryFromYAML {
    log.Warn("classification.sidecar_registry is set in config but is not yet consumed; " +
        "use the named fields (crime.enabled, mining.enabled, etc.) to control sidecar behaviour")
}
```

---

## Task 8: Add tests for runOptionalClassifiers dispatch

**Files:**
- Modify: `classifier/internal/classifier/classifier_routing_test.go`

**Step 1: Add mock classifiers for testing dispatch**

Create minimal mock crime/location classifiers that record calls:
```go
type mockCrimeClassifier struct {
    called bool
}
func (m *mockCrimeClassifier) Classify(_ context.Context, _ *domain.RawContent) (*CrimeResult, error) {
    m.called = true
    return &CrimeResult{Relevance: "not_crime"}, nil
}
```

**Step 2: Test nil-guard (disabled sidecar in routing table does not panic)**

```go
func TestRunOptionalClassifiers_NilSidecarDoesNotPanic(t *testing.T) {
    cfg := Config{
        CrimeClassifier: nil,
        RoutingTable: map[string][]string{"article": {"crime"}},
    }
    clf := NewClassifier(&mockLogger{}, nil, testhelpers.NewMockSourceReputationDB(), cfg)
    raw := &domain.RawContent{ID: "test-1", Title: "Test"}
    crime, _, _, _, _, _ := clf.classifyOptionalForPublishable(context.Background(), raw, "article", "")
    if crime != nil {
        t.Error("expected nil crime result when classifier is nil")
    }
}
```

**Step 3: Test unknown sidecar name does not panic**

```go
func TestRunOptionalClassifiers_UnknownSidecarDoesNotPanic(t *testing.T) {
    cfg := Config{
        RoutingTable: map[string][]string{"article": {"unknown_future_sidecar"}},
    }
    clf := NewClassifier(&mockLogger{}, nil, testhelpers.NewMockSourceReputationDB(), cfg)
    raw := &domain.RawContent{ID: "test-1", Title: "Test"}
    // Should not panic
    clf.classifyOptionalForPublishable(context.Background(), raw, "article", "")
}
```

**Step 4: Test enabled sidecar in routing table IS called**

```go
func TestRunOptionalClassifiers_EnabledSidecarIsCalled(t *testing.T) {
    mock := &mockCrimeClassifier{}
    crimeCC := &CrimeClassifier{ /* use constructor or test helper */ }
    // ... wire mock into CrimeClassifier and verify mock.called == true
}
```

Note: If `CrimeClassifier` doesn't expose the ML client for testing, test via `Classify()` output instead.

**Step 5: Run tests**
```bash
cd classifier && go test ./internal/classifier/... -v
```

---

## Task 9: Fix ClassifyBatch to log failure summary

**Files:**
- Modify: `classifier/internal/classifier/classifier.go:219-238`

**Step 1: Add failure counter and summary log**

```go
func (c *Classifier) ClassifyBatch(ctx context.Context, rawItems []*domain.RawContent) ([]*domain.ClassificationResult, error) {
    results := make([]*domain.ClassificationResult, len(rawItems))
    failedCount := 0

    for i, raw := range rawItems {
        result, err := c.Classify(ctx, raw)
        if err != nil {
            c.logger.Error("Batch classification failed for item",
                infralogger.Int("index", i),
                infralogger.String("content_id", raw.ID),
                infralogger.Error(err),
            )
            failedCount++
            continue
        }
        results[i] = result
    }

    if failedCount > 0 {
        c.logger.Error("Batch classification completed with failures",
            infralogger.Int("total", len(rawItems)),
            infralogger.Int("failed", failedCount),
            infralogger.Int("succeeded", len(rawItems)-failedCount),
        )
    }

    return results, nil
}
```

---

## Task 10: Lint and CI

**Step 1: Run linter**
```bash
task lint:classifier
```

**Step 2: Run full CI**
```bash
task ci
```

Expected: all green.
