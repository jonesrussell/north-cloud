"""Tests for nc_ml.app."""

import pytest
from asgi_lifespan import LifespanManager
from httpx import ASGITransport, AsyncClient

from nc_ml.app import create_app
from nc_ml.module import ClassifierModule, ExtractorModule
from nc_ml.schemas import ClassifierResult, ClassifyRequest, ExtractorResult


class StubClassifier(ClassifierModule):
    def name(self) -> str:
        return "stub-classifier"

    def version(self) -> str:
        return "0.1.0"

    def schema_version(self) -> str:
        return "1"

    async def initialize(self) -> None:
        pass

    async def shutdown(self) -> None:
        pass

    async def health_checks(self) -> dict[str, bool]:
        return {"model": True}

    async def classify(self, request: ClassifyRequest) -> ClassifierResult:
        return ClassifierResult(relevance=0.75, confidence=0.9)


class StubExtractor(ExtractorModule):
    def name(self) -> str:
        return "stub-extractor"

    def version(self) -> str:
        return "0.2.0"

    def schema_version(self) -> str:
        return "1"

    async def initialize(self) -> None:
        pass

    async def shutdown(self) -> None:
        pass

    async def health_checks(self) -> dict[str, bool]:
        return {"ready": True}

    async def extract(self, request: ClassifyRequest) -> ExtractorResult:
        return ExtractorResult()


class FailingModule(ClassifierModule):
    """Module whose initialize raises an error."""

    def name(self) -> str:
        return "failing-module"

    def version(self) -> str:
        return "0.0.1"

    def schema_version(self) -> str:
        return "1"

    async def initialize(self) -> None:
        raise RuntimeError("init failed")

    async def shutdown(self) -> None:
        pass

    async def health_checks(self) -> dict[str, bool]:
        return {"model": False}

    async def classify(self, request: ClassifyRequest) -> ClassifierResult:
        return ClassifierResult(relevance=0.0, confidence=0.0)


@pytest.fixture
async def classifier_app():
    app = create_app(StubClassifier())
    async with LifespanManager(app) as manager:
        yield manager.app


@pytest.fixture
async def extractor_app():
    app = create_app(StubExtractor())
    async with LifespanManager(app) as manager:
        yield manager.app


@pytest.fixture
async def failing_app():
    app = create_app(FailingModule())
    async with LifespanManager(app) as manager:
        yield manager.app


async def test_classify_returns_standard_response(classifier_app):
    transport = ASGITransport(app=classifier_app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post("/v1/classify", json={"title": "T", "body": "B"})
    assert resp.status_code == 200
    data = resp.json()
    assert data["module"] == "stub-classifier"
    assert data["relevance"] == 0.75
    assert data["confidence"] == 0.9
    assert "processing_time_ms" in data
    assert "request_id" in data


async def test_extract_returns_null_relevance(extractor_app):
    transport = ASGITransport(app=extractor_app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post("/v1/classify", json={"title": "T", "body": "B"})
    assert resp.status_code == 200
    data = resp.json()
    assert data["relevance"] is None
    assert data["confidence"] is None


async def test_health_endpoint(classifier_app):
    transport = ASGITransport(app=classifier_app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/v1/health")
    assert resp.status_code == 200
    data = resp.json()
    assert data["status"] == "healthy"
    assert data["models_loaded"] is True
    assert data["module"] == "stub-classifier"


async def test_metrics_endpoint(classifier_app):
    transport = ASGITransport(app=classifier_app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/metrics")
    assert resp.status_code == 200
    assert "ncml_" in resp.text
    assert resp.headers["content-type"].startswith("text/plain")


async def test_validation_error_returns_422(classifier_app):
    transport = ASGITransport(app=classifier_app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post("/v1/classify", json={"title": "T"})
    assert resp.status_code == 422


async def test_failed_init_returns_unhealthy(failing_app):
    transport = ASGITransport(app=failing_app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/v1/health")
    assert resp.status_code == 200
    data = resp.json()
    assert data["status"] == "unhealthy"
    assert data["models_loaded"] is False
