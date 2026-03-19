"""Mining ML classification module for the nc_ml framework."""

from pathlib import Path

from pydantic import ConfigDict

from nc_ml.module import ClassifierModule
from nc_ml.schemas import ClassifierResult, ClassifyRequest

from .classifier.commodity import CommodityClassifier
from .classifier.mining_stage import MiningStageClassifier
from .classifier.relevance import RelevanceClassifier


MAX_BODY_CHARS = 500

_RELEVANCE_SCORES = {"core_mining": 0.9, "peripheral_mining": 0.6, "not_mining": 0.1}


class MiningResult(ClassifierResult):
    """Result from the mining classifier module."""

    model_config = ConfigDict(extra="forbid")

    relevance_class: str
    mining_stage: str
    mining_stage_confidence: float
    commodities: list[str]
    commodity_scores: dict[str, float]


class Module(ClassifierModule):
    """Mining classification module.

    Runs three sub-classifiers: relevance, mining_stage, and commodity.
    """

    def __init__(self, models_dir: str = "models") -> None:
        self._models_dir = Path(models_dir)
        self._relevance: RelevanceClassifier | None = None
        self._stage: MiningStageClassifier | None = None
        self._commodity: CommodityClassifier | None = None

    def name(self) -> str:
        return "mining"

    def version(self) -> str:
        return "1.0.0-mining"

    def schema_version(self) -> str:
        return "1.0"

    async def initialize(self) -> None:
        """Load all three sub-models from the models directory."""
        self._relevance = RelevanceClassifier(
            str(self._models_dir / "relevance.joblib"),
        )
        self._stage = MiningStageClassifier(
            str(self._models_dir / "mining_stage.joblib"),
        )
        self._commodity = CommodityClassifier(
            str(self._models_dir / "commodity.joblib"),
        )

    async def shutdown(self) -> None:
        """Release model references."""
        self._relevance = None
        self._stage = None
        self._commodity = None

    async def health_checks(self) -> dict[str, bool]:
        """Report whether each sub-model is loaded."""
        return {
            "relevance_model_loaded": self._relevance is not None,
            "stage_model_loaded": self._stage is not None,
            "commodity_model_loaded": self._commodity is not None,
        }

    async def classify(self, request: ClassifyRequest) -> MiningResult:
        """Run all three classifiers on the input text."""
        if self._relevance is None:
            raise RuntimeError("Module not initialized — call initialize() first")

        text = f"{request.title} {request.body[:MAX_BODY_CHARS]}"

        relevance_result = self._relevance.classify(text)
        stage_result = self._stage.classify(text)
        commodity_result = self._commodity.classify(text)

        return MiningResult(
            relevance=_RELEVANCE_SCORES.get(relevance_result["relevance"], 0.1),
            confidence=relevance_result["confidence"],
            relevance_class=relevance_result["relevance"],
            mining_stage=stage_result["mining_stage"],
            mining_stage_confidence=stage_result["confidence"],
            commodities=commodity_result["commodities"],
            commodity_scores=commodity_result["scores"],
        )


def create_module() -> Module:
    """Factory function required by the nc_ml framework."""
    module_dir = Path(__file__).parent
    return Module(models_dir=str(module_dir / "models"))
