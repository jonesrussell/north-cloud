import pytest
from classifier.relevance import RelevanceClassifier


class TestRelevanceClassifier:
    @pytest.fixture(autouse=True)
    def setup(self):
        self.classifier = RelevanceClassifier("models/relevance.joblib")

    def test_classify_returns_relevance_and_confidence(self):
        result = self.classifier.classify("AI startup releases developer SDK")
        assert "relevance" in result
        assert "confidence" in result
        assert 0.0 <= result["confidence"] <= 1.0
        assert result["relevance"] in ["core_coforge", "peripheral", "not_relevant"]

    def test_empty_text_returns_not_relevant(self):
        result = self.classifier.classify("")
        assert result["relevance"] == "not_relevant"
        assert result["confidence"] == 0.5
