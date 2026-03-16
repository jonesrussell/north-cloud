"""Tests for entertainment classifier module."""

import pytest

from nc_ml.schemas import ClassifyRequest

from module import EntertainmentResult, Module


@pytest.fixture
def module():
    return Module()


async def test_entertainment_module_name():
    m = Module()
    assert m.name() == "entertainment"


async def test_entertainment_classify_returns_result(module):
    await module.initialize()
    req = ClassifyRequest(title="New Marvel movie trailer released", body="Marvel Studios...")
    result = await module.classify(req)
    assert isinstance(result, EntertainmentResult)
    assert isinstance(result.categories, list)
    assert 0.0 <= result.relevance <= 1.0


async def test_entertainment_health_always_healthy(module):
    await module.initialize()
    checks = await module.health_checks()
    assert checks == {}  # rule-based, no models
