"""Coforge ML classification module for the nc_ml framework."""

from pathlib import Path

from pydantic import ConfigDict

from nc_ml.module import ClassifierModule
from nc_ml.schemas import ClassifierResult, ClassifyRequest

from .classifier.audience import AudienceClassifier
from .classifier.industry import IndustryClassifier
from .classifier.relevance import RelevanceClassifier
from .classifier.topic import TopicClassifier


MAX_BODY_CHARS = 500

_RELEVANCE_SCORES = {"core_coforge": 0.9, "peripheral": 0.6, "not_relevant": 0.1}


class CoforgeResult(ClassifierResult):
    """Result from the coforge classifier module."""

    model_config = ConfigDict(extra="forbid")

    relevance_class: str
    audience: str
    audience_confidence: float
    topics: list[str]
    topic_scores: dict[str, float]
    industries: list[str]
    industry_scores: dict[str, float]


class Module(ClassifierModule):
    """Coforge classification module.

    Runs four sub-classifiers: relevance, audience, topic, and industry.
    """

    def __init__(self, models_dir: str = "models") -> None:
        self._models_dir = Path(models_dir)
        self._relevance: RelevanceClassifier | None = None
        self._audience: AudienceClassifier | None = None
        self._topic: TopicClassifier | None = None
        self._industry: IndustryClassifier | None = None

    def name(self) -> str:
        return "coforge"

    def version(self) -> str:
        return "1.0.0-coforge"

    def schema_version(self) -> str:
        return "1.0"

    async def initialize(self) -> None:
        """Load all four sub-models from the models directory."""
        self._relevance = RelevanceClassifier(
            str(self._models_dir / "relevance.joblib"),
        )
        self._audience = AudienceClassifier(
            str(self._models_dir / "audience.joblib"),
        )
        self._topic = TopicClassifier(
            str(self._models_dir / "topic.joblib"),
        )
        self._industry = IndustryClassifier(
            str(self._models_dir / "industry.joblib"),
        )

    async def shutdown(self) -> None:
        """Release model references."""
        self._relevance = None
        self._audience = None
        self._topic = None
        self._industry = None

    async def health_checks(self) -> dict[str, bool]:
        """Report whether each sub-model is loaded."""
        return {
            "relevance_model_loaded": self._relevance is not None,
            "audience_model_loaded": self._audience is not None,
            "topic_model_loaded": self._topic is not None,
            "industry_model_loaded": self._industry is not None,
        }

    async def classify(self, request: ClassifyRequest) -> CoforgeResult:
        """Run all four classifiers on the input text."""
        if self._relevance is None:
            raise RuntimeError("Module not initialized — call initialize() first")

        text = f"{request.title} {request.body[:MAX_BODY_CHARS]}"

        relevance_result = self._relevance.classify(text)
        audience_result = self._audience.classify(text)
        topic_result = self._topic.classify(text)
        industry_result = self._industry.classify(text)

        return CoforgeResult(
            relevance=_RELEVANCE_SCORES.get(relevance_result["relevance"], 0.1),
            confidence=relevance_result["confidence"],
            relevance_class=relevance_result["relevance"],
            audience=audience_result["audience"],
            audience_confidence=audience_result["confidence"],
            topics=topic_result["topics"],
            topic_scores=topic_result["scores"],
            industries=industry_result["industries"],
            industry_scores=industry_result["scores"],
        )


def create_module() -> Module:
    """Factory function required by the nc_ml framework."""
    return Module()
