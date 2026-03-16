"""Tests for the coforge ML classification module."""

from unittest.mock import MagicMock

import pytest

from nc_ml.schemas import ClassifyRequest

from module import CoforgeResult, Module


@pytest.fixture
def module() -> Module:
    """Create an uninitialized coforge module."""
    return Module()


def test_coforge_module_name(module: Module) -> None:
    """Module name must be 'coforge'."""
    assert module.name() == "coforge"


@pytest.mark.asyncio
async def test_coforge_classify_returns_result(module: Module) -> None:
    """With mocked models, classify should return a valid CoforgeResult."""
    # Mock relevance classifier
    mock_relevance = MagicMock()
    mock_relevance.classify.return_value = {
        "relevance": "core_coforge",
        "confidence": 0.93,
    }

    # Mock audience classifier
    mock_audience = MagicMock()
    mock_audience.classify.return_value = {
        "audience": "developer",
        "confidence": 0.87,
    }

    # Mock topic classifier
    mock_topic = MagicMock()
    mock_topic.classify.return_value = {
        "topics": ["framework_release", "open_source"],
        "scores": {"framework_release": 0.82, "open_source": 0.71, "devtools": 0.15},
    }

    # Mock industry classifier
    mock_industry = MagicMock()
    mock_industry.classify.return_value = {
        "industries": ["ai_ml", "devtools"],
        "scores": {"ai_ml": 0.89, "devtools": 0.65, "fintech": 0.12},
    }

    # Inject mocked classifiers
    module._relevance = mock_relevance
    module._audience = mock_audience
    module._topic = mock_topic
    module._industry = mock_industry

    request = ClassifyRequest(
        title="New AI framework released as open source",
        body="A major tech company has released their internal ML framework.",
    )
    result = await module.classify(request)

    assert isinstance(result, CoforgeResult)
    assert result.relevance == 0.93
    assert result.confidence == 0.93
    assert result.audience == "developer"
    assert result.audience_confidence == 0.87
    assert result.topics == ["framework_release", "open_source"]
    assert result.topic_scores == {"framework_release": 0.82, "open_source": 0.71, "devtools": 0.15}
    assert result.industries == ["ai_ml", "devtools"]
    assert result.industry_scores == {"ai_ml": 0.89, "devtools": 0.65, "fintech": 0.12}


@pytest.mark.asyncio
async def test_coforge_health_reports_models(module: Module) -> None:
    """Before init, not all health checks should pass."""
    checks = await module.health_checks()
    assert not all(checks.values())
    assert checks["relevance_model_loaded"] is False
    assert checks["audience_model_loaded"] is False
    assert checks["topic_model_loaded"] is False
    assert checks["industry_model_loaded"] is False
