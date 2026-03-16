# Unified ML Sidecar Framework Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace 5 separate ML sidecars and 5 duplicated Go clients with a unified Python framework package and a single Go client with retry, timeout, and circuit breaker.

**Architecture:** Monorepo shared library (`ml-framework/`) provides base FastAPI app, schemas, ABCs, logging, metrics. Each domain (`ml-modules/{name}/`) implements `ClassifierModule` or `ExtractorModule`. Single `Dockerfile.ml-sidecar` with `MODULE_NAME` build arg. Unified `classifier/internal/mlclient/` replaces all existing client packages.

**Tech Stack:** Python 3.12, FastAPI, Pydantic v2, structlog, prometheus_client, uvicorn, Go 1.26+

**Spec:** `docs/superpowers/specs/2026-03-16-unified-ml-sidecar-framework-design.md`

---

## Chunk 1: Python Framework Foundation (Phase 1)

### File Map

```
CREATE: ml-framework/pyproject.toml
CREATE: ml-framework/nc_ml/__init__.py
CREATE: ml-framework/nc_ml/schemas.py
CREATE: ml-framework/nc_ml/module.py
CREATE: ml-framework/nc_ml/logging.py
CREATE: ml-framework/nc_ml/metrics.py
CREATE: ml-framework/nc_ml/middleware.py
CREATE: ml-framework/nc_ml/health.py
CREATE: ml-framework/nc_ml/model_loader.py
CREATE: ml-framework/nc_ml/app.py
CREATE: ml-framework/nc_ml/main.py
CREATE: ml-framework/tests/__init__.py
CREATE: ml-framework/tests/test_schemas.py
CREATE: ml-framework/tests/test_module.py
CREATE: ml-framework/tests/test_middleware.py
CREATE: ml-framework/tests/test_health.py
CREATE: ml-framework/tests/test_model_loader.py
CREATE: ml-framework/tests/test_app.py
CREATE: ml-framework/tests/test_logging.py
CREATE: ml-framework/tests/test_metrics.py
CREATE: docker/Dockerfile.ml-sidecar
```

---

### Task 1: Project Scaffolding

**Files:**
- Create: `ml-framework/pyproject.toml`
- Create: `ml-framework/nc_ml/__init__.py`
- Create: `ml-framework/tests/__init__.py`

- [ ] **Step 1: Create pyproject.toml**

```toml
[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[project]
name = "nc-ml"
version = "1.0.0"
description = "North Cloud unified ML sidecar framework"
requires-python = ">=3.12"
dependencies = [
    "fastapi>=0.115.0",
    "uvicorn[standard]>=0.34.0",
    "pydantic>=2.0.0",
    "structlog>=24.0.0",
    "prometheus-client>=0.21.0",
]

[project.optional-dependencies]
test = [
    "pytest>=8.0.0",
    "pytest-asyncio>=0.24.0",
    "httpx>=0.27.0",
]

[tool.pytest.ini_options]
asyncio_mode = "auto"
```

- [ ] **Step 2: Create nc_ml/__init__.py**

```python
"""North Cloud unified ML sidecar framework."""

__version__ = "1.0.0"
```

- [ ] **Step 3: Create empty tests/__init__.py**

Empty file.

- [ ] **Step 4: Verify package installs**

Run: `cd ml-framework && pip install -e ".[test]" && python -c "import nc_ml; print(nc_ml.__version__)"`
Expected: `1.0.0`

- [ ] **Step 5: Commit**

```bash
git add ml-framework/pyproject.toml ml-framework/nc_ml/__init__.py ml-framework/tests/__init__.py
git commit -m "feat(ml-framework): scaffold nc-ml Python package"
```

---

### Task 2: Standard Schemas

**Files:**
- Create: `ml-framework/nc_ml/schemas.py`
- Create: `ml-framework/tests/test_schemas.py`

- [ ] **Step 1: Write failing schema tests**

```python
# ml-framework/tests/test_schemas.py
import pytest
from datetime import datetime, timezone
from pydantic import ValidationError


def test_classify_request_valid():
    from nc_ml.schemas import ClassifyRequest

    req = ClassifyRequest(title="Test Title", body="Test body content")
    assert req.title == "Test Title"
    assert req.body == "Test body content"
    assert req.metadata is None


def test_classify_request_with_metadata():
    from nc_ml.schemas import ClassifyRequest

    req = ClassifyRequest(
        title="Test", body="Body", metadata={"source_id": "abc123"}
    )
    assert req.metadata == {"source_id": "abc123"}


def test_classify_request_missing_required():
    from nc_ml.schemas import ClassifyRequest

    with pytest.raises(ValidationError):
        ClassifyRequest(title="Test")  # missing body


def test_module_result_forbids_extra():
    from nc_ml.schemas import ModuleResult

    with pytest.raises(ValidationError):
        ModuleResult(unexpected_field="value")


def test_standard_response_classifier():
    from nc_ml.schemas import StandardResponse, ModuleResult

    resp = StandardResponse(
        module="crime",
        version="2025-02-01-crime-v2",
        schema_version="1.0",
        result=ModuleResult(),
        relevance=0.91,
        confidence=0.87,
        processing_time_ms=12.4,
        request_id="req-abc123",
    )
    assert resp.module == "crime"
    assert resp.relevance == 0.91


def test_standard_response_extractor_null_relevance():
    from nc_ml.schemas import StandardResponse, ModuleResult

    resp = StandardResponse(
        module="drill",
        version="2026-03-01-drill-v1",
        schema_version="1.0",
        result=ModuleResult(),
        relevance=None,
        confidence=None,
        processing_time_ms=45.2,
        request_id="req-def456",
    )
    assert resp.relevance is None
    assert resp.confidence is None


def test_standard_error():
    from nc_ml.schemas import StandardError

    err = StandardError(
        error="prediction_error",
        message="Model failed to classify",
        module="crime",
        request_id="req-abc123",
        timestamp=datetime.now(timezone.utc),
    )
    assert err.error == "prediction_error"


def test_health_response_healthy():
    from nc_ml.schemas import HealthResponse

    health = HealthResponse(
        status="healthy",
        module="crime",
        version="2025-02-01-crime-v2",
        schema_version="1.0",
        models_loaded=True,
        uptime_seconds=120.5,
    )
    assert health.status == "healthy"
    assert health.checks is None


def test_health_response_degraded_with_checks():
    from nc_ml.schemas import HealthResponse

    health = HealthResponse(
        status="degraded",
        module="mining",
        version="2025-01-01-mining-v1",
        schema_version="1.0",
        models_loaded=True,
        uptime_seconds=60.0,
        checks={"relevance_model": True, "stage_model": False},
    )
    assert health.status == "degraded"
    assert health.checks["stage_model"] is False


def test_health_response_invalid_status():
    from nc_ml.schemas import HealthResponse

    with pytest.raises(ValidationError):
        HealthResponse(
            status="unknown",
            module="crime",
            version="v1",
            schema_version="1.0",
            models_loaded=True,
            uptime_seconds=0,
        )
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ml-framework && python -m pytest tests/test_schemas.py -v`
Expected: FAIL (ModuleNotFoundError for nc_ml.schemas)

- [ ] **Step 3: Implement schemas**

```python
# ml-framework/nc_ml/schemas.py
"""Standard request/response/error/health schemas for all NC ML modules."""

from datetime import datetime
from typing import Any, Literal

from pydantic import BaseModel, ConfigDict


class ClassifyRequest(BaseModel):
    """Standard input for all modules - classifiers and extractors."""

    title: str
    body: str
    metadata: dict[str, Any] | None = None


class ModuleResult(BaseModel):
    """Base class for module results. Each module defines its own subclass."""

    model_config = ConfigDict(extra="forbid")


class ClassifierResult(ModuleResult):
    """Base for classifier outputs. Includes relevance and confidence."""

    relevance: float
    confidence: float


class ExtractorResult(ModuleResult):
    """Base for extractor outputs. No relevance or confidence."""

    pass


class StandardResponse(BaseModel):
    """Standard response envelope for all modules."""

    module: str
    version: str
    schema_version: str
    result: ModuleResult
    relevance: float | None = None
    confidence: float | None = None
    processing_time_ms: float
    request_id: str


class StandardError(BaseModel):
    """Standard error response."""

    error: str
    message: str
    module: str
    request_id: str
    timestamp: datetime


class HealthResponse(BaseModel):
    """Standard health check response."""

    status: Literal["healthy", "degraded", "unhealthy"]
    module: str
    version: str
    schema_version: str
    models_loaded: bool
    uptime_seconds: float
    checks: dict[str, bool] | None = None
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ml-framework && python -m pytest tests/test_schemas.py -v`
Expected: All 10 tests PASS

- [ ] **Step 5: Commit**

```bash
git add ml-framework/nc_ml/schemas.py ml-framework/tests/test_schemas.py
git commit -m "feat(ml-framework): add standard schemas (request, response, error, health)"
```

---

### Task 3: Module ABCs

**Files:**
- Create: `ml-framework/nc_ml/module.py`
- Create: `ml-framework/tests/test_module.py`

- [ ] **Step 1: Write failing module interface tests**

```python
# ml-framework/tests/test_module.py
import pytest
from nc_ml.schemas import ClassifyRequest, ClassifierResult, ExtractorResult, ModuleResult


def test_base_module_is_abstract():
    from nc_ml.module import BaseModule

    with pytest.raises(TypeError):
        BaseModule()


def test_classifier_module_is_abstract():
    from nc_ml.module import ClassifierModule

    with pytest.raises(TypeError):
        ClassifierModule()


def test_extractor_module_is_abstract():
    from nc_ml.module import ExtractorModule

    with pytest.raises(TypeError):
        ExtractorModule()


class FakeClassifierResult(ClassifierResult):
    categories: list[str] = []


class FakeExtractorResult(ExtractorResult):
    items: list[str] = []


async def test_classifier_module_concrete():
    from nc_ml.module import ClassifierModule

    class FakeClassifier(ClassifierModule):
        def name(self) -> str:
            return "fake"

        def version(self) -> str:
            return "2026-01-01-fake-v1"

        def schema_version(self) -> str:
            return "1.0"

        async def initialize(self) -> None:
            pass

        async def shutdown(self) -> None:
            pass

        async def health_checks(self) -> dict[str, bool]:
            return {}

        async def classify(self, request: ClassifyRequest) -> FakeClassifierResult:
            return FakeClassifierResult(
                relevance=0.9, confidence=0.8, categories=["test"]
            )

    module = FakeClassifier()
    assert module.name() == "fake"

    req = ClassifyRequest(title="Test", body="Body")
    result = await module.classify(req)
    assert result.relevance == 0.9
    assert result.categories == ["test"]


async def test_extractor_module_concrete():
    from nc_ml.module import ExtractorModule

    class FakeExtractor(ExtractorModule):
        def name(self) -> str:
            return "drill"

        def version(self) -> str:
            return "2026-01-01-drill-v1"

        def schema_version(self) -> str:
            return "1.0"

        async def initialize(self) -> None:
            pass

        async def shutdown(self) -> None:
            pass

        async def health_checks(self) -> dict[str, bool]:
            return {}

        async def extract(self, request: ClassifyRequest) -> FakeExtractorResult:
            return FakeExtractorResult(items=["item1"])

    module = FakeExtractor()
    req = ClassifyRequest(title="Test", body="Body")
    result = await module.extract(req)
    assert result.items == ["item1"]
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ml-framework && python -m pytest tests/test_module.py -v`
Expected: FAIL (cannot import module)

- [ ] **Step 3: Implement module ABCs**

```python
# ml-framework/nc_ml/module.py
"""Abstract base classes for ML sidecar modules."""

from abc import ABC, abstractmethod

from nc_ml.schemas import ClassifyRequest, ClassifierResult, ExtractorResult


class BaseModule(ABC):
    """Shared lifecycle for all modules."""

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
        """Granular health checks. Return empty dict for rule-based modules."""


class ClassifierModule(BaseModule):
    """For modules that return categorical classifications."""

    @abstractmethod
    async def classify(self, request: ClassifyRequest) -> ClassifierResult:
        """Run classification."""


class ExtractorModule(BaseModule):
    """For modules that return structured extracted data."""

    @abstractmethod
    async def extract(self, request: ClassifyRequest) -> ExtractorResult:
        """Run extraction."""
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ml-framework && python -m pytest tests/test_module.py -v`
Expected: All 5 tests PASS

- [ ] **Step 5: Commit**

```bash
git add ml-framework/nc_ml/module.py ml-framework/tests/test_module.py
git commit -m "feat(ml-framework): add BaseModule, ClassifierModule, ExtractorModule ABCs"
```

---

### Task 4: Structured Logging

**Files:**
- Create: `ml-framework/nc_ml/logging.py`

- [ ] **Step 1: Implement structured logger**

```python
# ml-framework/nc_ml/logging.py
"""Structured JSON logging for all NC ML modules."""

import structlog


def configure_logging(service_name: str, log_level: str = "info") -> None:
    """Configure structlog for JSON output to stdout."""
    structlog.configure(
        processors=[
            structlog.contextvars.merge_contextvars,
            structlog.processors.add_log_level,
            structlog.processors.TimeStamper(fmt="iso", utc=True, key="timestamp"),
            structlog.processors.StackInfoRenderer(),
            structlog.processors.format_exc_info,
            structlog.processors.JSONRenderer(),
        ],
        wrapper_class=structlog.make_filtering_bound_logger(
            structlog.get_level_from_name(log_level)
        ),
        context_class=dict,
        logger_factory=structlog.PrintLoggerFactory(),
        cache_logger_on_first_use=True,
    )


def get_logger(module: str, **kwargs: object) -> structlog.stdlib.BoundLogger:
    """Get a logger pre-bound with module context."""
    return structlog.get_logger(module=module, **kwargs)
```

- [ ] **Step 2: Verify logger works**

Run: `cd ml-framework && python -c "from nc_ml.logging import configure_logging, get_logger; configure_logging('nc-ml-test'); log = get_logger('test'); log.info('hello', key='value')"`
Expected: JSON line with timestamp, level, module, message, key

- [ ] **Step 3: Commit**

```bash
git add ml-framework/nc_ml/logging.py
git commit -m "feat(ml-framework): add structured JSON logging via structlog"
```

---

### Task 5: Prometheus Metrics

**Files:**
- Create: `ml-framework/nc_ml/metrics.py`

- [ ] **Step 1: Implement metrics registry**

```python
# ml-framework/nc_ml/metrics.py
"""Prometheus metrics for all NC ML modules."""

from prometheus_client import Counter, Gauge, Histogram

NCML_PREFIX = "ncml_"

requests_total = Counter(
    f"{NCML_PREFIX}requests_total",
    "Total requests",
    ["module", "status"],
)

request_duration_seconds = Histogram(
    f"{NCML_PREFIX}request_duration_seconds",
    "End-to-end request latency",
    ["module"],
    buckets=(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0),
)

prediction_duration_seconds = Histogram(
    f"{NCML_PREFIX}prediction_duration_seconds",
    "Module predict/extract time only",
    ["module"],
    buckets=(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0),
)

errors_total = Counter(
    f"{NCML_PREFIX}errors_total",
    "Errors by type",
    ["module", "error_code"],
)

model_info = Gauge(
    f"{NCML_PREFIX}model_info",
    "Model version info (always 1)",
    ["module", "version", "schema_version"],
)

health_status = Gauge(
    f"{NCML_PREFIX}health_status",
    "Health status (1 for current, 0 for others)",
    ["module", "status"],
)


def set_model_info(module: str, version: str, schema_version: str) -> None:
    """Set the model info gauge for a module."""
    model_info.labels(module=module, version=version, schema_version=schema_version).set(1)


def set_health(module: str, status: str) -> None:
    """Set health gauge. Sets current status to 1, others to 0."""
    for s in ("healthy", "degraded", "unhealthy"):
        health_status.labels(module=module, status=s).set(1 if s == status else 0)
```

- [ ] **Step 2: Write logging tests**

```python
# ml-framework/tests/test_logging.py
import json
import io
from unittest.mock import patch


def test_configure_logging_does_not_raise():
    from nc_ml.logging import configure_logging
    configure_logging("nc-ml-test")


def test_get_logger_returns_bound_logger():
    from nc_ml.logging import configure_logging, get_logger
    configure_logging("nc-ml-test")
    log = get_logger("test")
    assert log is not None


def test_logger_outputs_json(capsys):
    from nc_ml.logging import configure_logging, get_logger
    configure_logging("nc-ml-test")
    log = get_logger("test")
    log.info("hello", key="value")
    output = capsys.readouterr().out
    parsed = json.loads(output.strip())
    assert parsed["module"] == "test"
    assert parsed["key"] == "value"
    assert "timestamp" in parsed
```

- [ ] **Step 3: Run logging tests**

Run: `cd ml-framework && python -m pytest tests/test_logging.py -v`
Expected: All 3 tests PASS

- [ ] **Step 4: Write metrics tests**

```python
# ml-framework/tests/test_metrics.py
from nc_ml.metrics import (
    requests_total,
    set_model_info,
    set_health,
    health_status,
)


def test_set_model_info_sets_gauge():
    set_model_info("test", "v1", "1.0")
    # No exception means labels were created successfully


def test_set_health_sets_current_to_one():
    set_health("test", "healthy")
    assert health_status.labels(module="test", status="healthy")._value.get() == 1.0
    assert health_status.labels(module="test", status="degraded")._value.get() == 0.0
    assert health_status.labels(module="test", status="unhealthy")._value.get() == 0.0


def test_set_health_toggles():
    set_health("test2", "degraded")
    assert health_status.labels(module="test2", status="degraded")._value.get() == 1.0
    assert health_status.labels(module="test2", status="healthy")._value.get() == 0.0


def test_requests_counter_increments():
    before = requests_total.labels(module="test", status="success")._value.get()
    requests_total.labels(module="test", status="success").inc()
    after = requests_total.labels(module="test", status="success")._value.get()
    assert after == before + 1
```

- [ ] **Step 5: Run metrics tests**

Run: `cd ml-framework && python -m pytest tests/test_metrics.py -v`
Expected: All 4 tests PASS

- [ ] **Step 6: Commit**

```bash
git add ml-framework/nc_ml/logging.py ml-framework/nc_ml/metrics.py ml-framework/tests/test_logging.py ml-framework/tests/test_metrics.py
git commit -m "feat(ml-framework): add structured logging and Prometheus metrics with tests"
```

---

### Task 6: Middleware (request_id, timing, error handling)

**Files:**
- Create: `ml-framework/nc_ml/middleware.py`
- Create: `ml-framework/tests/test_middleware.py`

- [ ] **Step 1: Write failing middleware tests**

```python
# ml-framework/tests/test_middleware.py
import pytest
from datetime import datetime, timezone
from fastapi import FastAPI
from fastapi.testclient import TestClient

from nc_ml.middleware import add_middleware
from nc_ml.schemas import StandardError


def _make_app() -> FastAPI:
    app = FastAPI()
    add_middleware(app, module_name="test")

    @app.get("/ok")
    async def ok():
        return {"status": "ok"}

    @app.get("/fail")
    async def fail():
        raise ValueError("something broke")

    return app


def test_request_id_injected():
    client = TestClient(_make_app())
    resp = client.get("/ok")
    assert resp.status_code == 200
    assert "x-request-id" in resp.headers


def test_request_id_propagated():
    client = TestClient(_make_app())
    resp = client.get("/ok", headers={"x-request-id": "custom-123"})
    assert resp.headers["x-request-id"] == "custom-123"


def test_unhandled_exception_wrapped():
    client = TestClient(_make_app(), raise_server_exceptions=False)
    resp = client.get("/fail")
    assert resp.status_code == 500
    body = resp.json()
    assert body["error"] == "internal_error"
    assert body["module"] == "test"
    assert "request_id" in body
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ml-framework && python -m pytest tests/test_middleware.py -v`
Expected: FAIL

- [ ] **Step 3: Implement middleware**

```python
# ml-framework/nc_ml/middleware.py
"""Request middleware: request_id injection, timing, error handling."""

import time
import uuid
from datetime import datetime, timezone

import structlog
from fastapi import FastAPI, Request, Response
from starlette.middleware.base import BaseHTTPMiddleware, RequestResponseEndpoint

from nc_ml.schemas import StandardError


class RequestMiddleware(BaseHTTPMiddleware):
    def __init__(self, app: FastAPI, module_name: str) -> None:
        super().__init__(app)
        self.module_name = module_name
        self.logger = structlog.get_logger(module=module_name)

    async def dispatch(
        self, request: Request, call_next: RequestResponseEndpoint
    ) -> Response:
        request_id = request.headers.get("x-request-id", str(uuid.uuid4()))
        request.state.request_id = request_id
        start = time.monotonic()

        try:
            response = await call_next(request)
            duration_ms = (time.monotonic() - start) * 1000
            response.headers["x-request-id"] = request_id
            response.headers["x-processing-time-ms"] = f"{duration_ms:.2f}"
            return response
        except Exception:
            duration_ms = (time.monotonic() - start) * 1000
            self.logger.error(
                "unhandled exception",
                request_id=request_id,
                duration_ms=duration_ms,
                exc_info=True,
            )
            error = StandardError(
                error="internal_error",
                message="An internal error occurred",
                module=self.module_name,
                request_id=request_id,
                timestamp=datetime.now(timezone.utc),
            )
            from fastapi.responses import JSONResponse

            return JSONResponse(
                status_code=500,
                content=error.model_dump(mode="json"),
                headers={"x-request-id": request_id},
            )


def add_middleware(app: FastAPI, module_name: str) -> None:
    """Add standard middleware to a FastAPI app."""
    app.add_middleware(RequestMiddleware, module_name=module_name)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ml-framework && python -m pytest tests/test_middleware.py -v`
Expected: All 3 tests PASS

- [ ] **Step 5: Commit**

```bash
git add ml-framework/nc_ml/middleware.py ml-framework/tests/test_middleware.py
git commit -m "feat(ml-framework): add request middleware (request_id, timing, error wrapping)"
```

---

### Task 7: Health Endpoint Logic

**Files:**
- Create: `ml-framework/nc_ml/health.py`
- Create: `ml-framework/tests/test_health.py`

- [ ] **Step 1: Write failing health tests**

```python
# ml-framework/tests/test_health.py
from nc_ml.health import aggregate_health_status


def test_all_true_is_healthy():
    assert aggregate_health_status({"a": True, "b": True}) == "healthy"


def test_some_false_is_degraded():
    assert aggregate_health_status({"a": True, "b": False}) == "degraded"


def test_all_false_is_unhealthy():
    assert aggregate_health_status({"a": False, "b": False}) == "unhealthy"


def test_empty_checks_is_healthy():
    assert aggregate_health_status({}) == "healthy"
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ml-framework && python -m pytest tests/test_health.py -v`
Expected: FAIL

- [ ] **Step 3: Implement health logic**

```python
# ml-framework/nc_ml/health.py
"""Health check aggregation logic."""

from typing import Literal


def aggregate_health_status(
    checks: dict[str, bool],
) -> Literal["healthy", "degraded", "unhealthy"]:
    """Aggregate granular checks into a three-state status.

    - All True (or empty) -> healthy
    - Some False -> degraded
    - All False -> unhealthy
    """
    if not checks:
        return "healthy"
    values = list(checks.values())
    if all(values):
        return "healthy"
    if any(values):
        return "degraded"
    return "unhealthy"
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ml-framework && python -m pytest tests/test_health.py -v`
Expected: All 4 tests PASS

- [ ] **Step 5: Commit**

```bash
git add ml-framework/nc_ml/health.py ml-framework/tests/test_health.py
git commit -m "feat(ml-framework): add health check aggregation logic"
```

---

### Task 8: Model Loader

**Files:**
- Create: `ml-framework/nc_ml/model_loader.py`
- Create: `ml-framework/tests/test_model_loader.py`

- [ ] **Step 1: Write failing model loader tests**

```python
# ml-framework/tests/test_model_loader.py
import pytest
import tempfile
import os
from pathlib import Path


def test_load_model_success(tmp_path):
    """Test loading a valid model file."""
    from nc_ml.model_loader import load_model

    # Create a fake joblib file (just needs to be loadable)
    import joblib

    model = {"type": "fake", "version": 1}
    model_path = tmp_path / "test_model.joblib"
    joblib.dump(model, model_path)

    loaded = load_model(model_path)
    assert loaded == model


def test_load_model_missing_file():
    from nc_ml.model_loader import load_model, ModelLoadError

    with pytest.raises(ModelLoadError, match="not found"):
        load_model(Path("/nonexistent/model.joblib"))


def test_load_models_dir(tmp_path):
    """Test loading all models from a directory."""
    from nc_ml.model_loader import load_models_from_dir

    import joblib

    joblib.dump({"name": "a"}, tmp_path / "model_a.joblib")
    joblib.dump({"name": "b"}, tmp_path / "model_b.joblib")

    models = load_models_from_dir(tmp_path)
    assert "model_a" in models
    assert "model_b" in models
    assert models["model_a"] == {"name": "a"}


def test_load_models_dir_empty(tmp_path):
    from nc_ml.model_loader import load_models_from_dir

    models = load_models_from_dir(tmp_path)
    assert models == {}
```

- [ ] **Step 2: Add joblib to optional dependencies in pyproject.toml**

Update `pyproject.toml` to add:
```toml
[project.optional-dependencies]
ml = ["joblib>=1.4.0", "scikit-learn>=1.5.0"]
test = [
    "pytest>=8.0.0",
    "pytest-asyncio>=0.24.0",
    "httpx>=0.27.0",
    "joblib>=1.4.0",
]
```

Then reinstall: `cd ml-framework && pip install -e ".[test]"`

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd ml-framework && python -m pytest tests/test_model_loader.py -v`
Expected: FAIL (ModuleNotFoundError for nc_ml.model_loader)

- [ ] **Step 4: Implement model loader**

```python
# ml-framework/nc_ml/model_loader.py
"""Model loading utilities for ML modules."""

from pathlib import Path
from typing import Any

import structlog

logger = structlog.get_logger()


class ModelLoadError(Exception):
    """Raised when a model file cannot be loaded."""


def load_model(path: Path) -> Any:
    """Load a single model file (joblib format)."""
    if not path.exists():
        raise ModelLoadError(f"Model file not found: {path}")
    try:
        import joblib

        model = joblib.load(path)
        logger.info("model loaded", path=str(path))
        return model
    except Exception as e:
        raise ModelLoadError(f"Failed to load model {path}: {e}") from e


def load_models_from_dir(directory: Path) -> dict[str, Any]:
    """Load all .joblib files from a directory. Returns {stem: model}."""
    models: dict[str, Any] = {}
    if not directory.exists():
        return models
    for path in sorted(directory.glob("*.joblib")):
        models[path.stem] = load_model(path)
    return models
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd ml-framework && python -m pytest tests/test_model_loader.py -v`
Expected: All 4 tests PASS

- [ ] **Step 6: Commit**

```bash
git add ml-framework/nc_ml/model_loader.py ml-framework/tests/test_model_loader.py ml-framework/pyproject.toml
git commit -m "feat(ml-framework): add model loader with joblib support"
```

---

### Task 9: App Factory

**Files:**
- Create: `ml-framework/nc_ml/app.py`
- Create: `ml-framework/nc_ml/main.py`
- Create: `ml-framework/tests/test_app.py`

- [ ] **Step 1: Write failing app factory tests**

```python
# ml-framework/tests/test_app.py
import pytest
from fastapi.testclient import TestClient

from nc_ml.schemas import ClassifyRequest, ClassifierResult, ExtractorResult
from nc_ml.module import ClassifierModule, ExtractorModule


class StubResult(ClassifierResult):
    categories: list[str] = []


class StubClassifier(ClassifierModule):
    def name(self) -> str:
        return "stub"

    def version(self) -> str:
        return "2026-01-01-stub-v1"

    def schema_version(self) -> str:
        return "1.0"

    async def initialize(self) -> None:
        pass

    async def shutdown(self) -> None:
        pass

    async def health_checks(self) -> dict[str, bool]:
        return {"model": True}

    async def classify(self, request: ClassifyRequest) -> StubResult:
        return StubResult(relevance=0.9, confidence=0.8, categories=["test"])


class StubExtractResult(ExtractorResult):
    items: list[str] = []


class StubExtractor(ExtractorModule):
    def name(self) -> str:
        return "extractor"

    def version(self) -> str:
        return "2026-01-01-extractor-v1"

    def schema_version(self) -> str:
        return "1.0"

    async def initialize(self) -> None:
        pass

    async def shutdown(self) -> None:
        pass

    async def health_checks(self) -> dict[str, bool]:
        return {}

    async def extract(self, request: ClassifyRequest) -> StubExtractResult:
        return StubExtractResult(items=["a", "b"])


def test_classify_endpoint():
    from nc_ml.app import create_app

    app = create_app(StubClassifier())
    client = TestClient(app)
    resp = client.post("/v1/classify", json={"title": "Test", "body": "Body"})
    assert resp.status_code == 200
    body = resp.json()
    assert body["module"] == "stub"
    assert body["version"] == "2026-01-01-stub-v1"
    assert body["schema_version"] == "1.0"
    assert body["relevance"] == 0.9
    assert body["confidence"] == 0.8
    assert body["result"]["categories"] == ["test"]
    assert "request_id" in body
    assert "processing_time_ms" in body


def test_extract_endpoint():
    from nc_ml.app import create_app

    app = create_app(StubExtractor())
    client = TestClient(app)
    resp = client.post("/v1/classify", json={"title": "Test", "body": "Body"})
    assert resp.status_code == 200
    body = resp.json()
    assert body["module"] == "extractor"
    assert body["relevance"] is None
    assert body["confidence"] is None
    assert body["result"]["items"] == ["a", "b"]


def test_health_endpoint():
    from nc_ml.app import create_app

    app = create_app(StubClassifier())
    client = TestClient(app)
    resp = client.get("/v1/health")
    assert resp.status_code == 200
    body = resp.json()
    assert body["status"] == "healthy"
    assert body["module"] == "stub"
    assert body["models_loaded"] is True
    assert body["checks"] == {"model": True}


def test_metrics_endpoint():
    from nc_ml.app import create_app

    app = create_app(StubClassifier())
    client = TestClient(app)
    resp = client.get("/metrics")
    assert resp.status_code == 200
    assert "ncml_" in resp.text


def test_classify_validation_error():
    from nc_ml.app import create_app

    app = create_app(StubClassifier())
    client = TestClient(app)
    resp = client.post("/v1/classify", json={"title": "Test"})  # missing body
    assert resp.status_code == 422


def test_health_after_failed_init():
    from nc_ml.app import create_app

    class FailingModule(StubClassifier):
        async def initialize(self) -> None:
            raise RuntimeError("model load failed")

    app = create_app(FailingModule())
    client = TestClient(app, raise_server_exceptions=False)
    resp = client.get("/v1/health")
    assert resp.status_code == 200
    body = resp.json()
    assert body["status"] == "unhealthy"
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ml-framework && python -m pytest tests/test_app.py -v`
Expected: FAIL

- [ ] **Step 3: Implement app factory**

```python
# ml-framework/nc_ml/app.py
"""FastAPI app factory for NC ML modules."""

import time
from contextlib import asynccontextmanager
from datetime import datetime, timezone

import structlog
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
from prometheus_client import generate_latest, CONTENT_TYPE_LATEST
from starlette.responses import Response

from nc_ml.health import aggregate_health_status
from nc_ml.logging import configure_logging, get_logger
from nc_ml.metrics import (
    prediction_duration_seconds,
    request_duration_seconds,
    requests_total,
    errors_total,
    set_health,
    set_model_info,
)
from nc_ml.middleware import add_middleware
from nc_ml.module import BaseModule, ClassifierModule, ExtractorModule
from nc_ml.schemas import (
    ClassifyRequest,
    HealthResponse,
    StandardError,
    StandardResponse,
)


def create_app(module: BaseModule) -> FastAPI:
    """Create a FastAPI app wired to the given module."""
    module_name = module.name()
    service_name = f"nc-ml-{module_name}"
    start_time = time.monotonic()
    init_failed = False

    configure_logging(service_name)
    logger = get_logger(module_name)

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        nonlocal init_failed
        try:
            await module.initialize()
            set_model_info(module_name, module.version(), module.schema_version())
            set_health(module_name, "healthy")
            logger.info(
                "module initialized",
                version=module.version(),
                schema_version=module.schema_version(),
            )
        except Exception:
            init_failed = True
            set_health(module_name, "unhealthy")
            logger.error("module initialization failed", exc_info=True)
        yield
        await module.shutdown()
        logger.info("module shut down")

    app = FastAPI(title=service_name, lifespan=lifespan)
    add_middleware(app, module_name)

    @app.post("/v1/classify", response_model=StandardResponse)
    async def classify_endpoint(request_body: ClassifyRequest, request: Request):
        request_id = getattr(request.state, "request_id", "unknown")

        if init_failed:
            error = StandardError(
                error="module_unavailable",
                message="Module failed to initialize",
                module=module_name,
                request_id=request_id,
                timestamp=datetime.now(timezone.utc),
            )
            return JSONResponse(status_code=503, content=error.model_dump(mode="json"))

        predict_start = time.monotonic()
        try:
            if isinstance(module, ClassifierModule):
                result = await module.classify(request_body)
                relevance = result.relevance
                confidence = result.confidence
            elif isinstance(module, ExtractorModule):
                result = await module.extract(request_body)
                relevance = None
                confidence = None
            else:
                raise TypeError(f"Unknown module type: {type(module)}")

            predict_duration = time.monotonic() - predict_start
            total_duration = time.monotonic() - (
                request.state.request_start
                if hasattr(request.state, "request_start")
                else predict_start
            )

            prediction_duration_seconds.labels(module=module_name).observe(predict_duration)
            requests_total.labels(module=module_name, status="success").inc()

            return StandardResponse(
                module=module_name,
                version=module.version(),
                schema_version=module.schema_version(),
                result=result,
                relevance=relevance,
                confidence=confidence,
                processing_time_ms=predict_duration * 1000,
                request_id=request_id,
            )
        except Exception as e:
            errors_total.labels(module=module_name, error_code="prediction_error").inc()
            requests_total.labels(module=module_name, status="error").inc()
            logger.error("classification failed", request_id=request_id, exc_info=True)
            error = StandardError(
                error="prediction_error",
                message=str(e),
                module=module_name,
                request_id=request_id,
                timestamp=datetime.now(timezone.utc),
            )
            return JSONResponse(status_code=500, content=error.model_dump(mode="json"))

    @app.get("/v1/health", response_model=HealthResponse)
    async def health_endpoint():
        if init_failed:
            set_health(module_name, "unhealthy")
            return HealthResponse(
                status="unhealthy",
                module=module_name,
                version=module.version(),
                schema_version=module.schema_version(),
                models_loaded=False,
                uptime_seconds=time.monotonic() - start_time,
            )

        checks = await module.health_checks()
        status = aggregate_health_status(checks)
        set_health(module_name, status)

        return HealthResponse(
            status=status,
            module=module_name,
            version=module.version(),
            schema_version=module.schema_version(),
            models_loaded=True,
            uptime_seconds=time.monotonic() - start_time,
            checks=checks if checks else None,
        )

    @app.get("/metrics")
    async def metrics_endpoint():
        return Response(
            content=generate_latest(),
            media_type=CONTENT_TYPE_LATEST,
        )

    return app
```

- [ ] **Step 4: Implement main.py entry point**

```python
# ml-framework/nc_ml/main.py
"""Dynamic module loader entry point for uvicorn."""

import importlib
import os
import sys

# Add /opt/module to sys.path so module imports work
sys.path.insert(0, "/opt/module")

module_name = os.environ.get("MODULE_NAME")
if not module_name:
    raise RuntimeError("MODULE_NAME environment variable is required")

mod = importlib.import_module("module")
from nc_ml.app import create_app

app = create_app(mod.Module())
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd ml-framework && python -m pytest tests/test_app.py -v`
Expected: All 6 tests PASS

- [ ] **Step 6: Commit**

```bash
git add ml-framework/nc_ml/app.py ml-framework/nc_ml/main.py ml-framework/tests/test_app.py
git commit -m "feat(ml-framework): add app factory, /v1/classify, /v1/health, /metrics endpoints"
```

---

### Task 10: Shared Dockerfile

**Files:**
- Create: `docker/Dockerfile.ml-sidecar`

- [ ] **Step 1: Create docker/ directory**

```bash
mkdir -p docker
```

- [ ] **Step 2: Create the Dockerfile**

```dockerfile
# docker/Dockerfile.ml-sidecar
# Unified ML sidecar Dockerfile - one template for all modules
ARG PYTHON_VERSION=3.12
FROM python:${PYTHON_VERSION}-slim AS base

# System deps needed by numpy/scipy/scikit-learn
RUN apt-get update && apt-get install -y --no-install-recommends libgomp1 \
    && rm -rf /var/lib/apt/lists/*

# Install the framework (cached independently from modules)
COPY ml-framework/ /opt/ml-framework/
RUN pip install --no-cache-dir /opt/ml-framework/

# Install module (changes per build)
ARG MODULE_NAME
RUN test -n "${MODULE_NAME}" || (echo "MODULE_NAME build arg is required" && exit 1)
COPY ml-modules/${MODULE_NAME}/ /opt/module/
RUN if [ -f /opt/module/requirements.txt ]; then \
        pip install --no-cache-dir -r /opt/module/requirements.txt; \
    fi

ENV MODULE_NAME=${MODULE_NAME}
ENV PORT=8080
ENV WORKERS=1
ENV LOG_LEVEL=info

WORKDIR /opt/module
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8080/v1/health')"

CMD ["sh", "-c", "python -m uvicorn nc_ml.main:app --host 0.0.0.0 --port ${PORT} --workers ${WORKERS} --log-level ${LOG_LEVEL}"]
```

- [ ] **Step 3: Commit**

```bash
git add docker/Dockerfile.ml-sidecar
git commit -m "feat(ml-framework): add shared Dockerfile template for all ML modules"
```

---

### Task 11: Run Full Framework Test Suite

- [ ] **Step 1: Run all framework tests**

Run: `cd ml-framework && python -m pytest tests/ -v --tb=short`
Expected: All tests PASS (schemas, module, middleware, health, model_loader, app, logging, metrics)

- [ ] **Step 2: Verify total test count**

Expected: ~27 tests across 6 test files

---

## Chunk 2: Crime Module Port & Unified Go Client (Phases 2-3)

### File Map

```
CREATE: ml-modules/crime/__init__.py
CREATE: ml-modules/crime/module.py
CREATE: ml-modules/crime/requirements.txt
COPY:   ml-modules/crime/models/ (from ml-sidecars/crime-ml/models/)
CREATE: ml-modules/crime/tests/__init__.py
CREATE: ml-modules/crime/tests/test_module.py
CREATE: classifier/internal/mlclient/client.go (new unified client)
CREATE: classifier/internal/mlclient/transport.go
CREATE: classifier/internal/mlclient/schemas.go
CREATE: classifier/internal/mlclient/errors.go
CREATE: classifier/internal/mlclient/options.go
CREATE: classifier/internal/mlclient/breaker.go
CREATE: classifier/internal/mlclient/client_test.go
CREATE: classifier/internal/mlclient/breaker_test.go
```

---

### Task 12: Port Crime Classification Logic

**Files:**
- Create: `ml-modules/crime/__init__.py`
- Create: `ml-modules/crime/module.py`
- Create: `ml-modules/crime/requirements.txt`
- Create: `ml-modules/crime/tests/__init__.py`
- Create: `ml-modules/crime/tests/test_module.py`
- Reference: `ml-sidecars/crime-ml/main.py`, `ml-sidecars/crime-ml/classifier/`

- [ ] **Step 1: Read existing crime-ml code**

Read these files to understand the classification logic:
- `ml-sidecars/crime-ml/main.py`
- `ml-sidecars/crime-ml/classifier/relevance.py`
- `ml-sidecars/crime-ml/classifier/crime_type.py`
- `ml-sidecars/crime-ml/classifier/location.py`
- `ml-sidecars/crime-ml/classifier/preprocessor.py`
- `ml-sidecars/crime-ml/requirements.txt`

- [ ] **Step 2: Create __init__.py and requirements.txt**

Create empty `ml-modules/crime/__init__.py` and `ml-modules/crime/tests/__init__.py`.

Create `ml-modules/crime/requirements.txt`:
```
scikit-learn>=1.5.0
joblib>=1.4.0
```

- [ ] **Step 3: Write failing crime module test**

```python
# ml-modules/crime/tests/test_module.py
import pytest
from unittest.mock import MagicMock, patch
from nc_ml.schemas import ClassifyRequest


async def test_crime_module_name():
    from module import Module

    m = Module()
    assert m.name() == "crime"


async def test_crime_module_version():
    from module import Module

    m = Module()
    assert "crime" in m.version()


async def test_crime_module_schema_version():
    from module import Module

    m = Module()
    assert m.schema_version() == "1.0"


async def test_crime_module_health_no_models():
    """Before initialize(), health should report models not loaded."""
    from module import Module

    m = Module()
    checks = await m.health_checks()
    assert not all(checks.values()) if checks else True


async def test_crime_classify_returns_result():
    """After initialization with models, classify should return CrimeResult."""
    from module import Module, CrimeResult

    m = Module()
    # Mock model loading for unit test (integration test uses real models)
    m._models_loaded = True
    m._relevance_model = MagicMock(
        predict_proba=MagicMock(return_value=[[0.1, 0.9]])
    )
    m._crime_type_model = MagicMock(
        predict_proba=MagicMock(return_value=[[0.8, 0.1, 0.1]])
    )
    m._location_model = MagicMock(
        predict_proba=MagicMock(return_value=[[0.3, 0.7]])
    )

    req = ClassifyRequest(title="Man arrested for assault", body="Police report details...")
    result = await m.classify(req)

    assert isinstance(result, CrimeResult)
    assert 0.0 <= result.relevance <= 1.0
    assert 0.0 <= result.confidence <= 1.0
    assert isinstance(result.crime_types, list)
    assert isinstance(result.crime_type_scores, dict)
    assert isinstance(result.location_detected, bool)
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `cd ml-modules/crime && python -m pytest tests/test_module.py -v`
Expected: FAIL

- [ ] **Step 5: Implement crime module**

Port the classification logic from `ml-sidecars/crime-ml/` into `ml-modules/crime/module.py`. The module implements `ClassifierModule` and uses the existing preprocessor, relevance, crime_type, and location classifiers. Strip all FastAPI boilerplate — the framework handles HTTP.

Key structure:
```python
# ml-modules/crime/module.py
from pathlib import Path
from nc_ml.module import ClassifierModule
from nc_ml.schemas import ClassifyRequest, ClassifierResult
from nc_ml.model_loader import load_model, ModelLoadError

class CrimeResult(ClassifierResult):
    crime_types: list[str]
    crime_type_scores: dict[str, float]
    location_detected: bool

class Module(ClassifierModule):
    # Port classification logic from ml-sidecars/crime-ml/classifier/
    # Load 3 joblib models in initialize()
    # Preprocess text, run models, return CrimeResult in classify()
```

The exact implementation must be ported from the existing crime-ml classifier files. Do not rewrite the ML logic — move it.

- [ ] **Step 6: Copy model files**

```bash
mkdir -p ml-modules/crime/models
cp ml-sidecars/crime-ml/models/*.joblib ml-modules/crime/models/ 2>/dev/null || echo "No model files to copy (dev environment)"
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `cd ml-modules/crime && python -m pytest tests/test_module.py -v`
Expected: All 5 tests PASS

- [ ] **Step 8: Commit**

```bash
git add ml-modules/crime/
git commit -m "feat(ml-modules): port crime classifier to unified framework"
```

---

### Task 13: Verify Crime Module via Docker

- [ ] **Step 1: Build crime-ml container with new Dockerfile**

```bash
docker build -f docker/Dockerfile.ml-sidecar --build-arg MODULE_NAME=crime -t nc-ml-crime:test .
```

Expected: Build succeeds

- [ ] **Step 2: Run container and test health**

```bash
docker run -d --name nc-ml-crime-test -p 18076:8080 nc-ml-crime:test
sleep 5
curl -s http://localhost:18076/v1/health | python -m json.tool
```

Expected: JSON response with `"status": "healthy"` (or `"unhealthy"` if no model files in dev)

- [ ] **Step 3: Test classify endpoint**

```bash
curl -s -X POST http://localhost:18076/v1/classify \
  -H "Content-Type: application/json" \
  -d '{"title": "Man arrested for robbery", "body": "Police arrested a suspect..."}' \
  | python -m json.tool
```

Expected: StandardResponse with `module: "crime"`, `result` containing crime_types, relevance, etc.

- [ ] **Step 4: Test metrics endpoint**

```bash
curl -s http://localhost:18076/metrics | grep ncml_
```

Expected: Prometheus metrics with `ncml_` prefix

- [ ] **Step 5: Clean up**

```bash
docker stop nc-ml-crime-test && docker rm nc-ml-crime-test
```

- [ ] **Step 6: Commit any fixes**

If any changes were needed, commit them.

---

### Task 14: Unified Go Client — Schemas and Errors

**Files:**
- Create: `classifier/internal/mlclient/schemas.go` (replaces old)
- Create: `classifier/internal/mlclient/errors.go`

**Important:** The new unified client will be built in a temporary directory first, then swapped in during Task 17 when the wiring is updated. This avoids breaking the build between tasks.

- [ ] **Step 1: Read existing mlclient to understand current interface**

Read: `classifier/internal/mlclient/client.go`
Read: `classifier/internal/mltransport/transport.go`

- [ ] **Step 2: Create unified client in a new directory**

```bash
mkdir -p classifier/internal/mlclientv2
```

All new files in Tasks 14-16 go into `classifier/internal/mlclientv2/` (package name `mlclientv2`). Task 17 renames it to `mlclient` and deletes the old one atomically.

- [ ] **Step 3: Create schemas.go**

```go
// classifier/internal/mlclientv2/schemas.go
package mlclientv2

import "encoding/json"

// StandardResponse mirrors the Python StandardResponse envelope.
type StandardResponse struct {
	Module           string          `json:"module"`
	Version          string          `json:"version"`
	SchemaVersion    string          `json:"schema_version"`
	Result           json.RawMessage `json:"result"`
	Relevance        *float64        `json:"relevance"`
	Confidence       *float64        `json:"confidence"`
	ProcessingTimeMs float64         `json:"processing_time_ms"`
	RequestID        string          `json:"request_id"`
}

// StandardError mirrors the Python StandardError envelope.
type StandardError struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	Module    string `json:"module"`
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
}

// HealthResponse mirrors the Python HealthResponse envelope.
type HealthResponse struct {
	Status        string          `json:"status"`
	Module        string          `json:"module"`
	Version       string          `json:"version"`
	SchemaVersion string          `json:"schema_version"`
	ModelsLoaded  bool            `json:"models_loaded"`
	UptimeSeconds float64         `json:"uptime_seconds"`
	Checks        map[string]bool `json:"checks,omitempty"`
}

// classifyRequest is the request body sent to ML modules.
type classifyRequest struct {
	Title    string         `json:"title"`
	Body     string         `json:"body"`
	Metadata map[string]any `json:"metadata,omitempty"`
}
```

- [ ] **Step 4: Create errors.go**

```go
// classifier/internal/mlclientv2/errors.go
package mlclientv2

import "errors"

var (
	// ErrUnavailable indicates the module is unreachable or circuit is open.
	ErrUnavailable = errors.New("ml module unavailable")

	// ErrUnhealthy indicates the module health check returned unhealthy.
	ErrUnhealthy = errors.New("ml module unhealthy")

	// ErrTimeout indicates the request timed out.
	ErrTimeout = errors.New("ml module request timed out")

	// ErrSchemaVersion indicates an unexpected schema version.
	ErrSchemaVersion = errors.New("ml module schema version mismatch")
)
```

- [ ] **Step 5: Commit**

```bash
git add classifier/internal/mlclientv2/schemas.go classifier/internal/mlclientv2/errors.go
git commit -m "feat(mlclientv2): add unified schemas and typed errors"
```

---

### Task 15: Unified Go Client — Options and Circuit Breaker

**Files:**
- Create: `classifier/internal/mlclientv2/options.go`
- Create: `classifier/internal/mlclientv2/breaker.go`
- Create: `classifier/internal/mlclientv2/breaker_test.go`

- [ ] **Step 1: Create options.go**

```go
// classifier/internal/mlclientv2/options.go
package mlclientv2

import "time"

type clientOptions struct {
	timeout         time.Duration
	retryCount      int
	retryBaseDelay  time.Duration
	breakerTrips    int
	breakerCooldown time.Duration
}

func defaultOptions() clientOptions {
	return clientOptions{
		timeout:         5 * time.Second,
		retryCount:      1,
		retryBaseDelay:  100 * time.Millisecond,
		breakerTrips:    5,
		breakerCooldown: 30 * time.Second,
	}
}

// Option configures the ML client.
type Option func(*clientOptions)

// WithTimeout sets the per-request timeout.
func WithTimeout(d time.Duration) Option {
	return func(o *clientOptions) { o.timeout = d }
}

// WithRetry sets retry count and base delay (exponential backoff with jitter).
func WithRetry(count int, baseDelay time.Duration) Option {
	return func(o *clientOptions) {
		o.retryCount = count
		o.retryBaseDelay = baseDelay
	}
}

// WithCircuitBreaker configures the circuit breaker.
func WithCircuitBreaker(trips int, cooldown time.Duration) Option {
	return func(o *clientOptions) {
		o.breakerTrips = trips
		o.breakerCooldown = cooldown
	}
}
```

- [ ] **Step 2: Write failing circuit breaker tests**

```go
// classifier/internal/mlclientv2/breaker_test.go
package mlclientv2

import (
	"testing"
	"time"
)

func TestBreakerStartsClosed(t *testing.T) {
	b := newBreaker(3, 100*time.Millisecond)
	if !b.allow() {
		t.Fatal("breaker should allow requests when closed")
	}
}

func TestBreakerOpensAfterTrips(t *testing.T) {
	b := newBreaker(3, 100*time.Millisecond)
	b.recordFailure()
	b.recordFailure()
	b.recordFailure()
	if b.allow() {
		t.Fatal("breaker should be open after 3 failures")
	}
}

func TestBreakerResetsOnSuccess(t *testing.T) {
	b := newBreaker(3, 100*time.Millisecond)
	b.recordFailure()
	b.recordFailure()
	b.recordSuccess()
	if !b.allow() {
		t.Fatal("breaker should be closed after success reset")
	}
}

func TestBreakerHalfOpenAfterCooldown(t *testing.T) {
	b := newBreaker(1, 50*time.Millisecond)
	b.recordFailure()
	if b.allow() {
		t.Fatal("breaker should be open immediately after trip")
	}
	time.Sleep(60 * time.Millisecond)
	if !b.allow() {
		t.Fatal("breaker should be half-open after cooldown")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd classifier && go test ./internal/mlclientv2/ -run TestBreaker -v`
Expected: FAIL (breaker.go does not exist)

- [ ] **Step 4: Implement circuit breaker**

```go
// classifier/internal/mlclientv2/breaker.go
package mlclientv2

import (
	"sync"
	"time"
)

type breakerState int

const (
	breakerClosed breakerState = iota
	breakerOpen
	breakerHalfOpen
)

type circuitBreaker struct {
	mu          sync.Mutex
	state       breakerState
	failures    int
	maxFailures int
	cooldown    time.Duration
	openedAt    time.Time
}

func newBreaker(maxFailures int, cooldown time.Duration) *circuitBreaker {
	return &circuitBreaker{
		state:       breakerClosed,
		maxFailures: maxFailures,
		cooldown:    cooldown,
	}
}

func (b *circuitBreaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case breakerClosed:
		return true
	case breakerOpen:
		if time.Since(b.openedAt) > b.cooldown {
			b.state = breakerHalfOpen
			return true
		}
		return false
	case breakerHalfOpen:
		return true
	default:
		return false
	}
}

func (b *circuitBreaker) recordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.state = breakerClosed
}

func (b *circuitBreaker) recordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.failures >= b.maxFailures {
		b.state = breakerOpen
		b.openedAt = time.Now()
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd classifier && go test ./internal/mlclientv2/ -run TestBreaker -v`
Expected: All 4 tests PASS

- [ ] **Step 6: Commit**

```bash
git add classifier/internal/mlclientv2/options.go classifier/internal/mlclientv2/breaker.go classifier/internal/mlclientv2/breaker_test.go
git commit -m "feat(mlclientv2): add functional options and circuit breaker"
```

---

### Task 16: Unified Go Client — Transport and Client

**Files:**
- Create: `classifier/internal/mlclientv2/transport.go`
- Create: `classifier/internal/mlclientv2/client.go`
- Create: `classifier/internal/mlclientv2/client_test.go`

- [ ] **Step 1: Write failing client tests**

```go
// classifier/internal/mlclientv2/client_test.go
package mlclientv2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func TestClassifySuccess(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := StandardResponse{
			Module:           "crime",
			Version:          "v1",
			SchemaVersion:    "1.0",
			Result:           json.RawMessage(`{"crime_types":["assault"]}`),
			Relevance:        float64Ptr(0.9),
			Confidence:       float64Ptr(0.8),
			ProcessingTimeMs: 12.0,
			RequestID:        "req-1",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()

	client := NewClient("crime", srv.URL, WithTimeout(2*time.Second))
	resp, err := client.Classify(context.Background(), "Test", "Body")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Module != "crime" {
		t.Fatalf("expected module 'crime', got %q", resp.Module)
	}
	if *resp.Relevance != 0.9 {
		t.Fatalf("expected relevance 0.9, got %f", *resp.Relevance)
	}
}

func TestClassifyTimeout(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	})
	defer srv.Close()

	client := NewClient("slow", srv.URL, WithTimeout(50*time.Millisecond))
	_, err := client.Classify(context.Background(), "Test", "Body")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestClassifyCircuitBreaker(t *testing.T) {
	callCount := 0
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	defer srv.Close()

	client := NewClient("test", srv.URL,
		WithCircuitBreaker(2, 5*time.Second),
		WithRetry(1, 10*time.Millisecond),
	)

	// Trip the breaker
	client.Classify(context.Background(), "a", "b")
	client.Classify(context.Background(), "a", "b")

	// Circuit should be open now — no network call
	beforeCount := callCount
	_, err := client.Classify(context.Background(), "a", "b")
	if err == nil {
		t.Fatal("expected ErrUnavailable when circuit is open")
	}
	if callCount != beforeCount {
		t.Fatal("circuit breaker should prevent network calls when open")
	}
}

func TestHealthSuccess(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := HealthResponse{
			Status:        "healthy",
			Module:        "crime",
			Version:       "v1",
			SchemaVersion: "1.0",
			ModelsLoaded:  true,
			UptimeSeconds: 120.0,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()

	client := NewClient("crime", srv.URL)
	health, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if health.Status != "healthy" {
		t.Fatalf("expected healthy, got %q", health.Status)
	}
}

func float64Ptr(f float64) *float64 { return &f }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd classifier && go test ./internal/mlclientv2/ -run "TestClassify|TestHealth" -v`
Expected: FAIL

- [ ] **Step 3: Implement transport.go**

```go
// classifier/internal/mlclientv2/transport.go
package mlclientv2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"time"
)

func (c *Client) doPost(ctx context.Context, path string, body any) ([]byte, int, error) {
	reqBody, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := range c.opts.retryCount {
		if attempt > 0 {
			delay := c.opts.retryBaseDelay * time.Duration(1<<(attempt-1))
			jitter := time.Duration(rand.Int64N(int64(delay / 2)))
			time.Sleep(delay + jitter)
		}

		data, status, err := c.doSinglePost(ctx, path, reqBody)
		if err == nil && status < 500 {
			return data, status, nil
		}
		lastErr = err
		if status > 0 && status != http.StatusServiceUnavailable {
			return data, status, err
		}
	}
	return nil, 0, lastErr
}

func (c *Client) doSinglePost(ctx context.Context, path string, body []byte) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.opts.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return data, resp.StatusCode, nil
}

func (c *Client) doGet(ctx context.Context, path string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.opts.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return data, resp.StatusCode, nil
}
```

- [ ] **Step 4: Implement client.go**

```go
// classifier/internal/mlclientv2/client.go
package mlclientv2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Client talks to any ML sidecar module via the standard envelope.
type Client struct {
	moduleName string
	baseURL    string
	httpClient *http.Client
	opts       clientOptions
	breaker    *circuitBreaker
}

// NewClient creates a client for a specific ML module.
func NewClient(moduleName, baseURL string, opts ...Option) *Client {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	return &Client{
		moduleName: moduleName,
		baseURL:    baseURL,
		httpClient: &http.Client{},
		opts:       o,
		breaker:    newBreaker(o.breakerTrips, o.breakerCooldown),
	}
}

// Classify sends a classification/extraction request and returns the standard envelope.
func (c *Client) Classify(ctx context.Context, title, body string) (*StandardResponse, error) {
	if !c.breaker.allow() {
		return nil, fmt.Errorf("%s: %w", c.moduleName, ErrUnavailable)
	}

	reqBody := classifyRequest{Title: title, Body: body}
	data, status, err := c.doPost(ctx, "/v1/classify", reqBody)
	if err != nil {
		c.breaker.recordFailure()
		return nil, fmt.Errorf("%s: %w", c.moduleName, ErrUnavailable)
	}

	if status == http.StatusServiceUnavailable {
		c.breaker.recordFailure()
		return nil, fmt.Errorf("%s: %w", c.moduleName, ErrUnavailable)
	}

	if status >= 400 {
		c.breaker.recordSuccess()
		var sErr StandardError
		if jsonErr := json.Unmarshal(data, &sErr); jsonErr == nil {
			return nil, fmt.Errorf("%s: %s: %s", c.moduleName, sErr.Error, sErr.Message)
		}
		return nil, fmt.Errorf("%s: HTTP %d", c.moduleName, status)
	}

	var resp StandardResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		c.breaker.recordSuccess()
		return nil, fmt.Errorf("%s: decode response: %w", c.moduleName, err)
	}

	c.breaker.recordSuccess()
	return &resp, nil
}

// Health checks the module's health endpoint.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	data, status, err := c.doGet(ctx, "/v1/health")
	if err != nil {
		return nil, fmt.Errorf("%s health: %w", c.moduleName, ErrUnavailable)
	}
	if status >= 400 {
		return nil, fmt.Errorf("%s health: HTTP %d", c.moduleName, status)
	}

	var resp HealthResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("%s health: decode: %w", c.moduleName, err)
	}
	return &resp, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd classifier && go test ./internal/mlclientv2/ -v`
Expected: All tests PASS (client, breaker)

- [ ] **Step 6: Commit**

```bash
git add classifier/internal/mlclientv2/
git commit -m "feat(mlclientv2): unified Go client with transport, retry, and circuit breaker"
```

---

### Task 17: Wire Crime Through Unified Client (Atomic Swap)

**Files:**
- Rename: `classifier/internal/mlclientv2/` -> `classifier/internal/mlclient/` (replaces old)
- Delete: `classifier/internal/mlclient/` (old client, backed up as part of rename)
- Modify: `classifier/internal/bootstrap/classifier.go` — swap crime client creation
- Modify: `classifier/internal/classifier/crime.go` — update MLClassifier interface usage
- Modify: `classifier/internal/classifier/classifier.go` — update crime classify call

This task performs the atomic swap: rename mlclientv2 to mlclient, update all imports, and wire crime through the new client in one commit.

- [ ] **Step 1: Read existing classifier wiring**

Read these files to understand the current crime ML integration:
- `classifier/internal/bootstrap/classifier.go` — how `mlclient.NewClient()` is called and passed to `NewCrimeClassifier`
- `classifier/internal/classifier/crime.go` — `MLClassifier` interface, `CrimeClassifier` struct, how `Classify()` calls the old client and processes `*mlclient.ClassifyResponse`
- `classifier/internal/classifier/classifier.go` — how `Classify()` orchestrates crime classification
- `classifier/internal/config/config.go` — crime `Enabled` and `MLServiceURL` config fields

- [ ] **Step 2: Perform the atomic rename**

```bash
rm -rf classifier/internal/mlclient
mv classifier/internal/mlclientv2 classifier/internal/mlclient
```

Then update the package declaration in all files under `classifier/internal/mlclient/`:
Change `package mlclientv2` -> `package mlclient` in every .go file.

- [ ] **Step 3: Update bootstrap to create unified client**

In `classifier/internal/bootstrap/classifier.go`, replace the old crime client creation:
```go
// Old: crimeClient := mlclient.NewClient(cfg.Classification.Crime.MLServiceURL)
// New:
crimeClient := mlclient.NewClient("crime", cfg.Classification.Crime.MLServiceURL,
    mlclient.WithTimeout(5*time.Second),
    mlclient.WithRetry(2, 100*time.Millisecond),
    mlclient.WithCircuitBreaker(5, 30*time.Second),
)
```

- [ ] **Step 4: Update crime.go to use new StandardResponse**

The `MLClassifier` interface and `CrimeClassifier` struct in `classifier/internal/classifier/crime.go` currently return/consume the old `*mlclient.ClassifyResponse`. Update to:

1. Change the interface/struct to accept `*mlclient.Client` (new unified type)
2. In the classify method, unmarshal `resp.Result` (json.RawMessage) into a domain-specific CrimeResult struct:

```go
resp, err := c.client.Classify(ctx, doc.Title, doc.Body)
if err != nil {
    // existing fallback logic unchanged
    return nil, err
}
var crimeResult CrimeMLResponse  // define this struct matching the Python crime module output
if unmarshalErr := json.Unmarshal(resp.Result, &crimeResult); unmarshalErr != nil {
    return nil, fmt.Errorf("crime result decode: %w", unmarshalErr)
}
// Map crimeResult fields to the existing ClassificationResult fields
```

- [ ] **Step 5: Update import paths**

Search all files under `classifier/` for imports of the old `mlclient` package path and ensure they point to the renamed package. Also update any imports of `mltransport` that crime.go may have used.

- [ ] **Step 6: Run classifier tests**

Run: `cd classifier && go test ./... -v`
Expected: All tests PASS

- [ ] **Step 7: Commit**

```bash
git add classifier/internal/
git commit -m "feat(classifier): wire crime through unified mlclient, atomic swap from mlclientv2"
```

---

## Chunk 3: Remaining Modules, Cleanup & Drill (Phases 4-7)

### Task 18: Port Entertainment Module (rule-based)

**Files:**
- Create: `ml-modules/entertainment/__init__.py`
- Create: `ml-modules/entertainment/module.py`
- Create: `ml-modules/entertainment/tests/__init__.py`
- Create: `ml-modules/entertainment/tests/test_module.py`
- Reference: `ml-sidecars/entertainment-ml/classifier/relevance.py`

- [ ] **Step 1: Read existing entertainment-ml code**

Read: `ml-sidecars/entertainment-ml/main.py`, `ml-sidecars/entertainment-ml/classifier/relevance.py`

- [ ] **Step 2: Write failing test**

```python
# ml-modules/entertainment/tests/test_module.py
import pytest
from nc_ml.schemas import ClassifyRequest


async def test_entertainment_module_name():
    from module import Module
    assert Module().name() == "entertainment"


async def test_entertainment_classify_returns_result():
    from module import Module, EntertainmentResult
    m = Module()
    await m.initialize()
    req = ClassifyRequest(title="New Marvel movie trailer released", body="Marvel Studios...")
    result = await m.classify(req)
    assert isinstance(result, EntertainmentResult)
    assert isinstance(result.categories, list)
    assert 0.0 <= result.relevance <= 1.0


async def test_entertainment_health_always_healthy():
    from module import Module
    m = Module()
    await m.initialize()
    checks = await m.health_checks()
    assert checks == {}  # rule-based, no models
```

- [ ] **Step 3: Implement entertainment module**

Port rule-based logic from `ml-sidecars/entertainment-ml/classifier/relevance.py`. No model files needed.

- [ ] **Step 4: Run tests, commit**

```bash
git add ml-modules/entertainment/
git commit -m "feat(ml-modules): port entertainment classifier (rule-based) to unified framework"
```

---

### Task 19: Port Indigenous Module (rule-based)

**Files:**
- Create: `ml-modules/indigenous/__init__.py`
- Create: `ml-modules/indigenous/module.py`
- Create: `ml-modules/indigenous/tests/__init__.py`
- Create: `ml-modules/indigenous/tests/test_module.py`
- Reference: `ml-sidecars/indigenous-ml/main.py`, `ml-sidecars/indigenous-ml/classifier/relevance.py`

- [ ] **Step 1: Read existing indigenous-ml code**

Read: `ml-sidecars/indigenous-ml/main.py`, `ml-sidecars/indigenous-ml/classifier/relevance.py`

Indigenous classification uses keyword-based rules to detect Indigenous-related content (community names, treaty references, band council keywords, etc.). It returns relevance score and category labels.

- [ ] **Step 2: Write failing test**

```python
# ml-modules/indigenous/tests/test_module.py
import pytest
from nc_ml.schemas import ClassifyRequest


async def test_indigenous_module_name():
    from module import Module
    assert Module().name() == "indigenous"


async def test_indigenous_classify_relevant_content():
    from module import Module, IndigenousResult
    m = Module()
    await m.initialize()
    req = ClassifyRequest(
        title="First Nation community celebrates treaty day",
        body="The band council organized events for treaty day celebrations..."
    )
    result = await m.classify(req)
    assert isinstance(result, IndigenousResult)
    assert result.relevance > 0.5
    assert isinstance(result.categories, list)


async def test_indigenous_classify_irrelevant_content():
    from module import Module, IndigenousResult
    m = Module()
    await m.initialize()
    req = ClassifyRequest(title="Stock market rises", body="Investors saw gains...")
    result = await m.classify(req)
    assert isinstance(result, IndigenousResult)
    assert result.relevance < 0.5


async def test_indigenous_health_empty_checks():
    from module import Module
    m = Module()
    await m.initialize()
    checks = await m.health_checks()
    assert checks == {}  # rule-based, no models
```

- [ ] **Step 3: Implement indigenous module**

Port rule-based keyword logic from `ml-sidecars/indigenous-ml/classifier/relevance.py`. The module implements `ClassifierModule`. No model files needed.

```python
class IndigenousResult(ClassifierResult):
    categories: list[str]
```

- [ ] **Step 4: Run tests, commit**

Run: `cd ml-modules/indigenous && PYTHONPATH=. python -m pytest tests/ -v`

```bash
git add ml-modules/indigenous/
git commit -m "feat(ml-modules): port indigenous classifier (rule-based) to unified framework"
```

---

### Task 20: Port Coforge Module (ML-based)

**Files:**
- Create: `ml-modules/coforge/__init__.py`
- Create: `ml-modules/coforge/module.py`
- Create: `ml-modules/coforge/requirements.txt`
- Copy: `ml-modules/coforge/models/` (from `ml-sidecars/coforge-ml/models/`)
- Create: `ml-modules/coforge/tests/__init__.py`
- Create: `ml-modules/coforge/tests/test_module.py`
- Reference: `ml-sidecars/coforge-ml/classifier/` (audience, industry, topic, relevance, preprocessor)

- [ ] **Step 1: Read existing coforge-ml code**

Read all files under `ml-sidecars/coforge-ml/classifier/` — audience.py, industry.py, topic.py, relevance.py, preprocessor.py

- [ ] **Step 2: Write failing test**

Test should verify CoforgeResult has: audience, audience_confidence, topics, topic_scores, industries, industry_scores.

- [ ] **Step 3: Implement coforge module**

Port classification logic. 4 models: relevance, audience, topic, industry. Use `nc_ml.model_loader.load_model()` for each.

- [ ] **Step 4: Copy models, run tests, commit**

```bash
cp -r ml-sidecars/coforge-ml/models/ ml-modules/coforge/models/ 2>/dev/null || echo "No models in dev"
cd ml-modules/coforge && PYTHONPATH=. python -m pytest tests/ -v
git add ml-modules/coforge/
git commit -m "feat(ml-modules): port coforge classifier to unified framework"
```

---

### Task 21: Port Mining Module (ML-based)

Most complex module — 4 models + `train_and_export.py` training script.

**Files:**
- Create: `ml-modules/mining/__init__.py`
- Create: `ml-modules/mining/module.py`
- Create: `ml-modules/mining/requirements.txt`
- Create: `ml-modules/mining/train_and_export.py` (copy from old sidecar, update imports)
- Copy: `ml-modules/mining/models/` (from `ml-sidecars/mining-ml/models/`)
- Create: `ml-modules/mining/tests/__init__.py`
- Create: `ml-modules/mining/tests/test_module.py`
- Reference: `ml-sidecars/mining-ml/classifier/` (relevance, mining_stage, commodity, location, preprocessor)

- [ ] **Step 1: Read existing mining-ml code**

Read all files under `ml-sidecars/mining-ml/classifier/` — relevance.py, mining_stage.py, commodity.py, location.py, preprocessor.py. Also read `ml-sidecars/mining-ml/train_and_export.py`.

- [ ] **Step 2: Write failing test**

```python
# ml-modules/mining/tests/test_module.py
import pytest
from unittest.mock import MagicMock
from nc_ml.schemas import ClassifyRequest


async def test_mining_module_name():
    from module import Module
    assert Module().name() == "mining"


async def test_mining_classify_returns_result():
    from module import Module, MiningResult
    m = Module()
    # Mock all 4 models for unit test
    m._models_loaded = True
    m._relevance_model = MagicMock(predict_proba=MagicMock(return_value=[[0.1, 0.9]]))
    m._stage_model = MagicMock(predict_proba=MagicMock(return_value=[[0.7, 0.2, 0.1]]))
    m._commodity_model = MagicMock(predict_proba=MagicMock(return_value=[[0.6, 0.3, 0.1]]))
    m._location_model = MagicMock(predict_proba=MagicMock(return_value=[[0.2, 0.8]]))

    req = ClassifyRequest(title="Gold mine expansion", body="The company announced...")
    result = await m.classify(req)

    assert isinstance(result, MiningResult)
    assert 0.0 <= result.relevance <= 1.0
    assert isinstance(result.mining_stage, str)
    assert isinstance(result.mining_stage_confidence, float)
    assert isinstance(result.commodities, list)
    assert isinstance(result.commodity_scores, dict)


async def test_mining_health_reports_models():
    from module import Module
    m = Module()
    checks = await m.health_checks()
    # Before init, models not loaded
    assert not all(checks.values()) if checks else True
```

- [ ] **Step 3: Implement mining module**

Port all 4 classifiers from `ml-sidecars/mining-ml/classifier/`. The module implements `ClassifierModule`. Use `nc_ml.model_loader.load_model()` for each of the 4 models.

```python
class MiningResult(ClassifierResult):
    mining_stage: str
    mining_stage_confidence: float
    commodities: list[str]
    commodity_scores: dict[str, float]
```

- [ ] **Step 4: Copy train_and_export.py**

Copy `ml-sidecars/mining-ml/train_and_export.py` to `ml-modules/mining/train_and_export.py`. This is a training utility, not production code. Update any imports that referenced the old `classifier/` package to work standalone. It is kept alongside the module for model retraining.

- [ ] **Step 5: Copy models, run tests, commit**

```bash
cp -r ml-sidecars/mining-ml/models/ ml-modules/mining/models/ 2>/dev/null || echo "No models in dev"
cd ml-modules/mining && PYTHONPATH=. python -m pytest tests/ -v
git add ml-modules/mining/
git commit -m "feat(ml-modules): port mining classifier to unified framework"
```

---

### Task 22: Wire All Modules Through Unified Go Client

**Files:**
- Modify: `classifier/internal/bootstrap/classifier.go` — create unified clients for all modules
- Modify: `classifier/internal/classifier/mining.go` — use `mlclient.Client` + `json.RawMessage`
- Modify: `classifier/internal/classifier/coforge.go` — same pattern
- Modify: `classifier/internal/classifier/entertainment.go` — same pattern
- Modify: `classifier/internal/classifier/indigenous.go` — same pattern
- Modify: `classifier/internal/classifier/drill.go` (if exists) — same pattern
- Delete: `classifier/internal/miningmlclient/`
- Delete: `classifier/internal/coforgemlclient/`
- Delete: `classifier/internal/entertainmentmlclient/`
- Delete: `classifier/internal/indigenousmlclient/`
- Delete: `classifier/internal/drillmlclient/` (if exists)
- Delete: `classifier/internal/mltransport/`

- [ ] **Step 1: Read each domain classification file**

Read `classifier/internal/classifier/mining.go`, `coforge.go`, `entertainment.go`, `indigenous.go` to understand how each uses its old dedicated client.

- [ ] **Step 2: Replace all remaining old clients with unified mlclient**

For each module (mining, coforge, entertainment, indigenous):
1. In bootstrap: replace old client creation with `mlclient.NewClient("{name}", cfg.{Name}.MLServiceURL, mlclient.WithTimeout(5*time.Second), mlclient.WithRetry(2, 100*time.Millisecond), mlclient.WithCircuitBreaker(5, 30*time.Second))`
2. In domain file: replace old client calls with `client.Classify()` + `json.Unmarshal(resp.Result, &domainResult)`
3. Remove imports of old client packages

- [ ] **Step 3: Delete old client packages**

```bash
rm -rf classifier/internal/miningmlclient
rm -rf classifier/internal/coforgemlclient
rm -rf classifier/internal/entertainmentmlclient
rm -rf classifier/internal/indigenousmlclient
rm -rf classifier/internal/drillmlclient
rm -rf classifier/internal/mltransport
```

- [ ] **Step 4: Verify no old imports remain**

```bash
cd classifier && grep -r "miningmlclient\|coforgemlclient\|entertainmentmlclient\|indigenousmlclient\|drillmlclient\|mltransport" internal/ --include="*.go"
```
Expected: No matches

- [ ] **Step 5: Run all classifier tests**

Run: `cd classifier && go test ./... -v`
Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
git add classifier/internal/
git commit -m "feat(classifier): wire all modules through unified mlclient, delete old clients"
```

---

### Task 23: Update Docker Compose

**Files:**
- Modify: `docker-compose.base.yml` (lines ~373-465: ML sidecar definitions)
- Modify: `docker-compose.dev.yml` (lines ~585-612: ML sidecar dev overrides)
- Modify: `docker-compose.prod.yml` (lines ~237-310: ML sidecar prod definitions)

- [ ] **Step 1: Update docker-compose.base.yml**

Replace all existing ML sidecar service definitions with new ones using the shared Dockerfile. Add the `x-ml-sidecar` YAML anchor:

```yaml
x-ml-sidecar: &ml-sidecar-defaults
  build:
    context: .
    dockerfile: docker/Dockerfile.ml-sidecar
  restart: unless-stopped
  networks:
    - north-cloud-network
  deploy:
    resources:
      limits:
        cpus: "0.5"
        memory: 512M

services:
  crime-ml:
    <<: *ml-sidecar-defaults
    build:
      args:
        MODULE_NAME: crime
    ports:
      - "${CRIME_ML_PORT:-8076}:8080"

  mining-ml:
    <<: *ml-sidecar-defaults
    build:
      args:
        MODULE_NAME: mining
    deploy:
      resources:
        limits:
          memory: 1G
    ports:
      - "${MINING_ML_PORT:-8077}:8080"

  coforge-ml:
    <<: *ml-sidecar-defaults
    build:
      args:
        MODULE_NAME: coforge
    ports:
      - "${COFORGE_ML_PORT:-8078}:8080"

  entertainment-ml:
    <<: *ml-sidecar-defaults
    build:
      args:
        MODULE_NAME: entertainment
    deploy:
      resources:
        limits:
          memory: 256M
    ports:
      - "${ENTERTAINMENT_ML_PORT:-8079}:8080"

  indigenous-ml:
    <<: *ml-sidecar-defaults
    build:
      args:
        MODULE_NAME: indigenous
    deploy:
      resources:
        limits:
          memory: 256M
    ports:
      - "${INDIGENOUS_ML_PORT:-8080}:8080"

  drill-ml:
    <<: *ml-sidecar-defaults
    build:
      args:
        MODULE_NAME: drill
    ports:
      - "${DRILL_ML_PORT:-8081}:8080"
```

- [ ] **Step 2: Update docker-compose.dev.yml**

Add `ml` profile and volume mounts for each module:
```yaml
services:
  crime-ml:
    profiles: ["ml"]
    volumes:
      - ./ml-modules/crime:/opt/module
      - ./ml-framework:/opt/ml-framework
  # Same pattern for all other modules
```

- [ ] **Step 3: Update docker-compose.prod.yml**

Update classifier `depends_on` to include all ML sidecars with `condition: service_healthy`. Update `ML_SERVICE_URL` environment variables. Update service definitions to use new build pattern.

- [ ] **Step 4: Verify compose config is valid**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml config --quiet`
Expected: No errors

- [ ] **Step 5: Verify no old sidecar image references remain**

```bash
grep -r "ml-sidecars\|jonesrussell/crime-ml\|jonesrussell/mining-ml" docker-compose*.yml
```
Expected: No matches (all should reference the new Dockerfile.ml-sidecar build)

- [ ] **Step 6: Commit**

```bash
git commit -m "feat(docker): update compose files for unified ML sidecar framework"
```

---

### Task 24: Delete Old Code

**Files:**
- Delete: `ml-sidecars/` (entire directory)
- Delete: `classifier/internal/mltransport/` (if not already deleted in Task 22)
- Delete: `classifier/internal/miningmlclient/` (if not already deleted in Task 22)
- Delete: `classifier/internal/coforgemlclient/` (if not already deleted in Task 22)
- Delete: `classifier/internal/entertainmentmlclient/` (if not already deleted in Task 22)
- Delete: `classifier/internal/indigenousmlclient/` (if not already deleted in Task 22)
- Delete: `classifier/internal/drillmlclient/` (if exists)
- Delete: `classifier/internal/mlhealth/` (old per-sidecar health check helpers)

- [ ] **Step 1: Verify no imports of old packages remain**

```bash
cd classifier && grep -r "mltransport\|miningmlclient\|coforgemlclient\|entertainmentmlclient\|indigenousmlclient\|drillmlclient\|mlhealth\|mlclientv2" internal/ --include="*.go"
```
Expected: No matches

- [ ] **Step 2: Delete old directories (skip if already deleted in Task 22)**

```bash
rm -rf ml-sidecars/
rm -rf classifier/internal/mltransport/
rm -rf classifier/internal/miningmlclient/
rm -rf classifier/internal/coforgemlclient/
rm -rf classifier/internal/entertainmentmlclient/
rm -rf classifier/internal/indigenousmlclient/
rm -rf classifier/internal/drillmlclient/
rm -rf classifier/internal/mlhealth/
```

- [ ] **Step 3: Verify classifier still compiles and tests pass**

Run: `cd classifier && go build ./... && go test ./... -v`
Expected: Build and tests PASS

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor: delete old ML sidecars and duplicated Go clients"
```

---

### Task 25: Update Documentation

**Files:**
- Modify: `CLAUDE.md`
- Modify: `ARCHITECTURE.md`
- Modify: `classifier/CLAUDE.md`

- [ ] **Step 1: Update CLAUDE.md orchestration table**

Add new entries to the orchestration table at the top of `CLAUDE.md`:

| File pattern | Service context | Spec |
|---|---|---|
| `ml-framework/**` | — | `docs/specs/unified-ml-sidecar-framework-design.md` |
| `ml-modules/**` | — | `docs/specs/unified-ml-sidecar-framework-design.md` |

Remove the old `ml-sidecars/**` entry if it exists.

Also update the "Add a new ML sidecar" section in Common Operations to reflect the new 6-step process.

- [ ] **Step 2: Update ARCHITECTURE.md**

Update the ML pipeline section with new directory structure, module pattern, and unified Go client.

- [ ] **Step 3: Update classifier/CLAUDE.md**

Replace per-client documentation with unified mlclient documentation. Document the `json.RawMessage` pattern for domain-specific result deserialization.

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md ARCHITECTURE.md classifier/CLAUDE.md
git commit -m "docs: update documentation for unified ML sidecar framework"
```

---

### Task 26: Create GitHub Issues for Missing Modules

- [ ] **Step 1: Create drill extractor issue**

```bash
gh issue create --title "feat(ml-modules): implement drill extractor module" \
  --body "Implement the drill extractor as an ExtractorModule in ml-modules/drill/.

  Spec: docs/superpowers/specs/2026-03-16-unified-ml-sidecar-framework-design.md

  - Regex + LLM hybrid extraction
  - Outputs DrillResult with intercepts array
  - First module built natively in the unified framework
  - Validates the 'add a new module' workflow"
```

- [ ] **Step 2: Create CI pipeline issue**

```bash
gh issue create --title "chore: add ml-framework and ml-modules to CI pipeline" \
  --body "Add ml-framework and ml-modules to the CI pipeline (Taskfile, GitHub Actions).

  - Add pytest runs for ml-framework/tests/
  - Add pytest runs for each ml-module's tests/
  - Add Python linting (ruff) for ml-framework/ and ml-modules/
  - Add Docker build verification for all modules"
```

- [ ] **Step 3: Create any other issues discovered during implementation**

File issues for bugs, missing tests, or improvements discovered during the migration.

- [ ] **Step 4: Commit any remaining changes**

---

### Task 27: Final Verification

- [ ] **Step 1: Run full framework test suite**

Run: `cd ml-framework && python -m pytest tests/ -v`
Expected: All tests PASS

- [ ] **Step 2: Run all module tests**

```bash
for module in crime mining coforge entertainment indigenous; do
  echo "=== Testing $module ==="
  cd ml-modules/$module && python -m pytest tests/ -v && cd ../..
done
```

- [ ] **Step 3: Run classifier tests**

Run: `cd classifier && go test ./... -v`
Expected: All tests PASS

- [ ] **Step 4: Run Go linter**

Run: `task lint:force`
Expected: No violations

- [ ] **Step 5: Run Python linter on framework and modules**

```bash
pip install ruff
ruff check ml-framework/ ml-modules/
```
Expected: No violations (or only advisory warnings)

- [ ] **Step 6: Build all containers**

```bash
for module in crime mining coforge entertainment indigenous; do
  docker build -f docker/Dockerfile.ml-sidecar --build-arg MODULE_NAME=$module -t nc-ml-$module:test .
done
```
Expected: All builds succeed

- [ ] **Step 7: Verify compose starts cleanly**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml --profile ml up -d`
Expected: All ML sidecar containers start and pass health checks
