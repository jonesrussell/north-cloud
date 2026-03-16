"""Tests for the mining ML classification module."""

from unittest.mock import MagicMock

import pytest

from nc_ml.schemas import ClassifyRequest

from module import MiningResult, Module


@pytest.fixture
def module() -> Module:
    """Create an uninitialized mining module."""
    return Module()


def test_mining_module_name(module: Module) -> None:
    """Module name must be 'mining'."""
    assert module.name() == "mining"


@pytest.mark.asyncio
async def test_mining_classify_returns_result(module: Module) -> None:
    """With mocked models, classify should return a valid MiningResult."""
    # Mock relevance classifier
    mock_relevance = MagicMock()
    mock_relevance.classify.return_value = {
        "relevance": "core_mining",
        "confidence": 0.95,
    }

    # Mock stage classifier
    mock_stage = MagicMock()
    mock_stage.classify.return_value = {
        "mining_stage": "exploration",
        "confidence": 0.88,
    }

    # Mock commodity classifier
    mock_commodity = MagicMock()
    mock_commodity.classify.return_value = {
        "commodities": ["gold", "copper"],
        "scores": {"gold": 0.91, "copper": 0.75, "lithium": 0.1},
    }

    # Mock location classifier
    mock_location = MagicMock()
    mock_location.classify.return_value = {
        "location": "local_canada",
        "confidence": 0.82,
    }

    # Inject mocked classifiers
    module._relevance = mock_relevance
    module._stage = mock_stage
    module._commodity = mock_commodity
    module._location = mock_location

    request = ClassifyRequest(
        title="Gold exploration project in Northern Ontario",
        body="Drill results show promising gold and copper mineralization.",
    )
    result = await module.classify(request)

    assert isinstance(result, MiningResult)
    assert result.relevance == 0.95
    assert result.confidence == 0.95
    assert result.mining_stage == "exploration"
    assert result.mining_stage_confidence == 0.88
    assert result.commodities == ["gold", "copper"]
    assert result.commodity_scores == {"gold": 0.91, "copper": 0.75, "lithium": 0.1}


@pytest.mark.asyncio
async def test_mining_health_reports_models(module: Module) -> None:
    """Before init, not all health checks should pass."""
    checks = await module.health_checks()
    assert not all(checks.values())
    assert checks["relevance_model_loaded"] is False
    assert checks["stage_model_loaded"] is False
    assert checks["commodity_model_loaded"] is False
    assert checks["location_model_loaded"] is False
