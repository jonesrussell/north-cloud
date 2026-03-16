# Unified ML Sidecar Framework Design

**Date:** 2026-03-16
**Status:** Approved
**Scope:** Replace 5 separate ML sidecars with a unified framework

---

## Problem

North Cloud has 5 ML sidecars (crime, mining, coforge, entertainment, indigenous) that were built by copy-pasting a FastAPI template. Each is a standalone container with its own health check, logging, error handling, and response format. The Go classifier has 5 near-identical client packages wrapping a shared transport.

Pain points:
- Code duplication across both Python sidecars and Go clients
- Inconsistent health checks (crime always returns 200, mining returns 503 on failure)
- Inconsistent model loading (mining generates placeholders, crime crashes on missing models)
- Response format drift between modules
- Image bloat (1-2.5GB from venv copies)
- No timeout, retry, or circuit breaker in the Go transport
- No model versioning
- No standardized metrics or structured logging
- Adding a new module requires copying and modifying many files across 3 directories

A 6th module (drill extractor) is planned. The current pattern does not scale.

---

## Constraints (Non-Negotiable)

- Go classifier remains stateless and cannot embed Python
- Python models remain isolated but follow a unified pattern
- Latency stays low (FastAPI is fine)
- Deployment stays simple (Docker, Compose, K8s-ready)
- Observability is first-class (structured logs, metrics, health endpoints)
- All sidecars share a single API schema, error schema, logging format, and health check
- All sidecars support explicit model versioning
- All Go clients are unified into a single transport and interface layer
- No heavyweight ML-ops platforms (Triton, Seldon, TF Serving, Ray, etc.)

---

## Architecture: Monorepo Shared Library (Approach A)

A single Python package (`ml-framework/`) provides the base FastAPI app, schemas, logging, metrics, and the ModelModule interface. Each domain (crime, mining, coforge, entertainment, indigenous, drill) is a module implementing the interface. Each module builds into its own container using a shared Dockerfile template.

### Why This Approach

- Enforces standardization at the code level (not just convention)
- Preserves container isolation per domain
- Modules are thin plugins with domain logic only
- Single Dockerfile eliminates operational drift
- Adding a new module is 6 steps, not 60

### Rejected Alternatives

- **Single container, multi-model:** Violates isolation. One crash takes down all models. Cannot scale independently.
- **Shared base image, independent apps:** No code-level enforcement. Drift returns within 2-3 modules.

---

## Directory Structure

```
ml-framework/                          # shared Python package
  pyproject.toml                       # package metadata, deps (nc-ml)
  nc_ml/
    __init__.py
    main.py                            # MODULE_NAME env var -> dynamic import -> create_app()
    app.py                             # FastAPI app factory: create_app(module)
    module.py                          # BaseModule, ClassifierModule, ExtractorModule ABCs
    schemas.py                         # ClassifyRequest, StandardResponse, StandardError, HealthResponse
    health.py                          # /health endpoint (model version, readiness, uptime)
    logging.py                         # structured JSON logger (structlog, service name, request_id)
    metrics.py                         # Prometheus metrics (request count, latency, errors)
    middleware.py                      # request_id injection, timing, error handling
    model_loader.py                    # joblib loader with version tracking

ml-modules/
  crime/
    module.py                          # CrimeModule(ClassifierModule)
    models/                            # serialized model files
    requirements.txt                   # scikit-learn, joblib
    tests/
  mining/
    module.py                          # MiningModule(ClassifierModule)
    models/
    requirements.txt
    tests/
  coforge/
    module.py                          # CoforgeModule(ClassifierModule)
    models/
    requirements.txt
    tests/
  entertainment/
    module.py                          # EntertainmentModule(ClassifierModule) rule-based
    tests/
  indigenous/
    module.py                          # IndigenousModule(ClassifierModule) rule-based
    tests/
  drill/
    module.py                          # DrillModule(ExtractorModule) regex + LLM hybrid
    requirements.txt
    tests/

docker/
  Dockerfile.ml-sidecar               # shared template: ARG MODULE_NAME
```

---

## Module Interface (ABCs)

### BaseModule (shared lifecycle)

```python
class BaseModule(ABC):
    @abstractmethod
    def name(self) -> str:
        """Module identifier: 'crime', 'mining', 'drill', etc."""

    @abstractmethod
    def version(self) -> str:
        """Model/rule version string: '2025-02-01-crime-v2'"""

    @abstractmethod
    def schema_version(self) -> str:
        """Output schema version: '1.0'. Bump on breaking result changes."""

    @abstractmethod
    async def initialize(self) -> None:
        """Called once at startup. Load models, compile regexes, warm caches.
        Raise if the module cannot serve requests."""

    @abstractmethod
    async def shutdown(self) -> None:
        """Called on app shutdown. Release resources."""

    @abstractmethod
    async def health_checks(self) -> dict[str, bool]:
        """Granular health: {'relevance_model': True, 'type_model': False}.
        Return empty dict for rule-based modules."""
```

- `initialize()` is async so models load in thread pool without blocking the event loop
- If `initialize()` raises, health is set to `unhealthy`, container fails Docker healthcheck
- `health_checks()` aggregates into three-state status: all True -> healthy, some False -> degraded, all False or init failed -> unhealthy

### ClassifierModule

```python
class ClassifierModule(BaseModule):
    @abstractmethod
    async def classify(self, request: ClassifyRequest) -> ClassifierResult:
        """Run classification. Return domain-specific result with relevance/confidence."""

class ClassifierResult(ModuleResult):
    relevance: float          # 0.0-1.0
    confidence: float         # 0.0-1.0
```

Domain modules subclass ClassifierResult:

```python
class CrimeResult(ClassifierResult):
    crime_types: list[str]
    crime_type_scores: dict[str, float]
    location_detected: bool

class MiningResult(ClassifierResult):
    mining_stage: str
    mining_stage_confidence: float
    commodities: list[str]
    commodity_scores: dict[str, float]

class EntertainmentResult(ClassifierResult):
    categories: list[str]
```

### ExtractorModule

```python
class ExtractorModule(BaseModule):
    @abstractmethod
    async def extract(self, request: ClassifyRequest) -> ExtractorResult:
        """Run extraction. Return domain-specific structured data."""

class ExtractorResult(ModuleResult):
    pass
```

```python
class DrillResult(ExtractorResult):
    intercepts: list[DrillIntercept]
    extraction_method: Literal["regex", "llm", "hybrid"]
    raw_matches: int
    validated_matches: int

class DrillIntercept(BaseModel):
    hole_id: str
    commodity: str
    intercept_m: float
    grade: float
    unit: str
```

The framework promotes relevance and confidence from ClassifierResult to the envelope. ExtractorResult sets both to None.

---

## Standard Schemas

### Request

```python
class ClassifyRequest(BaseModel):
    title: str
    body: str
    metadata: dict[str, Any] | None = None  # optional pass-through
```

### Response Envelope

```python
class StandardResponse(BaseModel):
    module: str                        # "crime", "mining", "drill"
    version: str                       # model/rule version
    schema_version: str                # "1.0"
    result: ModuleResult               # domain-specific (polymorphic)
    relevance: float | None = None     # classifiers set, extractors null
    confidence: float | None = None    # classifiers set, extractors null
    processing_time_ms: float
    request_id: str
```

### Error

```python
class StandardError(BaseModel):
    error: str                         # machine-readable: "model_load_failed", "prediction_error"
    message: str                       # human-readable
    module: str
    request_id: str
    timestamp: datetime                # UTC ISO 8601
```

HTTP status mapping:
- 400: validation error (bad input)
- 422: Pydantic validation failure
- 500: prediction/extraction error
- 503: module not ready

### Health

```python
class HealthResponse(BaseModel):
    status: Literal["healthy", "degraded", "unhealthy"]
    module: str
    version: str
    schema_version: str
    models_loaded: bool                # True for rule-based (no models to fail)
    uptime_seconds: float
    checks: dict[str, bool] | None = None
```

---

## App Factory

```python
# nc_ml/app.py
def create_app(module: BaseModule) -> FastAPI:
    app = FastAPI(title=f"nc-ml-{module.name()}")
    # Registers: startup/shutdown lifecycle, /classify endpoint, /health endpoint,
    # /metrics endpoint, middleware (request_id, timing, error wrapping)
    # Detects ClassifierModule vs ExtractorModule, calls classify() or extract()
    # Wraps result in StandardResponse envelope
    return app
```

Module entrypoint:

```python
# nc_ml/main.py
module_name = os.environ["MODULE_NAME"]
mod = importlib.import_module(f"{module_name}.module")
app = create_app(mod.Module())
```

---

## Unified Go Client

### Package Structure

```
classifier/internal/mlclient/
  client.go          # single Client struct for any module
  transport.go       # HTTP transport with retry, timeout, circuit breaker
  schemas.go         # Go structs mirroring Python envelopes
  errors.go          # ErrUnavailable, ErrUnhealthy, ErrTimeout, ErrSchemaVersion
  options.go         # WithTimeout, WithRetry, WithCircuitBreaker
  client_test.go
  transport_test.go
```

### Go Schemas

```go
type StandardResponse struct {
    Module           string          `json:"module"`
    Version          string          `json:"version"`
    SchemaVersion    string          `json:"schema_version"`
    Result           json.RawMessage `json:"result"`      // domain-specific, decoded by caller
    Relevance        *float64        `json:"relevance"`
    Confidence       *float64        `json:"confidence"`
    ProcessingTimeMs float64         `json:"processing_time_ms"`
    RequestID        string          `json:"request_id"`
}
```

Result is json.RawMessage. The client deserializes the envelope but never interprets the domain payload. Domain-specific code in the classifier unmarshals Result into typed structs.

### Client Interface

```go
func NewClient(moduleName, baseURL string, opts ...Option) *Client
func (c *Client) Classify(ctx context.Context, title, body string) (*StandardResponse, error)
func (c *Client) Health(ctx context.Context) (*HealthResponse, error)
```

### Transport Options

```go
type clientOptions struct {
    timeout        time.Duration       // default: 5s
    retryCount     int                 // default: 1 (no retry)
    retryBaseDelay time.Duration       // default: 100ms, exponential backoff
    breakerTrips   int                 // default: 5
    breakerCooldown time.Duration      // default: 30s
}
```

- Retry only on network errors and 503. Never retry 400/422/500.
- Circuit breaker: closed -> open -> half-open. Simple Go implementation, no external library.
- When circuit is open, Classify() returns ErrUnavailable immediately.

### Classifier Integration

```go
type Processor struct {
    clients map[string]*mlclient.Client  // keyed by module name
}
```

Domain code deserializes Result into typed structs per module. The existing non-blocking pattern (log warning, fall back to rule-based) works unchanged.

### Deleted After Migration

- `classifier/internal/mltransport/`
- `classifier/internal/miningmlclient/`
- `classifier/internal/coforgemlclient/`
- `classifier/internal/entertainmentmlclient/`
- `classifier/internal/indigenousmlclient/`
- Old `classifier/internal/mlclient/`

---

## Observability

### Structured Logging

All modules use the same JSON logger via structlog. One JSON object per line to stdout.

```json
{
  "timestamp": "2026-03-16T14:32:01.123Z",
  "level": "info",
  "service": "nc-ml-crime",
  "module": "crime",
  "request_id": "req-abc123",
  "message": "classification complete",
  "duration_ms": 12.4,
  "relevance": 0.91
}
```

Framework logs startup, shutdown, request begin/end, errors, and health checks automatically. Modules get a pre-configured logger with context pre-bound (self.logger). Loki collects via existing Docker log driver pipeline.

### Prometheus Metrics

Every sidecar exposes GET /metrics. Framework registers standard metrics:

| Metric | Type | Labels |
|---|---|---|
| ncml_requests_total | Counter | module, status |
| ncml_request_duration_seconds | Histogram | module |
| ncml_prediction_duration_seconds | Histogram | module |
| ncml_errors_total | Counter | module, error_code |
| ncml_model_info | Gauge | module, version, schema_version |
| ncml_health_status | Gauge | module, status |

Modules can register custom metrics with mandatory ncml_ prefix.

### Go Client Observability

Structured logs via infrastructure/logger on every classify call. Optional metrics hook (WithOnRequest) keeps mlclient dependency-free.

### Grafana Queries

- Loki: `{service=~"nc-ml-.*"}`, `{service="nc-ml-crime"}`
- Prometheus: `rate(ncml_requests_total[5m])`, `histogram_quantile(0.95, rate(ncml_request_duration_seconds_bucket[5m]))`

No new infrastructure required.

---

## Deployment

### Shared Dockerfile

```dockerfile
ARG PYTHON_VERSION=3.12
FROM python:${PYTHON_VERSION}-slim AS base

RUN apt-get update && apt-get install -y --no-install-recommends libgomp1 \
    && rm -rf /var/lib/apt/lists/*

COPY ml-framework/ /opt/ml-framework/
RUN pip install --no-cache-dir /opt/ml-framework/

ARG MODULE_NAME
COPY ml-modules/${MODULE_NAME}/ /opt/module/
RUN if [ -f /opt/module/requirements.txt ]; then \
        pip install --no-cache-dir -r /opt/module/requirements.txt; \
    fi

ENV MODULE_NAME=${MODULE_NAME} PORT=8080 WORKERS=1 LOG_LEVEL=info
EXPOSE ${PORT}

HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:${PORT}/health')"

ENTRYPOINT ["python", "-m", "uvicorn", "nc_ml.main:app", \
    "--host", "0.0.0.0", "--port", "${PORT}", "--workers", "${WORKERS}"]
```

- No venv in the image (deps installed directly, significant size reduction)
- Framework and module are separate COPY layers (independent caching)
- WORKERS=1 default (scale horizontally, not vertically)
- start-period=15s for model loading time

### Docker Compose

Uses x-ml-sidecar YAML anchor for shared defaults. Per-module overrides for memory limits (mining: 1G, rule-based: 256M) and ports. Dev override adds ml profile and volume mounts for hot reload.

### Adding a New Module

1. Create `ml-modules/{name}/module.py`
2. Add `requirements.txt` if needed
3. Add service block to docker-compose
4. Add `{NAME}_ENABLED` + `{NAME}_ML_SERVICE_URL` to classifier config
5. Add client entry in processor
6. Done

### CI Build

One build command per module, same Dockerfile, tagged with commit SHA for traceability.

---

## Versioning Strategy

Three independent version axes:

### Model Version

Format: `YYYY-MM-DD-{module}-v{N}` (e.g., `2025-02-01-crime-v2`)

Tracks the specific model artifact or rule logic revision. Hardcoded in version(). Returned in responses, health, and Prometheus gauge. Rule-based modules use the same format.

### Schema Version

Format: `{major}.{minor}` (e.g., `1.0`, `2.0`)

Tracks the shape of a module's result payload. Major bump for breaking changes (field removed/renamed/type changed). Minor bump for additions. Go client logs warning on mismatch, proceeds by default.

### API Version

Format: URL path prefix `/v1/`

Tracks the framework's HTTP contract. Expect this to stay at v1 for a long time.

| What changed | Model Version | Schema Version | API Version |
|---|---|---|---|
| Retrained model | bumps | unchanged | unchanged |
| Updated rules | bumps | unchanged | unchanged |
| Added result field | unchanged | minor bump | unchanged |
| Renamed result field | unchanged | major bump | unchanged |
| Changed envelope | unchanged | unchanged | bumps |
| Added new module | starts v1 | starts 1.0 | unchanged |

---

## Migration Plan

### Phase 1: Build the Framework (no production impact)

Create ml-framework/ and ml-modules/ with full framework code. Write framework tests. Verify stub containers start and respond to /health.

### Phase 2: Port Crime as Proof

Migrate crime-ml into the framework. Compare outputs against old sidecar for a test corpus. Verify identical results.

### Phase 3: Build Unified Go Client

Create new mlclient package. Wire crime-ml through new client. Keep old clients for other modules.

### Phase 4: Deploy Crime to Production

Replace old crime-ml with new. Monitor Grafana. Verify health, metrics, logs.

Rollback: revert compose and classifier to previous image tags.

### Phase 5: Port Remaining Modules (parallel, by complexity)

1. Entertainment (rule-based, simplest)
2. Indigenous (rule-based)
3. Coforge (4 ML models)
4. Mining (4 ML models, largest)

Per module: port logic, test against old output, switch client, deploy, monitor. Delete old client package after each.

### Phase 6: Cleanup

Delete ml-sidecars/, all old client packages, mltransport/. Update CLAUDE.md, ARCHITECTURE.md, classifier CLAUDE.md.

### Phase 7: Add Drill Extractor

First module built natively in the framework. Validates the "add a new module" workflow.

### Phase Dependencies

| Phase | Depends On | Can Parallelize With |
|---|---|---|
| 1: Framework | None | nothing |
| 2: Crime proof | Phase 1 | nothing |
| 3: Go client | Phase 1 | Phase 2 |
| 4: Crime deploy | Phases 2 + 3 | nothing |
| 5: Remaining modules | Phase 4 | Each module independent |
| 6: Cleanup | Phase 5 | nothing |
| 7: Drill extractor | Phase 6 (or Phase 4) | Phase 5 |
