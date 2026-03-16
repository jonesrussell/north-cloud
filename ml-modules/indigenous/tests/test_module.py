"""Tests for indigenous classifier module."""

import pytest

from nc_ml.schemas import ClassifyRequest

from module import IndigenousResult, Module


@pytest.fixture
def module():
    return Module()


async def test_indigenous_module_name():
    m = Module()
    assert m.name() == "indigenous"


async def test_indigenous_classify_relevant_content(module):
    await module.initialize()
    req = ClassifyRequest(
        title="First Nation community celebrates treaty day",
        body="The band council organized events for treaty day celebrations...",
    )
    result = await module.classify(req)
    assert isinstance(result, IndigenousResult)
    assert result.relevance > 0.5
    assert isinstance(result.categories, list)


async def test_indigenous_classify_irrelevant_content(module):
    await module.initialize()
    req = ClassifyRequest(title="Stock market rises", body="Investors saw gains...")
    result = await module.classify(req)
    assert isinstance(result, IndigenousResult)
    assert result.relevance < 0.5


async def test_indigenous_health_empty_checks(module):
    await module.initialize()
    checks = await module.health_checks()
    assert checks == {}  # rule-based, no models
