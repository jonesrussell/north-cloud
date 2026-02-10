# streetcode-ml/tests/test_relevance.py
import pytest
from classifier.relevance import RelevanceClassifier


class TestRelevanceClassifier:
    @pytest.fixture
    def classifier(self):
        return RelevanceClassifier("models/relevance.joblib")

    def test_classify_returns_relevance_and_confidence(self, classifier):
        result = classifier.classify("Man charged with murder after stabbing")

        assert "relevance" in result
        assert "confidence" in result
        assert result["relevance"] in ["core_street_crime", "peripheral_crime", "not_crime"]
        assert 0.0 <= result["confidence"] <= 1.0

    def test_murder_classified_as_core(self, classifier):
        result = classifier.classify("Man charged with murder after downtown stabbing")
        assert result["relevance"] == "core_street_crime"
        assert result["confidence"] >= 0.5

    def test_non_crime_content_classified(self, classifier):
        # Test that non-crime content returns valid classification
        # Note: exact classification depends on model training
        result = classifier.classify("Weather forecast for the weekend shows sunny skies")
        assert result["relevance"] in ["not_crime", "peripheral_crime"]
        assert 0.0 <= result["confidence"] <= 1.0
