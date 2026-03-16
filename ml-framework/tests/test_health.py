"""Tests for nc_ml.health."""

from nc_ml.health import aggregate_health_status


def test_empty_checks_healthy():
    assert aggregate_health_status({}) == "healthy"


def test_all_true_healthy():
    assert aggregate_health_status({"a": True, "b": True}) == "healthy"


def test_some_false_degraded():
    assert aggregate_health_status({"a": True, "b": False}) == "degraded"


def test_all_false_unhealthy():
    assert aggregate_health_status({"a": False, "b": False}) == "unhealthy"
