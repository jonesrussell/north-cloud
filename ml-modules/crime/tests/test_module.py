"""Tests for the crime ML classification module."""

from unittest.mock import MagicMock, patch

import pytest

from nc_ml.schemas import ClassifyRequest

from module import CrimeResult, Module


@pytest.fixture
def module() -> Module:
    """Create an uninitialized crime module."""
    return Module()


def test_crime_module_name(module: Module) -> None:
    """Module name must be 'crime'."""
    assert module.name() == "crime"


def test_crime_module_version(module: Module) -> None:
    """Module version must contain 'crime'."""
    assert "crime" in module.version()


def test_crime_module_schema_version(module: Module) -> None:
    """Schema version must be '1.0'."""
    assert module.schema_version() == "1.0"


@pytest.mark.asyncio
async def test_crime_module_health_no_models(module: Module) -> None:
    """Before init, health checks should report models not loaded."""
    checks = await module.health_checks()
    assert not all(checks.values())
    assert checks["relevance_model_loaded"] is False
    assert checks["crime_type_model_loaded"] is False
    assert checks["location_model_loaded"] is False


@pytest.mark.asyncio
async def test_crime_classify_returns_result(module: Module) -> None:
    """With mocked models, classify should return a valid CrimeResult."""
    # Mock relevance classifier
    mock_relevance = MagicMock()
    mock_relevance.classify.return_value = {
        "relevance": "core_street_crime",
        "confidence": 0.92,
    }

    # Mock crime type classifier
    mock_crime_type = MagicMock()
    mock_crime_type.classify.return_value = {
        "crime_types": ["assault", "robbery"],
        "scores": {"assault": 0.85, "robbery": 0.72, "theft": 0.1},
    }

    # Mock location classifier
    mock_location = MagicMock()
    mock_location.classify.return_value = {
        "location": "local_canada",
        "confidence": 0.88,
    }

    # Inject mocked classifiers
    module._relevance = mock_relevance
    module._crime_type = mock_crime_type
    module._location = mock_location

    request = ClassifyRequest(title="Man arrested after assault", body="A man was arrested downtown.")
    result = await module.classify(request)

    assert isinstance(result, CrimeResult)
    assert result.relevance == 0.9  # core_street_crime maps to 0.9
    assert result.confidence == 0.92
    assert result.relevance_class == "core_street_crime"
    assert result.crime_types == ["assault", "robbery"]
    assert result.crime_type_scores == {"assault": 0.85, "robbery": 0.72, "theft": 0.1}
    assert result.location_detected is True


@pytest.mark.asyncio
async def test_crime_classify_uninitialized_raises(module: Module) -> None:
    """Calling classify before initialize should raise RuntimeError."""
    request = ClassifyRequest(title="Test", body="Test body")
    with pytest.raises(RuntimeError, match="not initialized"):
        await module.classify(request)
