import pytest
from classifier.topic import TopicClassifier


class TestTopicClassifier:
    @pytest.fixture(autouse=True)
    def setup(self):
        self.classifier = TopicClassifier("models/topic.joblib")

    def test_classify_returns_topics_and_scores(self):
        result = self.classifier.classify("AI startup raises Series A funding")
        assert "topics" in result
        assert "scores" in result
        assert isinstance(result["topics"], list)
        assert isinstance(result["scores"], dict)

    def test_all_scores_are_valid(self):
        result = self.classifier.classify("Open source framework release")
        for score in result["scores"].values():
            assert 0.0 <= score <= 1.0

    def test_empty_text_returns_empty(self):
        result = self.classifier.classify("")
        assert result["topics"] == []
        assert result["scores"] == {}
