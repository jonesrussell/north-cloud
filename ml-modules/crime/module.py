"""Crime ML classification module for the nc_ml framework."""

from pathlib import Path

from pydantic import ConfigDict

from nc_ml.module import ClassifierModule
from nc_ml.schemas import ClassifierResult, ClassifyRequest

from .classifier.crime_type import CrimeTypeClassifier
from .classifier.location import LocationClassifier
from .classifier.relevance import RelevanceClassifier


MAX_BODY_CHARS = 500

_RELEVANCE_SCORES = {"core_street_crime": 0.9, "peripheral_crime": 0.6, "not_crime": 0.1}


class CrimeResult(ClassifierResult):
    """Result from the crime classifier module."""

    model_config = ConfigDict(extra="forbid")

    relevance_class: str
    crime_types: list[str]
    crime_type_scores: dict[str, float]
    location_detected: bool


class Module(ClassifierModule):
    """Crime classification module.

    Runs three sub-classifiers: relevance, crime_type, and location.
    """

    def __init__(self, models_dir: str = "models") -> None:
        self._models_dir = Path(models_dir)
        self._relevance: RelevanceClassifier | None = None
        self._crime_type: CrimeTypeClassifier | None = None
        self._location: LocationClassifier | None = None

    def name(self) -> str:
        return "crime"

    def version(self) -> str:
        return "1.0.0-crime"

    def schema_version(self) -> str:
        return "1.0"

    async def initialize(self) -> None:
        """Load all three sub-models from the models directory."""
        self._relevance = RelevanceClassifier(
            str(self._models_dir / "relevance.joblib"),
        )
        self._crime_type = CrimeTypeClassifier(
            str(self._models_dir / "crime_type.joblib"),
        )
        self._location = LocationClassifier(
            str(self._models_dir / "location.joblib"),
        )

    async def shutdown(self) -> None:
        """Release model references."""
        self._relevance = None
        self._crime_type = None
        self._location = None

    async def health_checks(self) -> dict[str, bool]:
        """Report whether each sub-model is loaded."""
        return {
            "relevance_model_loaded": self._relevance is not None,
            "crime_type_model_loaded": self._crime_type is not None,
            "location_model_loaded": self._location is not None,
        }

    async def classify(self, request: ClassifyRequest) -> CrimeResult:
        """Run all three classifiers on the input text."""
        if self._relevance is None:
            raise RuntimeError("Module not initialized — call initialize() first")

        text = f"{request.title} {request.body[:MAX_BODY_CHARS]}"

        relevance_result = self._relevance.classify(text)
        crime_type_result = self._crime_type.classify(text)
        location_result = self._location.classify(text)

        return CrimeResult(
            relevance=_RELEVANCE_SCORES.get(relevance_result["relevance"], 0.1),
            confidence=relevance_result["confidence"],
            relevance_class=relevance_result["relevance"],
            crime_types=crime_type_result["crime_types"],
            crime_type_scores=crime_type_result["scores"],
            location_detected=location_result["location"] != "not_specified",
        )


def create_module() -> Module:
    """Factory function required by the nc_ml framework."""
    return Module()
