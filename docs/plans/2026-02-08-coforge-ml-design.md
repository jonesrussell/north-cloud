# Coforge-ML Sidecar Design

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create an ML classification sidecar for coforge.xyz that classifies content along four axes — relevance, audience, topic, and industry — to connect developers and entrepreneurs. Establish gold-standard patterns that will be backported to crime-ml and mining-ml.

**Architecture:** Python FastAPI sidecar with four scikit-learn models, integrated into the classifier service via a Go HTTP client following the existing hybrid (rules + ML) classification pattern.

**Tech Stack:** Python 3.11, FastAPI, scikit-learn, joblib, Go 1.24+ (classifier integration)

---

## 1. Directory Structure & API Contract

### Directory Layout

```
coforge-ml/
├── main.py                    # FastAPI server with lifespan, models_loaded guard
├── Dockerfile                 # Python 3.11-slim, port 8078
├── requirements.txt           # Pinned versions matching crime-ml/mining-ml
├── train_and_export.py        # Training pipeline (real, not placeholder)
├── .gitignore
├── classifier/
│   ├── __init__.py
│   ├── relevance.py           # 3-class: core_coforge, peripheral, not_relevant
│   ├── audience.py            # 3-class: developer, entrepreneur, hybrid
│   ├── topic.py               # Multi-label: ~14 tags
│   ├── industry.py            # Multi-label: ~8 verticals
│   └── preprocessor.py        # Unified version (character-based, mining-ml style)
├── models/
│   ├── relevance.joblib
│   ├── audience.joblib
│   ├── topic.joblib
│   └── industry.joblib
└── tests/
    ├── test_preprocessor.py
    ├── test_api.py
    ├── test_relevance.py
    ├── test_audience.py
    ├── test_topic.py
    └── test_industry.py
```

### API Endpoints

**POST /classify**

Request:
```json
{"title": "string", "body": "string"}
```

Response:
```json
{
  "relevance": "core_coforge",
  "relevance_confidence": 0.92,
  "audience": "hybrid",
  "audience_confidence": 0.78,
  "topics": ["funding_round", "devtools"],
  "topic_scores": {"funding_round": 0.88, "devtools": 0.71},
  "industries": ["saas", "ai_ml"],
  "industry_scores": {"saas": 0.82, "ai_ml": 0.65},
  "processing_time_ms": 52,
  "model_version": "2026-02-08-coforge-v1"
}
```

**GET /health**

Returns status, model_version, and models_loaded flag.

Port **8078** (sequence: crime-ml 8076, mining-ml 8077, coforge-ml 8078).

---

## 2. Gold Standard Patterns

These six patterns fix inconsistencies found across crime-ml and mining-ml. Coforge-ml establishes the reference implementation.

### 2.1 Error Handling — models_loaded Guard

```python
@app.post("/classify")
async def classify(request: ClassifyRequest):
    if not state.models_loaded:
        return default_response(processing_time_ms=0)
```

No crashes if models fail to load. Return zero-confidence defaults. Crime-ml currently lacks this guard.

### 2.2 Model Versioning — Date-Prefixed Constant

```python
MODEL_VERSION = "2026-02-08-coforge-v1"
```

Defined once, returned in both `/classify` and `/health`. No more hardcoded `"1.0.0"` that never gets updated.

### 2.3 Preprocessor — Unified Character-Based Approach

```python
text = "".join(c if (c.isalnum() or c == "_" or c.isspace()) else " " for c in text)
```

Mining-ml's approach, not crime-ml's regex. Slightly safer against ReDoS edge cases. Same 1M character cap, same URL/email stripping.

### 2.4 Full Test Suite From Day One

Every classifier gets unit tests. API endpoint tests. Preprocessor tests. Mining-ml shipped without tests — coforge-ml won't.

### 2.5 Default URL Constant in Go Config

```go
const defaultCoforgeMLServiceURL = "http://coforge-ml:8078"
```

Prevents nil-pointer if env var is missing. Mining-ml's Go client is missing this.

### 2.6 Multi-Label Fallback Scoring

When `predict_proba` isn't available (LinearSVC), use `decision_function` scores normalized through sigmoid rather than raw binary 1.0/0.0. Gives more useful confidence even in fallback mode.

---

## 3. Label Taxonomy

### 3.1 Relevance (3-class, single-label)

| Label | Meaning |
|---|---|
| `core_coforge` | Directly about the dev-entrepreneur intersection — startup launches, dev tool funding, open-source business models |
| `peripheral` | Adjacent — general tech news or general business news that one audience might care about |
| `not_relevant` | Neither audience — celebrity gossip, sports, unrelated verticals |

### 3.2 Audience (3-class, single-label)

| Label | Meaning |
|---|---|
| `developer` | Primarily technical — framework releases, language updates, engineering deep-dives |
| `entrepreneur` | Primarily business — funding rounds, market shifts, founder stories, growth strategy |
| `hybrid` | Both — "AI startup open-sources their SDK", "Developer tool raises Series A" |

### 3.3 Topic (multi-label, threshold 0.3)

| Label | Domain | Examples |
|---|---|---|
| `framework_release` | Tech | "React 20 released", "New Go generics features" |
| `open_source` | Tech | "Project goes open-source", "CNCF adopts new project" |
| `devtools` | Tech | "New IDE plugin", "CI/CD platform launches" |
| `api_sdk` | Tech | "Stripe releases new SDK", "API v2 announced" |
| `language_update` | Tech | "Python 3.15 features", "Rust edition 2026" |
| `engineering_culture` | Tech | "How we scaled to 1M users", "Remote eng team practices" |
| `funding_round` | Business | "Series A", "Seed round", "IPO" |
| `acquisition` | Business | "Company X acquires Y" |
| `product_launch` | Business | "New SaaS product announced" |
| `founder_story` | Business | "From side project to $10M ARR" |
| `market_analysis` | Business | "State of DevTools 2026", "SaaS market trends" |
| `partnership` | Business | "Strategic partnership announced" |
| `developer_experience` | Cross | "DX improvements", "Developer advocacy" |
| `ai_ml` | Cross | "New LLM capabilities", "AI-powered dev tools" |

### 3.4 Industry (multi-label, threshold 0.3)

`ai_ml`, `fintech`, `saas`, `devtools`, `cloud_infra`, `cybersecurity`, `healthtech`, `other`

---

## 4. Classifier Integration (Go Side)

### 4.1 New Client Package

```
classifier/internal/coforgemlclient/
├── client.go          # HTTP client, 5s timeout, POST /classify
└── client_test.go     # Unit tests with httptest
```

### 4.2 Config

```go
type CoforgeConfig struct {
    Enabled      bool   `env:"COFORGE_ENABLED" yaml:"enabled"`
    MLServiceURL string `env:"COFORGE_ML_SERVICE_URL" yaml:"ml_service_url"`
}
```

Default: `const defaultCoforgeMLServiceURL = "http://coforge-ml:8078"`

### 4.3 Bootstrap

```go
func createCoforgeClassifier(cfg *config.Config, log infralogger.Logger) *classifier.CoforgeClassifier {
    if !cfg.Classification.Coforge.Enabled {
        return nil
    }
    mlClient := coforgemlclient.NewClient(cfg.Classification.Coforge.MLServiceURL)
    return classifier.NewCoforgeClassifier(mlClient, log, true)
}
```

### 4.4 Hybrid Classification (3-Layer)

1. **Rules layer**: Keyword patterns for high-confidence signals ("Series A", "open-source", "SDK", "raised $X")
2. **ML layer**: coforge-ml sidecar call (title + first 500 chars of body)
3. **Decision matrix**: Merge rules + ML, flag `review_required` on conflicts

| Rules | ML | Result | ReviewRequired |
|-------|-----|--------|----------------|
| core | core | core, high conf | false |
| core | not_relevant | core | true |
| core | - (unavailable) | core, rule conf | false |
| peripheral | core, high conf (>0.9) | peripheral->core | optional |
| not_relevant | core, high conf | peripheral | true |

### 4.5 Classified Content Output

```json
{
  "coforge": {
    "relevance": "core_coforge",
    "relevance_confidence": 0.92,
    "audience": "hybrid",
    "audience_confidence": 0.78,
    "topics": ["funding_round", "devtools"],
    "industries": ["saas", "ai_ml"]
  }
}
```

Nested under a `coforge` key — same pattern as `crime` and `mining`.

### 4.6 Graceful Degradation

If coforge-ml is unreachable, rules-only mode. Log a warning, continue processing. No pipeline stalls.

---

## 5. Docker & Publisher Routing

### 5.1 Docker Compose

**Base** (`docker-compose.base.yml`):
```yaml
coforge-ml:
  build:
    context: ./coforge-ml
    dockerfile: Dockerfile
  image: docker.io/jonesrussell/coforge-ml:latest
  deploy:
    resources:
      limits:
        cpus: "0.5"
        memory: 512M
  environment:
    MODEL_PATH: /app/models
  healthcheck:
    test: ["CMD", "curl", "-f", "http://localhost:8078/health"]
    interval: 30s
    timeout: 10s
    retries: 3
    start_period: 15s
```

**Dev** (`docker-compose.dev.yml`):
```yaml
classifier:
  depends_on:
    coforge-ml:
      condition: service_healthy
  environment:
    COFORGE_ENABLED: "true"
    COFORGE_ML_SERVICE_URL: "http://coforge-ml:8078"

coforge-ml:
  ports:
    - "${COFORGE_ML_PORT:-8078}:8078"
```

### 5.2 Elasticsearch Mapping

New nested object in classified_content:
```go
"coforge": map[string]any{
    "type": "object",
    "properties": getCoforgeFields(),
}
```

Fields: `relevance` (keyword), `relevance_confidence` (float), `audience` (keyword), `audience_confidence` (float), `topics` (keyword array), `industries` (keyword array).

### 5.3 Publisher Routing — Redis Channels

- `articles:coforge` — all core_coforge content
- `articles:coforge:developer` — developer-audience content
- `articles:coforge:entrepreneur` — entrepreneur-audience content
- `articles:coforge:hybrid` — hybrid content (both audiences)

Routes filter on `coforge.relevance = core_coforge` (or peripheral, depending on channel quality threshold), then split by `coforge.audience`. A single article can hit multiple channels.

---

## 6. Backport Checklist (Crime-ML & Mining-ML)

Once coforge-ml is built and validated, these fixes get applied to the existing sidecars.

### Crime-ML Backports

| Fix | What Changes |
|---|---|
| Add `models_loaded` guard | `main.py` — add flag + default response on `/classify` |
| Model version constant | `main.py` — replace `"1.0.0"` with `"YYYY-MM-DD-crime-vN"` |
| Unify preprocessor | `classifier/preprocessor.py` — switch from regex to character-based |
| Multi-label fallback scoring | `classifier/crime_type.py` — sigmoid over `decision_function` instead of binary |

### Mining-ML Backports

| Fix | What Changes |
|---|---|
| Add test suite | Create `tests/` directory mirroring crime-ml's structure |
| Default URL constant | `classifier/internal/config/config.go` — add `defaultMiningMLServiceURL` |
| Multi-label fallback scoring | `classifier/commodity.py` — same sigmoid fix |

### Go Classifier Backports

| Fix | What Changes |
|---|---|
| Default URL for mining-ml | `classifier/internal/config/config.go` |
| Consistent client patterns | Align `mlclient` and `miningmlclient` naming/error handling |

---

## Summary of Changes by Service

| Service | Changes |
|---------|---------|
| **coforge-ml** (new) | Full ML sidecar — 4 models, FastAPI, tests, gold-standard patterns |
| **classifier** | New `coforgemlclient` package, `CoforgeClassifier`, config, bootstrap |
| **index-manager** | Add `coforge` nested object to classified_content mapping |
| **publisher** | New routes for `articles:coforge`, `articles:coforge:developer`, `articles:coforge:entrepreneur`, `articles:coforge:hybrid` |
| **docker-compose** | Add coforge-ml service to base, dev, and prod |
| **crime-ml** (backport) | 4 fixes: models_loaded guard, version constant, preprocessor, fallback scoring |
| **mining-ml** (backport) | 3 fixes: test suite, default URL, fallback scoring |
