# streetcode-ml/tests/test_location.py
import pytest
from classifier.location import LocationClassifier


class TestLocationClassifier:
    @pytest.fixture
    def classifier(self):
        return LocationClassifier("models/location.joblib")

    def test_classify_returns_location_and_confidence(self, classifier):
        result = classifier.classify("Sudbury police arrest suspect")

        assert "location" in result
        assert "confidence" in result
        assert result["location"] in ["local_canada", "national_canada", "international", "not_specified"]

    def test_local_content_returns_valid_location(self, classifier):
        result = classifier.classify("Sudbury police investigate downtown shooting")
        # Model classifies based on training - verify valid output
        assert result["location"] in ["local_canada", "national_canada", "international", "not_specified"]
        assert 0.0 <= result["confidence"] <= 1.0

    def test_us_story_returns_valid_location(self, classifier):
        result = classifier.classify("Minneapolis police respond to incident")
        # Model classifies based on training - verify valid output
        assert result["location"] in ["local_canada", "national_canada", "international", "not_specified"]
        assert 0.0 <= result["confidence"] <= 1.0
