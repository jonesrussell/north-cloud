"""Tests for nc_ml.metrics."""

from nc_ml.metrics import (
    errors_total,
    health_status,
    requests_total,
    set_health,
    set_model_info,
    model_info,
)


def test_counter_increments():
    before = requests_total.labels(module="test", status="ok")._value.get()
    requests_total.labels(module="test", status="ok").inc()
    after = requests_total.labels(module="test", status="ok")._value.get()
    assert after == before + 1


def test_error_counter_increments():
    before = errors_total.labels(module="test", error_code="500")._value.get()
    errors_total.labels(module="test", error_code="500").inc()
    after = errors_total.labels(module="test", error_code="500")._value.get()
    assert after == before + 1


def test_set_model_info():
    set_model_info("test", "1.0.0", "1")
    val = model_info.labels(module="test", version="1.0.0", schema_version="1")._value.get()
    assert val == 1


def test_set_health_toggles():
    set_health("test-h", "healthy")
    assert health_status.labels(module="test-h", status="healthy")._value.get() == 1
    assert health_status.labels(module="test-h", status="degraded")._value.get() == 0
    assert health_status.labels(module="test-h", status="unhealthy")._value.get() == 0

    set_health("test-h", "degraded")
    assert health_status.labels(module="test-h", status="healthy")._value.get() == 0
    assert health_status.labels(module="test-h", status="degraded")._value.get() == 1
