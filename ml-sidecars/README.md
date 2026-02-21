# ML Sidecars

Python/FastAPI services that provide specialized content classification for the North Cloud Classifier. Each sidecar exposes a two-endpoint HTTP API (`POST /classify`, `GET /health`) and runs as an independent Docker container.

## Overview

The Classifier service runs a four-step base pipeline (content type, quality score, topic detection, source reputation) and then calls zero or more ML sidecars for domain-specific classification. Each sidecar call is non-blocking: if the sidecar is unreachable or returns an error, the Classifier logs a warning and falls back to its rule-based result for that domain. When a sidecar is disabled via configuration, the corresponding field is omitted entirely from the classified Elasticsearch document.

The Classifier combines rule-based signals with the sidecar's ML prediction using a per-domain decision matrix. This hybrid approach gives high precision from rules (fast, deterministic) and higher recall from the ML model (handles edge cases and ambiguous phrasing).

Health of all sidecars is queryable at `GET /api/v1/metrics/ml-health` on the Classifier.

## Sidecars

### crime-ml

- **Port**: 8076
- **Image**: `docker.io/jonesrussell/crime-ml:latest`
- **Classifier env vars**: `CRIME_ENABLED=true`, `CRIME_ML_SERVICE_URL=http://crime-ml:8076`
- **Model version**: `1.0.0`

Classifies articles for street crime relevance. Runs three trained scikit-learn models loaded from joblib files at startup: relevance (3-class), crime type (multi-label), and location specificity (4-class). All three models use a TF-IDF vectorizer over the combined title and first 500 characters of body text.

**Relevance classes**:
- `core_street_crime` — murders, shootings, assaults with arrest; homepage-eligible
- `peripheral_crime` — impaired driving, international crime, policy; category pages only
- `not_crime` — not crime-related

**Crime type labels** (multi-label, threshold 0.3): `violent_crime`, `property_crime`, `drug_crime`, `gang_violence`, `organized_crime`, `criminal_justice`, `other_crime`

**Location classes**: `local_canada`, `national_canada`, `international`, `not_specified`

**POST /classify request**:
```json
{
  "title": "Man charged after downtown shooting",
  "body": "Police arrested a 34-year-old man..."
}
```

**POST /classify response**:
```json
{
  "relevance": "core_street_crime",
  "relevance_confidence": 0.91,
  "crime_types": ["violent_crime"],
  "crime_type_scores": {
    "violent_crime": 0.87,
    "property_crime": 0.12,
    "drug_crime": 0.05,
    "gang_violence": 0.09,
    "organized_crime": 0.03,
    "criminal_justice": 0.21,
    "other_crime": 0.02
  },
  "location": "local_canada",
  "location_confidence": 0.83,
  "processing_time_ms": 12
}
```

**GET /health response**:
```json
{
  "status": "healthy",
  "model_version": "1.0.0",
  "uptime_seconds": 3612.4
}
```

Models are stored in `ml-sidecars/crime-ml/models/` as `relevance.joblib`, `crime_type.joblib`, and `location.joblib`. The health endpoint does not check model load state; it returns 200 unconditionally after startup completes.

---

### mining-ml

- **Port**: 8077
- **Image**: `docker.io/jonesrussell/mining-ml:latest`
- **Classifier env vars**: `MINING_ENABLED=true`, `MINING_ML_SERVICE_URL=http://mining-ml:8077`
- **Model version**: `2025-02-01-mining-v1`

Classifies articles for mining and natural resources industry relevance. Runs four trained scikit-learn models: relevance (3-class), mining stage (4-class), commodity (multi-label), and location (4-class). If model files are absent at build time, `train_and_export.py` generates placeholder models from synthetic training data so the container starts successfully. Production models replace these files.

The health endpoint returns HTTP 503 when models fail to load, allowing the Classifier's `depends_on: service_healthy` check to gate startup.

**Relevance classes**: `core_mining`, `peripheral_mining`, `not_mining`

**Mining stage classes**: `exploration`, `development`, `production`, `unspecified`

**Commodity labels** (multi-label, threshold 0.3): `gold`, `copper`, `lithium`, `nickel`, `uranium`, `iron_ore`, `rare_earths`, `other`

**Location classes**: `local_canada`, `national_canada`, `international`, `not_specified`

**POST /classify request**:
```json
{
  "title": "Junior miner reports drill results at Ontario gold project",
  "body": "The company intersected 2.4 g/t gold over 18 metres..."
}
```

**POST /classify response**:
```json
{
  "relevance": "core_mining",
  "relevance_confidence": 0.88,
  "mining_stage": "exploration",
  "mining_stage_confidence": 0.76,
  "commodities": ["gold"],
  "commodity_scores": {
    "gold": 0.81,
    "copper": 0.08,
    "lithium": 0.03,
    "nickel": 0.04,
    "uranium": 0.01,
    "iron_ore": 0.02,
    "rare_earths": 0.02,
    "other": 0.11
  },
  "location": "local_canada",
  "location_confidence": 0.79,
  "processing_time_ms": 18,
  "model_version": "2025-02-01-mining-v1"
}
```

When models are not loaded (startup failure), the endpoint returns HTTP 200 with `relevance: "not_mining"` and zero confidence scores rather than erroring. This allows the Classifier to receive a safe default when model loading fails after the health check passes.

**GET /health response** (models loaded):
```json
{
  "status": "healthy",
  "model_version": "2025-02-01-mining-v1",
  "models_loaded": true,
  "uptime_seconds": 128.7
}
```

**GET /health response** (models not loaded): HTTP 503

---

### coforge-ml

- **Port**: 8078
- **Image**: `docker.io/jonesrussell/coforge-ml:latest`
- **Classifier env vars**: `COFORGE_ENABLED=true`, `COFORGE_ML_SERVICE_URL=http://coforge-ml:8078`
- **Model version**: `2026-02-08-coforge-v1`

Classifies articles for Coforge-relevant technology content targeting developers and entrepreneurs. Runs four trained scikit-learn models: relevance (3-class), audience (3-class), topic (multi-label), and industry (multi-label). The health endpoint returns HTTP 503 when models fail to load.

**Relevance classes**: `core_coforge`, `peripheral`, `not_relevant`

**Audience classes**: `developer`, `entrepreneur`, `hybrid`

**Topic labels** (multi-label, threshold 0.3): `framework_release`, `open_source`, `devtools`, `api_sdk`, `language_update`, `engineering_culture`, `funding_round`, `acquisition`, `product_launch`, `founder_story`, `market_analysis`, `partnership`, `developer_experience`, `ai_ml`

**Industry labels** (multi-label, threshold 0.3): `ai_ml`, `fintech`, `saas`, `devtools`, `cloud_infra`, `cybersecurity`, `healthtech`, `other`

**POST /classify request**:
```json
{
  "title": "Vercel ships new AI SDK for React developers",
  "body": "The open-source toolkit lets developers integrate..."
}
```

**POST /classify response**:
```json
{
  "relevance": "core_coforge",
  "relevance_confidence": 0.84,
  "audience": "developer",
  "audience_confidence": 0.79,
  "topics": ["framework_release", "open_source", "developer_experience", "ai_ml"],
  "topic_scores": {
    "framework_release": 0.72,
    "open_source": 0.68,
    "devtools": 0.29,
    "api_sdk": 0.41,
    "language_update": 0.11,
    "engineering_culture": 0.08,
    "funding_round": 0.05,
    "acquisition": 0.03,
    "product_launch": 0.55,
    "founder_story": 0.06,
    "market_analysis": 0.12,
    "partnership": 0.09,
    "developer_experience": 0.63,
    "ai_ml": 0.77
  },
  "industries": ["ai_ml", "saas"],
  "industry_scores": {
    "ai_ml": 0.81,
    "fintech": 0.04,
    "saas": 0.61,
    "devtools": 0.28,
    "cloud_infra": 0.19,
    "cybersecurity": 0.03,
    "healthtech": 0.02,
    "other": 0.07
  },
  "processing_time_ms": 14,
  "model_version": "2026-02-08-coforge-v1"
}
```

**GET /health response** (models loaded):
```json
{
  "status": "healthy",
  "model_version": "2026-02-08-coforge-v1",
  "models_loaded": true,
  "uptime_seconds": 94.2
}
```

**GET /health response** (models not loaded): HTTP 503

---

### entertainment-ml

- **Port**: 8079
- **Image**: `docker.io/jonesrussell/entertainment-ml:latest`
- **Classifier env vars**: `ENTERTAINMENT_ENABLED=true`, `ENTERTAINMENT_ML_SERVICE_URL=http://entertainment-ml:8079`
- **Model version**: `2026-02-08-entertainment-v1`

Classifies articles for entertainment industry relevance. This sidecar is rule-based (no trained ML model); the title and first 500 characters of body are matched against regex patterns. The service has no model loading phase and starts with no external dependencies beyond FastAPI itself (`requirements.txt` contains only `fastapi`, `uvicorn`, `pydantic`).

**Classification logic**: Core patterns (e.g., `film`, `movie`, `album`, `oscar`, `review`, `celebrity`, `war film`) produce `core_entertainment`. Peripheral patterns (`entertainment`, `music`, `streaming`) produce `peripheral_entertainment`. No match produces `not_entertainment`. Confidence scales from 0.6 (base, no match) to 0.65 (peripheral) up to 0.95 (multiple core hits: `0.6 + 0.1 * core_hit_count`, capped at 0.95).

**Relevance classes**: `core_entertainment`, `peripheral_entertainment`, `not_entertainment`

**Category labels** (subset, up to 5): `film`, `war_film`, `television`, `music`, `gaming`, `reviews`, `celebrity`

**POST /classify request**:
```json
{
  "title": "Marvel's new film breaks box office records on opening weekend",
  "body": "The movie earned $200M domestically..."
}
```

**POST /classify response**:
```json
{
  "relevance": "core_entertainment",
  "relevance_confidence": 0.80,
  "categories": ["film"],
  "processing_time_ms": 1,
  "model_version": "2026-02-08-entertainment-v1"
}
```

**GET /health response**:
```json
{
  "status": "healthy",
  "model_version": "2026-02-08-entertainment-v1"
}
```

The health endpoint always returns HTTP 200. There is no `models_loaded` field because this sidecar has no model files.

---

### anishinaabe-ml

- **Port**: 8080
- **Image**: `docker.io/jonesrussell/anishinaabe-ml:latest`
- **Classifier env vars**: `ANISHINAABE_ENABLED=true`, `ANISHINAABE_ML_SERVICE_URL=http://anishinaabe-ml:8080`
- **Model version**: `2026-02-16-anishinaabe-v1`

Classifies articles for Anishinaabe and Indigenous content. Like `entertainment-ml`, this sidecar is rule-based with no trained ML model. Core patterns cover specific identifiers (Anishinaabe, Ojibwe, Métis, Inuit, residential school, treaty rights). Peripheral patterns cover broader Indigenous language (indigenous, reconciliation, reserve). Confidence calculation follows the same formula as entertainment-ml.

**Relevance classes**: `core_anishinaabe`, `peripheral_anishinaabe`, `not_anishinaabe`

**Category labels** (up to 5): `culture`, `language`, `governance`, `land_rights`, `education`

Category assignment uses keyword matching: Anishinaabe/Ojibwe terms map to `culture`; language-related terms map to `language`; treaty/governance terms map to `governance`; land rights/reserve terms map to `land_rights`; education/residential school/reconciliation terms map to `education`.

**POST /classify request**:
```json
{
  "title": "Anishinaabe community leaders sign historic land agreement",
  "body": "The treaty signing ceremony took place..."
}
```

**POST /classify response**:
```json
{
  "relevance": "core_anishinaabe",
  "relevance_confidence": 0.70,
  "categories": ["culture", "governance", "land_rights"],
  "processing_time_ms": 1,
  "model_version": "2026-02-16-anishinaabe-v1"
}
```

**GET /health response**:
```json
{
  "status": "healthy",
  "model_version": "2026-02-16-anishinaabe-v1"
}
```

The health endpoint always returns HTTP 200. Disabled by default (`ANISHINAABE_ENABLED=false`) in both development and production compose configurations.

---

## Common Architecture

All five sidecars share the same structure:

```
{sidecar}/
├── main.py              # FastAPI app, lifespan, ClassifyRequest/Response models, two endpoints
├── classifier/
│   ├── __init__.py
│   ├── relevance.py     # Relevance classifier (all sidecars)
│   ├── preprocessor.py  # Text cleaning (ML-backed sidecars only)
│   └── *.py             # Domain-specific sub-classifiers
├── models/              # joblib model files (ML-backed sidecars only)
├── requirements.txt
└── Dockerfile
```

**Framework**: FastAPI with Pydantic v2 request/response models and uvicorn as the ASGI server.

**Input text**: All sidecars receive `title` (string, required) and `body` (string, optional). The classifiers concatenate them as `f"{title} {body[:500]}"` — only the first 500 characters of body are used.

**Model loading**: ML-backed sidecars (crime-ml, mining-ml, coforge-ml) load joblib files during the FastAPI `lifespan` context manager. This blocks startup until all models are loaded. Rule-based sidecars (entertainment-ml, anishinaabe-ml) have no model loading phase.

**ML models**: Where used, models are scikit-learn `LogisticRegression` wrapped in `OneVsRestClassifier` for multi-label tasks. Features are extracted with `TfidfVectorizer` (bigrams, max 500 features). The model, vectorizer, and optional `MultiLabelBinarizer` are saved together in a single joblib dict.

**Resource limits**: Each container runs with a 0.5 CPU and 512 MB memory limit in both development and production.

**Python version**: 3.11 (slim base image).

---

## Quick Start

### Docker (Recommended)

All sidecars are included in the standard compose stack:

```bash
# Start all services including ML sidecars
task docker:dev:up

# Or manually
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d crime-ml mining-ml coforge-ml entertainment-ml anishinaabe-ml

# Check health
curl http://localhost:8076/health
curl http://localhost:8077/health
curl http://localhost:8078/health
curl http://localhost:8079/health
curl http://localhost:8080/health

# Test classification
curl -s -X POST http://localhost:8076/classify \
  -H "Content-Type: application/json" \
  -d '{"title": "Man shot dead in downtown parking lot", "body": "Police are investigating..."}'
```

### Local Development

```bash
cd ml-sidecars/crime-ml
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
uvicorn main:app --reload --port 8076
```

For mining-ml, generate placeholder models before starting:

```bash
cd ml-sidecars/mining-ml
python train_and_export.py
uvicorn main:app --reload --port 8077
```

---

## Configuration

Each sidecar is enabled and configured in the Classifier via environment variables:

| Sidecar | Enable flag | URL variable | Default URL |
|---------|-------------|--------------|-------------|
| crime-ml | `CRIME_ENABLED` | `CRIME_ML_SERVICE_URL` | `http://crime-ml:8076` |
| mining-ml | `MINING_ENABLED` | `MINING_ML_SERVICE_URL` | `http://mining-ml:8077` |
| coforge-ml | `COFORGE_ENABLED` | `COFORGE_ML_SERVICE_URL` | `http://coforge-ml:8078` |
| entertainment-ml | `ENTERTAINMENT_ENABLED` | `ENTERTAINMENT_ML_SERVICE_URL` | `http://entertainment-ml:8079` |
| anishinaabe-ml | `ANISHINAABE_ENABLED` | `ANISHINAABE_ML_SERVICE_URL` | `http://anishinaabe-ml:8080` |

Default states in `docker-compose.dev.yml`:
- `MINING_ENABLED=true`, `COFORGE_ENABLED=true`, `ENTERTAINMENT_ENABLED=true`
- `ANISHINAABE_ENABLED=false`
- `CRIME_ENABLED` is not set in the dev compose (controlled by `.env`)

The sidecars themselves have no configurable environment variables beyond `MODEL_PATH` (set to `/app/models` in compose; not currently read by the Python code, which uses the relative path `models/` relative to the working directory).

---

## Development

### Running Tests

Each ML-backed sidecar has a `tests/` directory with pytest-based unit tests:

```bash
cd ml-sidecars/crime-ml
pip install -r requirements.txt
pytest
```

### Adding or Replacing a Model

1. Train your scikit-learn model with the appropriate vectorizer.
2. Export with joblib as a dict `{'model': model, 'vectorizer': vectorizer, 'classes': [...]}`. Multi-label models also include `'mlb': mlb`.
3. Place the `.joblib` file in `ml-sidecars/{sidecar}/models/`.
4. Rebuild the container: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build {sidecar}`.

### Generating Placeholder Models

`mining-ml` and `crime-ml` ship with `train_and_export.py` scripts that generate synthetic placeholder models for local development and Docker image builds. These models predict correctly only on trivially clear examples — replace them with real training data for production.

```bash
# mining-ml
python ml-sidecars/mining-ml/train_and_export.py

# crime-ml
python ml-sidecars/crime-ml/train_and_export.py
```

### Adding a New Sidecar

Follow the existing pattern:

1. Create `ml-sidecars/{name}/main.py` with `ClassifyRequest`, `ClassifyResponse`, and `HealthResponse` Pydantic models.
2. Expose `POST /classify` and `GET /health` endpoints.
3. Add a `Dockerfile` (use `python:3.11-slim`, expose the port, run uvicorn).
4. Add `requirements.txt`.
5. Add the service to `docker-compose.base.yml` with health check, resource limits, and the `service-defaults` anchor.
6. Add the port mapping to `docker-compose.dev.yml`.
7. Add the enable flag and URL to the Classifier's environment in `docker-compose.dev.yml` and `docker-compose.prod.yml`.
8. Implement the corresponding Go client in `classifier/internal/{name}mlclient/client.go` using `mltransport.DoClassify` and `mltransport.DoHealth`.
9. Implement the hybrid classifier in `classifier/internal/classifier/{name}.go`.

---

## Integration

### How the Classifier Calls Sidecars

The Classifier calls each sidecar using a shared Go transport layer in `classifier/internal/mltransport/transport.go`. All calls go to `POST {baseURL}/classify` with a 5-second HTTP timeout.

**Request** (identical for all sidecars):
```json
{
  "title": "<article title>",
  "body": "<article body text>"
}
```

The Classifier passes the full article body; each sidecar truncates to its own 500-character limit internally.

**Transport behavior**:
- HTTP client timeout: 5 seconds (hardcoded in `mltransport`)
- On non-200 response: returns an error wrapping the status code
- On network failure: wraps `ErrUnavailable` from the domain-specific client package
- The Classifier logs a warning and continues with rules-only output when any sidecar call fails

### Which Articles Trigger Sidecar Calls

The Classifier gates sidecar calls on content type and subtype (see `classifier/internal/classifier/classifier.go: runOptionalClassifiers()`):

| Content type / subtype | Sidecars called |
|------------------------|-----------------|
| Non-article (page, listing, etc.) | None |
| Article — event subtype | Location only (not ML sidecars) |
| Article — blotter subtype | crime-ml only |
| Article — report subtype | None |
| Article — all other subtypes | All enabled sidecars |

### Output in Classified Documents

When a sidecar is enabled and its call succeeds, the Classifier merges the ML result with its rule-based result via the decision matrix and writes the domain field to the Elasticsearch classified document. When the sidecar is disabled (`ENABLED=false`) or the hybrid classifier returns nil, the field is omitted entirely.

Example fields in a classified Elasticsearch document:

```json
{
  "crime": {
    "street_crime_relevance": "core_street_crime",
    "sub_label": "",
    "crime_types": ["violent_crime"],
    "location_specificity": "local_canada",
    "final_confidence": 0.92,
    "homepage_eligible": true,
    "category_pages": ["violent-crime", "crime"],
    "review_required": false,
    "rule_relevance": "core_street_crime",
    "rule_confidence": 0.90,
    "ml_relevance": "core_street_crime",
    "ml_confidence": 0.91,
    "decision_path": "both_agree"
  },
  "mining": {
    "relevance": "core_mining",
    "mining_stage": "exploration",
    "commodities": ["gold"],
    "location": "local_canada",
    "final_confidence": 0.88,
    "review_required": false,
    "model_version": "2025-02-01-mining-v1"
  },
  "coforge": {
    "relevance": "core_coforge",
    "audience": "developer",
    "topics": ["framework_release", "open_source"],
    "industries": ["ai_ml", "saas"],
    "final_confidence": 0.84
  },
  "entertainment": {
    "relevance": "core_entertainment",
    "categories": ["film"],
    "final_confidence": 0.80
  },
  "anishinaabe": {
    "relevance": "core_anishinaabe",
    "categories": ["culture", "language"],
    "final_confidence": 0.88,
    "review_required": false,
    "model_version": "2026-02-16-anishinaabe-v1"
  }
}
```

Publisher routes filter on these fields to route articles to the appropriate Redis channels (e.g., articles with `mining.relevance = "core_mining"` route to `articles:mining`).

### Monitoring Sidecar Health

The Classifier exposes a health endpoint that aggregates reachability, latency, and model version for all configured sidecars:

```bash
curl -H "Authorization: Bearer <token>" http://localhost:8071/api/v1/metrics/ml-health
```
