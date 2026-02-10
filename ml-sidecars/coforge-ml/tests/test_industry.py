import pytest
from classifier.industry import IndustryClassifier


class TestIndustryClassifier:
    @pytest.fixture(autouse=True)
    def setup(self):
        self.classifier = IndustryClassifier("models/industry.joblib")

    def test_classify_returns_industries_and_scores(self):
        result = self.classifier.classify("Cloud security platform raises funding")
        assert "industries" in result
        assert "scores" in result
        assert isinstance(result["industries"], list)
        assert isinstance(result["scores"], dict)

    def test_all_scores_are_valid(self):
        result = self.classifier.classify("Fintech SaaS acquisition")
        for score in result["scores"].values():
            assert 0.0 <= score <= 1.0

    def test_empty_text_returns_empty(self):
        result = self.classifier.classify("")
        assert result["industries"] == []
        assert result["scores"] == {}
