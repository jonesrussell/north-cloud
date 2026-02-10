import pytest
from classifier.audience import AudienceClassifier


class TestAudienceClassifier:
    @pytest.fixture(autouse=True)
    def setup(self):
        self.classifier = AudienceClassifier("models/audience.joblib")

    def test_classify_returns_audience_and_confidence(self):
        result = self.classifier.classify("React framework release new hooks API")
        assert "audience" in result
        assert "confidence" in result
        assert 0.0 <= result["confidence"] <= 1.0
        assert result["audience"] in ["developer", "entrepreneur", "hybrid"]

    def test_empty_text_returns_hybrid(self):
        result = self.classifier.classify("")
        assert result["audience"] == "hybrid"
        assert result["confidence"] == 0.5
