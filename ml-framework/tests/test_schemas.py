"""Tests for nc_ml.schemas."""

from datetime import datetime, timezone

import pytest
from pydantic import ValidationError

from nc_ml.schemas import (
    ClassifierResult,
    ClassifyRequest,
    ExtractorResult,
    HealthResponse,
    ModuleResult,
    StandardError,
    StandardResponse,
)


def test_classify_request_valid():
    req = ClassifyRequest(title="Test", body="Body text")
    assert req.title == "Test"
    assert req.body == "Body text"
    assert req.metadata is None


def test_classify_request_with_metadata():
    req = ClassifyRequest(title="T", body="B", metadata={"key": "val"})
    assert req.metadata == {"key": "val"}


def test_classify_request_missing_required():
    with pytest.raises(ValidationError):
        ClassifyRequest(title="T")  # missing body


def test_classifier_result_valid():
    r = ClassifierResult(relevance=0.9, confidence=0.85)
    assert r.relevance == 0.9
    assert r.confidence == 0.85


def test_module_result_forbid_extra():
    with pytest.raises(ValidationError):
        ClassifierResult(relevance=0.9, confidence=0.8, extra_field="bad")


def test_extractor_result_valid():
    r = ExtractorResult()
    assert isinstance(r, ModuleResult)


def test_standard_response_valid():
    resp = StandardResponse(
        module="test",
        version="1.0.0",
        schema_version="1",
        result=ClassifierResult(relevance=0.9, confidence=0.8),
        relevance=0.9,
        confidence=0.8,
        processing_time_ms=12.5,
        request_id="abc-123",
    )
    assert resp.module == "test"


def test_standard_response_null_relevance():
    resp = StandardResponse(
        module="test",
        version="1.0.0",
        schema_version="1",
        result=ExtractorResult(),
        relevance=None,
        confidence=None,
        processing_time_ms=5.0,
        request_id="def-456",
    )
    assert resp.relevance is None
    assert resp.confidence is None


def test_standard_error_valid():
    err = StandardError(
        error="internal_error",
        message="Something went wrong",
        module="test",
        request_id="abc",
        timestamp=datetime.now(tz=timezone.utc),
    )
    assert err.error == "internal_error"


def test_health_response_valid_statuses():
    for status in ("healthy", "degraded", "unhealthy"):
        h = HealthResponse(
            status=status,
            module="test",
            version="1.0.0",
            schema_version="1",
            models_loaded=True,
            uptime_seconds=100.0,
        )
        assert h.status == status


def test_health_response_invalid_status():
    with pytest.raises(ValidationError):
        HealthResponse(
            status="broken",
            module="test",
            version="1.0.0",
            schema_version="1",
            models_loaded=True,
            uptime_seconds=100.0,
        )
