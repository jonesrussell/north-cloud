# AI Observer Service — Design

**Date:** 2026-03-07
**Status:** Approved

---

## Problem

Two converging pressures require a systematic response:

1. **Classifier refinement requires visibility into real-world failures.** Misclassifications, borderline cases, HTML extraction failures, and sidecar inconsistencies are only visible through manual log inspection today.

2. **Sidecars are multiplying.** Each container consumes ~512MB RAM and has its own lifecycle, logs, and failure modes. This is sustainable at 3–4 sidecars; it becomes a problem at 10–12.

---

## Core Principle: AI as Observer, Not Dependency

The ingestion pipeline stays fully deterministic. No AI in the critical path. The AI observer runs *after* ingestion as a background analyst — it watches the system and suggests improvements. It never blocks ingestion, never decides schema, never decides classification, never decides routing.

---

## Architecture Overview

```
[Redis Streams / ES / Loki]
         |
    [ai-observer]
    Ticker (30 min)
         |
   [Category passes — parallel]
    classifier | sidecar | ingestion
         |
    [LLM Provider]
    Anthropic Haiku (default)
         |
    [ai_insights ES index]
         |
    [Dashboard / Operator UX]
```

---

## Service Layout

New top-level service `ai-observer/`, following the same structure as `pipeline/`, `classifier/`, etc.

```
ai-observer/
  cmd/
    ai-observer/
      main.go
  internal/
    bootstrap/        # config -> logger -> ES -> Redis -> categories -> scheduler
    category/
      interface.go    # Category interface
      classifier/     # Classifier drift analysis
      sidecar/        # Sidecar anomaly detection
      ingestion/      # Ingestion failure patterns
    provider/
      interface.go    # LLMProvider interface
      anthropic/      # Anthropic implementation (Haiku default)
    scheduler/        # Ticker loop + cost-ceiling mutex
    sampler/          # Per-source sampling logic
    insights/         # ai_insights ES index writer
  Taskfile.yml
  go.mod
```

Uses existing `infrastructure/` packages: `config`, `logger`, `elasticsearch`, `redis`. No new runtime dependencies except the Anthropic Go SDK.

The service is a **single binary with no HTTP server**. It starts, waits for the ticker, runs parallel category passes, emits insights, and sleeps. No API surface in v0.

---

## Category Interface

```go
type Category interface {
    Name() string
    Sample(ctx context.Context, window time.Duration) ([]Event, error)
    Analyze(ctx context.Context, events []Event, provider provider.LLMProvider) ([]Insight, error)
    MaxEventsPerRun() int
    ModelTier() string // "haiku" | "sonnet"
}
```

Each category owns its own sampling logic and declares which model tier it needs. The scheduler calls `Sample` then `Analyze` — no shared state between categories.

---

## Three v0 Categories

### Category 1: `classifier/` — Drift Detection

- **Source:** `*_classified_content` ES index, last N minutes
- **Sampling:** Over-sample docs with `confidence < 0.6`; small slice of high-confidence for regression detection
- **Grouping:** By domain + label pair; compute borderline rates
- **Prompt focus:** Label drift, borderline clusters, domains that consistently produce low-confidence classifications
- **Model:** Haiku

### Category 2: `sidecar/` — Anomaly Detection

- **Source:** Redis Streams pipeline events (fallback: Loki)
- **Sampling:** Non-zero exit codes; high-duration outliers (>2σ from mean); payload size anomalies
- **Grouping:** By sidecar name + error type
- **Prompt focus:** Elevated failure rates, latency outliers, recurring error patterns
- **Model:** Haiku

### Category 3: `ingestion/` — Failure Pattern Detection

- **Source:** Loki — extraction errors, missing-field warnings, malformed HTML logs from crawler/classifier
- **Sampling:** Cluster by domain + error type; prioritize recurring patterns
- **Prompt focus:** Domains with systematic extraction failures, recurring missing-field patterns
- **Model:** Haiku

---

## Provider Interface

```go
type LLMProvider interface {
    Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)
    Name() string
}

type GenerateRequest struct {
    SystemPrompt string
    UserPrompt   string
    MaxTokens    int
    JSONSchema   string // enforce structured output
}

type GenerateResponse struct {
    Content      string
    InputTokens  int
    OutputTokens int
}
```

**Anthropic implementation:** Uses `github.com/anthropic-ai/sdk-go`. Maps `ModelTier` to `claude-haiku-4-5-20251001` or `claude-sonnet-4-6`. Returns token counts for cost tracking. Retries on 429 with exponential backoff (max 3 attempts).

**Pluggability:** The interface is designed so OpenAI or a local model (Ollama) can be added later without touching observer logic.

---

## Scheduler Loop & Cost-Ceiling Mutex

```
Ticker fires every AI_OBSERVER_INTERVAL_SECONDS
  |
Reset interval token counter (mutex-protected)
  |
Launch goroutines for each enabled category (parallel)
  Each goroutine:
    Sample events from source
    Check token budget remaining (acquire mutex, check, release)
    If budget allows  -> call provider -> deduct tokens -> emit insights
    If budget exhausted -> log skip, emit "budget_exceeded" metric
  |
Wait for all goroutines
  |
Write all insights to ES ai_insights index
  |
Sleep until next tick
```

The mutex guards only the shared `tokensUsedThisInterval` counter. Categories otherwise run fully in parallel. This keeps cost deterministic regardless of event volume spikes.

---

## Insight Envelope Schema

```json
{
  "id": "ins_20260307_classifier_001",
  "created_at": "2026-03-07T14:30:00Z",
  "category": "classifier",
  "severity": "medium",
  "summary": "Borderline rate for label 'policy_document' on example.gov rose from 11% to 27%.",
  "details": {
    "domain": "example.gov",
    "label": "policy_document",
    "borderline_rate": 0.27,
    "previous_rate": 0.11,
    "sample_size": 143
  },
  "suggested_actions": [
    "Review classifier rules for 'policy_document' vs 'news_article' on example.gov.",
    "Add targeted test cases for borderline scores on this domain."
  ],
  "observer_version": "0.1.0",
  "model": "claude-haiku-4-5-20251001",
  "tokens_used": 1840
}
```

Written to a single `ai_insights` ES index (not per-source). Dashboard filters by `category`, `severity`, `domain`.

---

## Config Flags

```
AI_OBSERVER_ENABLED=true
AI_OBSERVER_INTERVAL_SECONDS=1800
AI_OBSERVER_MAX_TOKENS_PER_INTERVAL=25000
AI_OBSERVER_DRY_RUN=false
AI_OBSERVER_CATEGORY_CLASSIFIER_ENABLED=true
AI_OBSERVER_CATEGORY_SIDECAR_ENABLED=true
AI_OBSERVER_CATEGORY_INGESTION_ENABLED=true
```

`DRY_RUN=true` logs prompts and projected cost without making real API calls. Enables safe testing in production before enabling live insights.

---

## Rollout Phases

| Phase | What ships | Notes |
|-------|------------|-------|
| 0 | Service scaffold, ES `ai_insights` index, no AI calls | Validates infra wiring |
| 1 | DRY_RUN mode — logs prompts + projected cost | Validates sampling + prompt quality |
| 2 | Live calls in dev, insights reviewed against intuition | Validates insight quality |
| 3 | Production with classifier category only | Low-risk first category |
| 4 | All three categories enabled | Full v0 |

---

## What This Does Not Do

- No AI in the ingestion or classification critical path
- No AI required for routing, schema validation, or sidecar execution
- No real-time alerting (polling interval is 30 min by design)
- No automated remediation in v0 (insights are advisory only)

---

## Future Extensions (Not in Scope)

- Tiered scheduling (fast anomaly / slow drift) — v2
- GitHub issue automation on `severity=high` — v2
- Classifier rule suggestion assistant — v3
- Sidecar portfolio advisor — v3
- Minoo integration for content authoring anomalies — v3
