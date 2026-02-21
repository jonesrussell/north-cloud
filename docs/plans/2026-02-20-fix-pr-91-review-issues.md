# Fix PR #91 Review Issues Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all critical and important issues found in the PR #91 code review, plus implement all "nice to have" improvements.

**Architecture:** All changes are in the classifier service. The fixes fall into four categories: test correctness (`classifier_routing_test.go`), silent-failure logging (`classifier.go`), processor wiring parity (`cmd/processor/processor.go`), and dead-code documentation (`config/config.go`). No new packages or interfaces are needed.

**Tech Stack:** Go 1.26, golangci-lint, `task test:classifier`, `task lint:classifier`

---

## Task 1: Fix `t.Helper()` misuse in top-level test functions

**Files:**
- Modify: `classifier/internal/classifier/classifier_routing_test.go:14,52,80`

**Step 1: Remove `t.Helper()` from the two top-level test functions**

The lines `t.Helper()` at line 14 (inside `TestResolveSidecars`) and line 80 (inside `TestResolveSidecars_MissingKey_ReturnsNilAndLogs`) are wrong — `t.Helper()` is for helper functions only, not `Test*` entry points. Also remove it from the subtest closure at line 52 — subtests registered via `t.Run` are also not helpers.

In `classifier/internal/classifier/classifier_routing_test.go`, make these changes:

```go
// Line 13-14: remove t.Helper()
func TestResolveSidecars(t *testing.T) {
    // (no t.Helper() here)
    routingTable := ...
```

```go
// Line 51-52: remove t.Helper() from t.Run closure
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // (no t.Helper() here)
            got := clf.ResolveSidecars(tt.contentType, tt.subtype)
```

```go
// Line 79-80: remove t.Helper()
func TestResolveSidecars_MissingKey_ReturnsNilAndLogs(t *testing.T) {
    // (no t.Helper() here)
    cfg := Config{
```

**Step 2: Run tests to confirm they still pass**

```bash
cd /home/fsd42/dev/north-cloud && task test:classifier
```

Expected: all tests pass.

**Step 3: Commit**

```bash
git add classifier/internal/classifier/classifier_routing_test.go
git commit -m "test(classifier): remove t.Helper() from top-level test functions"
```

---

## Task 2: Fix `assertEqualStringSlices` to distinguish nil from empty slice

**Files:**
- Modify: `classifier/internal/classifier/classifier_routing_test.go:47-48,59-76`

**Background:** `append([]string(nil), []string{}...)` returns `nil` in Go. So routing entries with empty slice values (`"article:report": {}`) produce a `nil` return from `ResolveSidecars`, not `[]string{}`. The test cases for `"article report"` and `"page has explicit empty routing"` claimed to expect `[]string{}` but the helper couldn't distinguish nil from empty — both have length 0.

The semantics matter: `nil` means "no routing entry found, log and skip" whereas `[]string{}` means "explicit entry found, explicitly empty list". We fix both the helper and the test expectations.

**Step 1: Update `assertEqualStringSlices` to check nil/non-nil distinction**

Replace the body of `assertEqualStringSlices` with:

```go
func assertEqualStringSlices(t *testing.T, got, want []string) {
    t.Helper()
    if (got == nil) != (want == nil) {
        t.Errorf("ResolveSidecars() nil mismatch: got nil=%v, want nil=%v; got=%v, want=%v",
            got == nil, want == nil, got, want)
        return
    }
    if want == nil {
        return
    }
    if len(got) != len(want) {
        t.Errorf("ResolveSidecars() length = %d, want %d; got %v", len(got), len(want), got)
        return
    }
    for i := range got {
        if got[i] != want[i] {
            t.Errorf("ResolveSidecars()[%d] = %q, want %q", i, got[i], want[i])
        }
    }
}
```

**Step 2: Update test cases that expect `[]string{}` to expect `nil`**

In the `tests` table in `TestResolveSidecars`, change:
```go
// Before:
{"article report", domain.ContentTypeArticle, domain.ContentSubtypeReport, []string{}},
{"page has explicit empty routing", domain.ContentTypePage, "", []string{}},

// After:
{"article report", domain.ContentTypeArticle, domain.ContentSubtypeReport, nil},
{"page has explicit empty routing", domain.ContentTypePage, "", nil},
```

**Step 3: Run tests to confirm they pass**

```bash
cd /home/fsd42/dev/north-cloud && task test:classifier
```

Expected: all tests pass.

**Step 4: Commit**

```bash
git add classifier/internal/classifier/classifier_routing_test.go
git commit -m "test(classifier): fix nil-vs-empty slice distinction in assertEqualStringSlices"
```

---

## Task 3: Add missing test — article subtype fallback to "article" key

**Files:**
- Modify: `classifier/internal/classifier/classifier_routing_test.go`

**Background:** `ResolveSidecars("article", "press_release")` when `"article:press_release"` is not in the routing table should fall back to the `"article"` key. The current tests never exercise this branch (the `"article default"` test uses `subtype=""` which takes the `else` branch directly).

**Step 1: Add the fallback test case to the table in `TestResolveSidecars`**

Add this case to the `tests` slice (after the existing "article blotter" case):

```go
{
    "article unknown subtype falls back to article",
    domain.ContentTypeArticle,
    "press_release", // not in routing table as "article:press_release"
    []string{"crime", "mining", "location"}, // same as "article"
},
```

The routing table used in `TestResolveSidecars` has `"article": {"crime", "mining", "location"}` but no `"article:press_release"` entry, so this correctly exercises the fallback path.

**Step 2: Run tests**

```bash
cd /home/fsd42/dev/north-cloud && task test:classifier
```

Expected: all tests pass, including the new case.

**Step 3: Commit**

```bash
git add classifier/internal/classifier/classifier_routing_test.go
git commit -m "test(classifier): add article subtype fallback test case for ResolveSidecars"
```

---

## Task 4: Add subtype-fallback debug log in `ResolveSidecars`

**Files:**
- Modify: `classifier/internal/classifier/classifier.go:83-103`

**Background:** When an article has a non-empty subtype that is not in the routing table, the code silently falls back to the `"article"` key with no log. An operator adding a new content subtype without a routing entry has no observability into which route was used.

**Step 1: Add a Debug log for the subtype miss before the fallback**

Current code (lines 83-103):
```go
func (c *Classifier) ResolveSidecars(contentType, subtype string) []string {
    var key string
    if contentType == domain.ContentTypeArticle && subtype != "" {
        key = "article:" + subtype
        if names, ok := c.routingTable[key]; ok {
            return names
        }
        key = "article"
    } else {
        key = contentType
    }
    ...
}
```

Replace with:
```go
func (c *Classifier) ResolveSidecars(contentType, subtype string) []string {
    var key string
    if contentType == domain.ContentTypeArticle && subtype != "" {
        key = "article:" + subtype
        if names, ok := c.routingTable[key]; ok {
            return names
        }
        c.logger.Debug("No routing entry for article subtype; falling back to article key",
            infralogger.String("content_subtype", subtype),
            infralogger.String("fallback_key", "article"),
        )
        key = "article"
    } else {
        key = contentType
    }
    if names, ok := c.routingTable[key]; ok {
        return names
    }
    c.logger.Debug("No routing entry for content type; skipping optional classifiers",
        infralogger.String("content_type", contentType),
        infralogger.String("content_subtype", subtype),
        infralogger.String("routing_key", key),
    )
    return nil
}
```

**Step 2: Run tests and lint**

```bash
cd /home/fsd42/dev/north-cloud && task test:classifier && task lint:classifier
```

Expected: all pass.

**Step 3: Commit**

```bash
git add classifier/internal/classifier/classifier.go
git commit -m "feat(classifier): log subtype fallback in ResolveSidecars for observability"
```

---

## Task 5: Warn on unknown sidecar names in `runOptionalClassifiers`

**Files:**
- Modify: `classifier/internal/classifier/classifier.go:243-260`

**Background:** If the routing table contains `"crme"` (typo for `"crime"`), `allowed["crime"]` is false and the crime sidecar silently doesn't run. There is no log output.

**Step 1: Add a named constant set and a warn loop**

At the top of the file (near the existing constants block at line 12), add:

```go
// knownSidecarNames is the set of sidecar names recognised by runOptionalClassifiers.
// Any name in the routing table not in this set is silently ignored but will emit a Warn log.
var knownSidecarNames = map[string]bool{
    "crime": true, "mining": true, "coforge": true,
    "entertainment": true, "anishinaabe": true, "location": true,
}
```

Then in `runOptionalClassifiers`, after building the `allowed` map, add the warning loop:

```go
func (c *Classifier) runOptionalClassifiers(
    ctx context.Context, raw *domain.RawContent, contentType string, sidecars []string,
) (*domain.CrimeResult, *domain.MiningResult, *domain.CoforgeResult, *domain.EntertainmentResult, *domain.AnishinaabeResult, *domain.LocationResult) {
    allowed := make(map[string]bool)
    for _, name := range sidecars {
        allowed[name] = true
        if !knownSidecarNames[name] {
            c.logger.Warn("Routing table contains unknown sidecar name; it will be ignored",
                infralogger.String("sidecar_name", name),
                infralogger.String("content_type", contentType),
                infralogger.String("content_id", raw.ID),
            )
        }
    }
    return c.runCrimeOptional(ctx, raw, contentType, allowed["crime"]),
        c.runMiningOptional(ctx, raw, contentType, allowed["mining"]),
        c.runCoforgeOptional(ctx, raw, contentType, allowed["coforge"]),
        c.runEntertainmentOptional(ctx, raw, contentType, allowed["entertainment"]),
        c.runAnishinaabeOptional(ctx, raw, contentType, allowed["anishinaabe"]),
        c.runLocationOptional(ctx, raw, allowed["location"])
}
```

Note: the warning is emitted at classification time, not startup time, because the routing table is populated at startup and the classifier doesn't have logger context until it processes an item. This is a reasonable trade-off.

**Step 2: Run tests and lint**

```bash
cd /home/fsd42/dev/north-cloud && task test:classifier && task lint:classifier
```

Expected: all pass. The linter may flag the `var` block as a package-level variable — if so, convert to a package-level function or use a local map literal inside the function (latter avoids linter issues with global state).

If the linter objects to the `var knownSidecarNames` global, move it inside `runOptionalClassifiers` as a local map:

```go
knownSidecarNames := map[string]bool{
    "crime": true, "mining": true, "coforge": true,
    "entertainment": true, "anishinaabe": true, "location": true,
}
```

**Step 3: Commit**

```bash
git add classifier/internal/classifier/classifier.go
git commit -m "feat(classifier): warn on unknown sidecar names in routing table"
```

---

## Task 6: Add nil-result warn log for mining/coforge/entertainment/anishinaabe helpers

**Files:**
- Modify: `classifier/internal/classifier/classifier.go:286-372`

**Background:** Each of the four ML sidecar helpers (`runMiningOptional`, `runCoforgeOptional`, `runEntertainmentOptional`, `runAnishinaabeOptional`) has the pattern:

```go
if result != nil {
    logSidecarSuccess(...)
}
return result // returns nil with zero log when result is nil and no error
```

This silently drops the result when a classifier returns `(nil, nil)`. While currently unreachable, it should emit a Warn to make future regressions visible.

**Step 1: Fix all four helpers — change `if result != nil { log }; return result` to explicit nil guard with Warn**

For `runMiningOptional` (lines 299-305), replace:
```go
    if minResult != nil {
        c.logSidecarSuccess("mining-ml", raw, contentType,
            minResult.Relevance, minResult.FinalConfidence,
            minResult.MLConfidenceRaw, minResult.RuleTriggered,
            minResult.DecisionPath, latencyMs, minResult.ProcessingTimeMs, minResult.ModelVersion)
    }
    return minResult
```
with:
```go
    if minResult == nil {
        c.logger.Warn("Mining sidecar returned nil result without error",
            infralogger.String("sidecar", "mining-ml"),
            infralogger.String("content_id", raw.ID),
            infralogger.Int64("latency_ms", latencyMs),
        )
        return nil
    }
    c.logSidecarSuccess("mining-ml", raw, contentType,
        minResult.Relevance, minResult.FinalConfidence,
        minResult.MLConfidenceRaw, minResult.RuleTriggered,
        minResult.DecisionPath, latencyMs, minResult.ProcessingTimeMs, minResult.ModelVersion)
    return minResult
```

Apply the same pattern for `runCoforgeOptional` (`cfResult`/`"coforge-ml"`), `runEntertainmentOptional` (`entResult`/`"entertainment-ml"`), and `runAnishinaabeOptional` (`aResult`/`"anishinaabe-ml"`).

Also fix `runCrimeOptional` (lines 275-276) which silently returns nil when `scResult == nil`:
```go
    // Before:
    if scResult == nil {
        return nil
    }
    // After:
    if scResult == nil {
        c.logger.Warn("Crime sidecar returned nil result without error",
            infralogger.String("sidecar", "crime-ml"),
            infralogger.String("content_id", raw.ID),
            infralogger.Int64("latency_ms", latencyMs),
        )
        return nil
    }
```

**Step 2: Run tests and lint**

```bash
cd /home/fsd42/dev/north-cloud && task test:classifier && task lint:classifier
```

Expected: all pass.

**Step 3: Commit**

```bash
git add classifier/internal/classifier/classifier.go
git commit -m "feat(classifier): warn when ML sidecar returns nil result without error"
```

---

## Task 7: Fix `runLocationOptional` — add latency, success log, consistent error log

**Files:**
- Modify: `classifier/internal/classifier/classifier.go:374-388`

**Background:** Location is the only sidecar helper with no latency measurement, no success log, and a hand-written error log missing 7 structured fields. This means location errors/successes are invisible in dashboards.

Location's `LocationResult` doesn't have `Relevance`, `FinalConfidence`, `MLConfidenceRaw`, `RuleTriggered`, `DecisionPath`, `ProcessingTimeMs`, `ModelVersion` fields that other sidecar results expose. We'll use empty/zero values for those `logSidecarSuccess` parameters.

**Step 1: Add latency, logSidecarError, logSidecarSuccess to `runLocationOptional`**

Replace the entire `runLocationOptional` function:

```go
func (c *Classifier) runLocationOptional(
    ctx context.Context, raw *domain.RawContent, run bool,
) *domain.LocationResult {
    if !run || c.location == nil {
        return nil
    }
    start := time.Now()
    locResult, locErr := c.location.Classify(ctx, raw)
    latencyMs := time.Since(start).Milliseconds()
    if locErr != nil {
        c.logSidecarError("location", raw, "", locErr, latencyMs)
        return nil
    }
    if locResult == nil {
        c.logger.Warn("Location sidecar returned nil result without error",
            infralogger.String("sidecar", "location"),
            infralogger.String("content_id", raw.ID),
            infralogger.Int64("latency_ms", latencyMs),
        )
        return nil
    }
    c.logSidecarSuccess("location", raw, "",
        "", 0, 0, "", "", latencyMs, 0, "")
    return locResult
}
```

Note: `contentType` is passed as `""` because `runLocationOptional` doesn't receive it (location is content-type-agnostic). This is consistent — the field will be empty in logs for location, which is accurate.

**Step 2: Run tests and lint**

```bash
cd /home/fsd42/dev/north-cloud && task test:classifier && task lint:classifier
```

Expected: all pass.

**Step 3: Commit**

```bash
git add classifier/internal/classifier/classifier.go
git commit -m "feat(classifier): add latency measurement and consistent logging to runLocationOptional"
```

---

## Task 8: Add TODO comment to dead `SidecarRegistry` field in config

**Files:**
- Modify: `classifier/internal/config/config.go:140-141`

**Background:** `SidecarRegistry` is populated by defaults but never read by bootstrap or classifier. Removing it now would break future work that intends to use it. Instead, add a clear comment so the next developer isn't confused.

**Step 1: Update the comment on `SidecarRegistry`**

Change:
```go
    // SidecarRegistry maps sidecar name (e.g. "crime", "mining") to enabled + URL. Optional; built from Crime/Mining/... if absent.
    SidecarRegistry map[string]SidecarConfig `yaml:"sidecar_registry"`
```

To:
```go
    // SidecarRegistry maps sidecar name (e.g. "crime", "mining") to enabled + URL.
    // Built from Crime/Mining/... named configs when absent in YAML.
    // NOTE: Currently populated by setClassificationDefaults but not yet consumed by the bootstrap
    // or classifier — the named fields (Crime, Mining, etc.) remain authoritative.
    // TODO: when declarative registry-driven dispatch is implemented, this will replace named fields.
    SidecarRegistry map[string]SidecarConfig `yaml:"sidecar_registry"`
```

**Step 2: Run lint**

```bash
cd /home/fsd42/dev/north-cloud && task lint:classifier
```

Expected: passes.

**Step 3: Commit**

```bash
git add classifier/internal/config/config.go
git commit -m "docs(classifier): clarify SidecarRegistry is populated but not yet consumed"
```

---

## Task 9: Fix processor `createClassifierConfig` — wire mining/coforge/entertainment/anishinaabe

**Files:**
- Modify: `classifier/cmd/processor/processor.go:1-30,182-230`

**Background:** The processor path only wires `CrimeClassifier`. The bootstrap path wires all five sidecars. This means with `MINING_ENABLED=true`, the processor path still passes `nil` for `MiningClassifier` — the routing table may list `"mining"`, `run=true`, but `c.mining == nil` silently skips it.

**Step 1: Add missing imports to processor.go**

The processor currently imports only `mlclient`. Add the other four ML clients:

```go
import (
    // ... existing imports ...
    "github.com/jonesrussell/north-cloud/classifier/internal/miningmlclient"
    "github.com/jonesrussell/north-cloud/classifier/internal/coforgemlclient"
    "github.com/jonesrussell/north-cloud/classifier/internal/entertainmentmlclient"
    "github.com/jonesrussell/north-cloud/classifier/internal/anishinaabemlclient"
)
```

**Step 2: Add missing constants**

The processor has `defaultMinQualityScore = 30` but bootstrap uses separate constants per-classifier. Since processor uses a single quality weight for all four, no new constants are needed — existing `defaultQualityWeight` applies.

**Step 3: Update `createClassifierConfig` to wire all five sidecars**

Replace the current `createClassifierConfig` in `processor.go`:

```go
func createClassifierConfig(cfg *config.Config, log infralogger.Logger) classifier.Config {
    return classifier.Config{
        Version:         "1.0.0",
        MinQualityScore: defaultMinQualityScore,
        UpdateSourceRep: true,
        QualityConfig: classifier.QualityConfig{
            WordCountWeight:   defaultQualityWeight,
            MetadataWeight:    defaultQualityWeight,
            RichnessWeight:    defaultQualityWeight,
            ReadabilityWeight: defaultQualityWeight,
            MinWordCount:      defaultMinWordCount,
            OptimalWordCount:  defaultOptimalWordCount800,
        },
        SourceReputationConfig: classifier.SourceReputationConfig{
            DefaultScore:               defaultReputationScore70,
            UpdateOnEachClassification: true,
            SpamThreshold:              defaultSpamThreshold,
            MinArticlesForTrust:        minArticlesForTrust,
            ReputationDecayRate:        defaultReputationDecayRate95,
        },
        CrimeClassifier:         createCrimeClassifier(cfg, log),
        MiningClassifier:        createMiningClassifier(cfg, log),
        CoforgeClassifier:       createCoforgeClassifier(cfg, log),
        EntertainmentClassifier: createEntertainmentClassifier(cfg, log),
        AnishinaabeClassifier:   createAnishinaabeClassifier(cfg, log),
        RoutingTable:            cfg.Classification.Routing,
    }
}
```

**Step 4: Add the four new classifier constructor functions**

After the existing `createCrimeClassifier` function, add:

```go
func createMiningClassifier(cfg *config.Config, log infralogger.Logger) *classifier.MiningClassifier {
    if !cfg.Classification.Mining.Enabled {
        return nil
    }
    var mlClient classifier.MLClassifier
    if cfg.Classification.Mining.MLServiceURL != "" {
        mlClient = miningmlclient.NewClient(cfg.Classification.Mining.MLServiceURL)
    }
    log.Info("Mining classifier enabled for processor",
        infralogger.String("ml_service_url", cfg.Classification.Mining.MLServiceURL))
    return classifier.NewMiningClassifier(mlClient, log, true)
}

func createCoforgeClassifier(cfg *config.Config, log infralogger.Logger) *classifier.CoforgeClassifier {
    if !cfg.Classification.Coforge.Enabled {
        return nil
    }
    var mlClient classifier.MLClassifier
    if cfg.Classification.Coforge.MLServiceURL != "" {
        mlClient = coforgemlclient.NewClient(cfg.Classification.Coforge.MLServiceURL)
    }
    log.Info("Coforge classifier enabled for processor",
        infralogger.String("ml_service_url", cfg.Classification.Coforge.MLServiceURL))
    return classifier.NewCoforgeClassifier(mlClient, log, true)
}

func createEntertainmentClassifier(cfg *config.Config, log infralogger.Logger) *classifier.EntertainmentClassifier {
    if !cfg.Classification.Entertainment.Enabled {
        return nil
    }
    var mlClient classifier.MLClassifier
    if cfg.Classification.Entertainment.MLServiceURL != "" {
        mlClient = entertainmentmlclient.NewClient(cfg.Classification.Entertainment.MLServiceURL)
    }
    log.Info("Entertainment classifier enabled for processor",
        infralogger.String("ml_service_url", cfg.Classification.Entertainment.MLServiceURL))
    return classifier.NewEntertainmentClassifier(mlClient, log, true)
}

func createAnishinaabeClassifier(cfg *config.Config, log infralogger.Logger) *classifier.AnishinaabeClassifier {
    if !cfg.Classification.Anishinaabe.Enabled {
        return nil
    }
    var mlClient classifier.MLClassifier
    if cfg.Classification.Anishinaabe.MLServiceURL != "" {
        mlClient = anishinaabemlclient.NewClient(cfg.Classification.Anishinaabe.MLServiceURL)
    }
    log.Info("Anishinaabe classifier enabled for processor",
        infralogger.String("ml_service_url", cfg.Classification.Anishinaabe.MLServiceURL))
    return classifier.NewAnishinaabeClassifier(mlClient, log, true)
}
```

**Step 5: Verify that `classifier.MLClassifier` is the correct interface for the mining/coforge/entertainment/anishinaabe clients**

Check `classifier/internal/classifier/mining.go` to see what interface `NewMiningClassifier` expects as its first parameter. If it's a different interface type (e.g., `*miningmlclient.Client` not `classifier.MLClassifier`), adjust the `var mlClient` type accordingly. The pattern matches `createCrimeClassifier` exactly, so it should be identical.

**Step 6: Run lint and tests**

```bash
cd /home/fsd42/dev/north-cloud && task lint:classifier && task test:classifier
```

Expected: all pass.

**Step 7: Commit**

```bash
git add classifier/cmd/processor/processor.go
git commit -m "fix(classifier): wire mining/coforge/entertainment/anishinaabe in processor createClassifierConfig"
```

---

## Task 10: Add startup-time routing validation warning in `NewClassifier` (nice-to-have)

**Files:**
- Modify: `classifier/internal/classifier/classifier.go:52-78`

**Background:** If the routing table references `"mining"` but `MiningClassifier` is nil (disabled), there's no warning at startup — the miss only appears silently at classification time. A startup warn tells operators immediately that their config is inconsistent.

**Step 1: Add sidecar nil-check after building routingTable in `NewClassifier`**

After the `routingTable` is built and the `Classifier` struct is returned, add a validation pass. However, since the `Classifier` struct isn't yet assigned when we're inside `NewClassifier`, we need to check `config` directly:

```go
func NewClassifier(
    logger infralogger.Logger,
    rules []domain.ClassificationRule,
    sourceRepDB SourceReputationDB,
    config Config,
) *Classifier {
    routingTable := make(map[string][]string)
    for k, v := range config.RoutingTable {
        routingTable[k] = append([]string(nil), v...)
    }
    // Warn if routing table references a disabled (nil) sidecar classifier.
    sidecarEnabled := map[string]bool{
        "crime":         config.CrimeClassifier != nil,
        "mining":        config.MiningClassifier != nil,
        "coforge":       config.CoforgeClassifier != nil,
        "entertainment": config.EntertainmentClassifier != nil,
        "anishinaabe":   config.AnishinaabeClassifier != nil,
        "location":      true, // always constructed below
    }
    for routeKey, names := range routingTable {
        for _, name := range names {
            if enabled, known := sidecarEnabled[name]; known && !enabled {
                logger.Warn("Routing table references disabled sidecar classifier",
                    infralogger.String("routing_key", routeKey),
                    infralogger.String("sidecar_name", name),
                )
            }
        }
    }
    return &Classifier{
        // ... existing fields ...
    }
}
```

**Step 2: Run tests and lint**

```bash
cd /home/fsd42/dev/north-cloud && task test:classifier && task lint:classifier
```

Expected: all pass.

**Step 3: Commit**

```bash
git add classifier/internal/classifier/classifier.go
git commit -m "feat(classifier): warn at startup when routing table references disabled sidecar"
```

---

## Task 11: Warn when classifier is enabled with empty ML URL (nice-to-have)

**Files:**
- Modify: `classifier/internal/bootstrap/classifier.go:199-215`

**Background:** `createOptionalClassifier` logs `"Crime classifier enabled"` at Info even when `mlURL=""`, meaning the ML component is absent and the sidecar will run rules-only. Operators can't distinguish fully-operational from rules-only mode from logs.

**Step 1: Change the log from unconditional Info to conditional Warn**

In `createOptionalClassifier` (lines 210-213):

```go
// Before:
logger.Info(label+" enabled", infralogger.String("ml_service_url", mlURL))
return newClassifier(client, logger, true)

// After:
if mlURL == "" {
    logger.Warn(label+" enabled but ML service URL is empty; running in rules-only mode",
        infralogger.String("ml_service_url", ""),
    )
} else {
    logger.Info(label+" enabled", infralogger.String("ml_service_url", mlURL))
}
return newClassifier(client, logger, true)
```

**Step 2: Run tests and lint**

```bash
cd /home/fsd42/dev/north-cloud && task test:classifier && task lint:classifier
```

Expected: all pass.

**Step 3: Commit**

```bash
git add classifier/internal/bootstrap/classifier.go
git commit -m "feat(classifier): warn when classifier is enabled with empty ML URL (rules-only mode)"
```

---

## Final Verification

After all tasks are committed:

```bash
cd /home/fsd42/dev/north-cloud && task lint:classifier && task test:classifier
```

Expected: all tests pass, zero lint violations.

Push and update the PR:

```bash
git push -u origin claude/classifier-declarative-routing
```
