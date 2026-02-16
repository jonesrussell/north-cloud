# ML Sidecar Observability Design

**Date**: 2026-02-16
**Status**: Approved
**Approach**: Classifier-side structured logging + enhanced Grafana dashboard

## Problem

The existing ML Sidecars Grafana dashboard only shows log counts, error counts, and raw log streams. There's no visibility into:
- Sidecar response latency or throughput
- What content is being classified and how
- Why the classifier made specific decisions (rule vs ML, confidence levels)
- Model health tracking over time
- Error breakdown by type

## Approach

**Classifier-side structured logging**: Add rich structured log fields to the classifier's existing ML sidecar call sites in `runOptionalClassifiers()`. Every successful classification emits an Info-level JSON log line with content context, sidecar response details, and decision path. Failures emit enriched Warn-level logs.

**Single data source**: All metrics derived from Loki queries against the classifier's structured logs. No Prometheus, no new infrastructure.

**Single dashboard**: Expand the existing ML Sidecars dashboard with collapsible rows covering health, performance, classification insights, content flow, and errors.

## Log Schema

### Successful classification

```json
{
  "msg": "ML sidecar classification complete",
  "service": "classifier",
  "sidecar": "crime-ml",
  "content_id": "abc123",
  "content_type": "news_article",
  "source": "toronto.com",
  "title_excerpt": "Resident faces drug weapon assault...",
  "relevance": "core_street_crime",
  "confidence": 0.92,
  "ml_confidence_raw": 0.87,
  "rule_triggered": "assault_charge_pattern",
  "decision_path": "rule_override",
  "latency_ms": 45,
  "processing_time_ms": 31,
  "model_version": "2025-02-01-crime-v1",
  "sidecar_response_size_bytes": 512,
  "outcome": "success"
}
```

### Failed classification

```json
{
  "msg": "ML sidecar classification failed",
  "service": "classifier",
  "sidecar": "crime-ml",
  "content_id": "abc123",
  "content_type": "news_article",
  "source": "toronto.com",
  "title_excerpt": "Resident faces drug weapon assault...",
  "outcome": "error",
  "error_type": "timeout",
  "error_detail": "http request: context deadline exceeded",
  "latency_ms": 5000
}
```

### Field inventory per sidecar

| Sidecar | Sidecar-specific fields |
|---------|------------------------|
| crime-ml | relevance (street_crime_relevance), crime_types, location_specificity |
| mining-ml | relevance, mining_stage, commodities |
| coforge-ml | relevance, audience, topics, industries |
| entertainment-ml | relevance, categories |
| anishinaabe-ml | relevance, categories |

Common fields logged for all: sidecar, content_id, content_type, source, title_excerpt, confidence, ml_confidence_raw, rule_triggered, decision_path, latency_ms, processing_time_ms, model_version, sidecar_response_size_bytes, outcome.

## Code Changes

### A. `mltransport/transport.go` - Add latency and response size tracking

Change `DoClassify()` signature to return latency and response size:

```go
func DoClassify(ctx context.Context, baseURL string, req *ClassifyRequest, respPtr any) (latencyMs int64, responseSizeBytes int, err error)
```

- Wrap HTTP call with `time.Now()` / `time.Since(start).Milliseconds()`
- Read response body into `[]byte` buffer, capture `len(body)`, then `json.Unmarshal` into `respPtr`
- All callers updated to accept new return values

### B. Domain types - Add decision context fields

Add to `CrimeResult`, `MiningResult`, `CoforgeResult`, `EntertainmentResult`, `AnishinaabeResult`:

```go
DecisionPath     string  `json:"decision_path,omitempty"`
MLConfidenceRaw  float64 `json:"ml_confidence_raw,omitempty"`
RuleTriggered    string  `json:"rule_triggered,omitempty"`
ProcessingTimeMs int64   `json:"processing_time_ms,omitempty"`
```

### C. ML client packages - Populate decision context during hybrid logic

Each client's `Classify()` function follows: rules -> ML -> merge -> decision matrix -> return.

Record at each step (no logic changes):
- `RuleTriggered` - after step 1 (which rule matched, or "no_rule")
- `MLConfidenceRaw` - after step 2 (raw ML score before adjustment)
- `DecisionPath` - after step 4 (which branch of the decision matrix was taken)
- `ProcessingTimeMs` - from the ML sidecar HTTP response (already returned, currently discarded)

Affected packages:
- `classifier/internal/mlclient/` (crime)
- `classifier/internal/miningmlclient/` (mining)
- `classifier/internal/coforgemlclient/` (coforge)
- `classifier/internal/entertainmentmlclient/` (entertainment)
- `classifier/internal/anishinaabemlclient/` (anishinaabe)

### D. `classifier.go:runOptionalClassifiers()` - Emit structured log lines

Add `contentType string` parameter (already computed in `Classify()` before this function is called).

After each sidecar call, emit a structured log line using `infralogger`:

```go
if crimeResult != nil {
    c.logger.Info("ML sidecar classification complete",
        infralogger.String("sidecar", "crime-ml"),
        infralogger.String("content_id", raw.ID),
        infralogger.String("content_type", contentType),
        infralogger.String("source", raw.SourceName),
        infralogger.String("title_excerpt", truncateWords(raw.Title, 10)),
        infralogger.String("relevance", crimeResult.Relevance),
        infralogger.Float64("confidence", crimeResult.FinalConfidence),
        infralogger.Float64("ml_confidence_raw", crimeResult.MLConfidenceRaw),
        infralogger.String("rule_triggered", crimeResult.RuleTriggered),
        infralogger.String("decision_path", crimeResult.DecisionPath),
        infralogger.Int64("latency_ms", crimeLatencyMs),
        infralogger.Int64("processing_time_ms", crimeResult.ProcessingTimeMs),
        infralogger.String("model_version", crimeResult.ModelVersion),
        infralogger.Int("sidecar_response_size_bytes", crimeRespSize),
        infralogger.String("outcome", "success"),
    )
}
```

On failure:

```go
c.logger.Warn("ML sidecar classification failed",
    infralogger.String("sidecar", "crime-ml"),
    infralogger.String("content_id", raw.ID),
    infralogger.String("content_type", contentType),
    infralogger.String("source", raw.SourceName),
    infralogger.String("title_excerpt", truncateWords(raw.Title, 10)),
    infralogger.String("outcome", "error"),
    infralogger.String("error_type", classifyErrorType(scErr)),
    infralogger.String("error_detail", scErr.Error()),
    infralogger.Int64("latency_ms", crimeLatencyMs),
)
```

Helper functions:
- `truncateWords(s string, n int) string` - returns first N words of a string
- `classifyErrorType(err error) string` - categorizes error as "timeout", "5xx", "decode", "connection", "unknown"

### E. Cognitive complexity management

`runOptionalClassifiers()` already has a `//nolint:gocognit` annotation. Adding log lines will increase line count. Extract per-sidecar logging into a helper:

```go
func (c *Classifier) logSidecarResult(sidecar, contentID, contentType, source, title string, result sidecarLogFields, latencyMs int64, respSize int)
func (c *Classifier) logSidecarError(sidecar, contentID, contentType, source, title string, err error, latencyMs int64)
```

Where `sidecarLogFields` is an interface or struct extracting common fields (relevance, confidence, ml_confidence_raw, rule_triggered, decision_path, processing_time_ms, model_version).

## Grafana Dashboard Layout

Single dashboard: "ML Sidecars" (uid: `north-cloud-ml-sidecars`). 6 collapsible rows.

Base Loki filter: `{service="classifier"} | json | msg="ML sidecar classification complete"`

### Row 1: Health Overview

| Panel | Type | LogQL |
|-------|------|-------|
| Sidecar Status (x5) | Stat (green/red) | `count_over_time({service="classifier"} \| json \| msg=~"ML sidecar.*" \| sidecar="X" [5m]) > 0` |
| Model Versions | Table | Latest `model_version` per sidecar extracted from last log line |
| Error Rate % (x5) | Gauge | `errors / total * 100` per sidecar over 1h |

### Row 2: Performance

| Panel | Type | LogQL |
|-------|------|-------|
| P95 Latency by Sidecar | Time series | `quantile_over_time(0.95, {service="classifier"} \| json \| msg="ML sidecar classification complete" \| unwrap latency_ms [$__interval]) by (sidecar)` |
| Throughput (classifications/min) | Time series | `sum(count_over_time(... [$__interval])) by (sidecar)` |
| Sidecar Processing Time | Time series | `avg_over_time(... \| unwrap processing_time_ms [$__interval]) by (sidecar)` |
| Avg Response Size (x5) | Stat | `avg_over_time(... \| unwrap sidecar_response_size_bytes [1h])` per sidecar |
| *Optional: Latency vs Response Size* | Scatter | Correlation panel (future enhancement) |

### Row 3: Classification Insights

| Panel | Type | LogQL |
|-------|------|-------|
| Relevance Distribution (x5) | Pie chart | `sum by (relevance) (count_over_time(... \| sidecar="X" [$__range]))` |
| Decision Path Distribution | Bar chart | `sum by (decision_path, sidecar) (count_over_time(...))` |
| Confidence Distribution | Heatmap | `... \| unwrap confidence` bucketed by sidecar |
| Rule vs ML Decisions | Stacked bar | `decision_path` grouped: rule_override + rules_only = "Rule-based", ml_high_confidence = "ML-driven", hybrid = "Hybrid" |

### Row 4: Content Flow

| Panel | Type | LogQL |
|-------|------|-------|
| Content Types Classified | Bar chart | `sum by (content_type) (count_over_time(...))` |
| Top Sources by Volume | Table | `topk(10, sum by (source) (count_over_time(...)))` |
| Classification Volume by Source | Time series | `sum by (source) (count_over_time(...))` top 5 sources |

### Row 5: Errors & Anomalies

| Panel | Type | LogQL |
|-------|------|-------|
| Error Rate by Sidecar | Time series | `sum(count_over_time(... \| outcome="error" [$__interval])) by (sidecar)` |
| Errors by Type | Stacked bar | `sum by (error_type, sidecar) (count_over_time(... \| outcome="error"))` |
| Recent Errors | Table | Last 20 error lines: content_id, source, title_excerpt, sidecar, error_type, error_detail |
| Confidence Drift | Time series | Rolling `avg_over_time(... \| unwrap confidence [1h])` per sidecar - spot degradation |
| *Optional: Rule Trigger Frequency* | Bar chart | `sum by (rule_triggered) (count_over_time(...))` - detect rule regressions |

### Row 6: Log Streams (existing, collapsible)

Keep existing raw log panels per sidecar for deep debugging.

## Files Modified

| File | Change |
|------|--------|
| `classifier/internal/mltransport/transport.go` | Return latency + response size from DoClassify |
| `classifier/internal/domain/classification.go` | Add DecisionPath, MLConfidenceRaw, RuleTriggered, ProcessingTimeMs to all result types |
| `classifier/internal/mlclient/*.go` | Populate decision context fields in crime hybrid logic |
| `classifier/internal/miningmlclient/*.go` | Populate decision context fields in mining hybrid logic |
| `classifier/internal/coforgemlclient/*.go` | Populate decision context fields in coforge hybrid logic |
| `classifier/internal/entertainmentmlclient/*.go` | Populate decision context fields in entertainment logic |
| `classifier/internal/anishinaabemlclient/*.go` | Populate decision context fields in anishinaabe logic |
| `classifier/internal/classifier/classifier.go` | Emit structured logs in runOptionalClassifiers, add helper functions |
| `infrastructure/grafana/provisioning/dashboards/north-cloud-ml-sidecars.json` | Replace with enhanced dashboard |

## Testing

- Unit tests for `truncateWords()` and `classifyErrorType()` helpers
- Update existing ML client tests to verify decision context fields are populated
- Verify `DoClassify()` callers handle new return values
- Manual: deploy to dev, run classifier against test content, verify log lines in Loki, confirm dashboard panels populate

## Out of Scope

- ML sidecar Python code changes (no structured logging in sidecars themselves)
- Prometheus metrics
- Language detection
- Alerting rules (can be added as follow-up using Grafana alerting on Loki queries)
