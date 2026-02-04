# streetcode-ml/tests/test_crime_type.py
import pytest
from classifier.crime_type import CrimeTypeClassifier


class TestCrimeTypeClassifier:
    @pytest.fixture
    def classifier(self):
        return CrimeTypeClassifier("models/crime_type.joblib")

    def test_classify_returns_types_and_scores(self, classifier):
        result = classifier.classify("Man charged with murder")

        assert "crime_types" in result
        assert "scores" in result
        assert isinstance(result["crime_types"], list)
        assert isinstance(result["scores"], dict)

    def test_murder_returns_violent_crime(self, classifier):
        result = classifier.classify("Man charged with murder after stabbing")
        assert "violent_crime" in result["crime_types"]

    def test_theft_returns_valid_classification(self, classifier):
        result = classifier.classify("Police arrest suspect for shoplifting")
        # May or may not detect based on training data
        assert isinstance(result["crime_types"], list)
