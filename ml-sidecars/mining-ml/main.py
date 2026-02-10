"""Mining ML Classifier - FastAPI Server."""

import time
from contextlib import asynccontextmanager
from typing import Optional

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from classifier.relevance import RelevanceClassifier
from classifier.mining_stage import MiningStageClassifier
from classifier.commodity import CommodityClassifier
from classifier.location import LocationClassifier


MODEL_VERSION = "2025-02-01-mining-v1"


# Application state
class AppState:
    """Application state holding loaded models."""

    relevance_classifier: Optional[RelevanceClassifier] = None
    mining_stage_classifier: Optional[MiningStageClassifier] = None
    commodity_classifier: Optional[CommodityClassifier] = None
    location_classifier: Optional[LocationClassifier] = None
    startup_time: Optional[float] = None
    models_loaded: bool = False


state = AppState()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Load ML models on startup, cleanup on shutdown."""
    # Startup
    try:
        state.relevance_classifier = RelevanceClassifier("models/relevance.joblib")
        state.mining_stage_classifier = MiningStageClassifier("models/mining_stage.joblib")
        state.commodity_classifier = CommodityClassifier("models/commodity.joblib")
        state.location_classifier = LocationClassifier("models/location.joblib")
        state.models_loaded = True
    except Exception:
        state.models_loaded = False
    state.startup_time = time.time()
    yield
    # Shutdown (nothing to clean up)


app = FastAPI(
    title="Mining ML Classifier",
    description="ML-based mining industry classification service",
    version="1.0.0",
    lifespan=lifespan,
)


class ClassifyRequest(BaseModel):
    """Request body for classification."""

    title: str
    body: str = ""


class ClassifyResponse(BaseModel):
    """Response body for classification.

    All keys are always present. Scores are independent probabilities per label;
    they are not normalized to 1.0 across labels.
    """

    relevance: str
    relevance_confidence: float
    mining_stage: str
    mining_stage_confidence: float
    commodities: list[str]
    commodity_scores: dict[str, float]
    location: str
    location_confidence: float
    processing_time_ms: int
    model_version: str


class HealthResponse(BaseModel):
    """Response body for health check."""

    status: str
    model_version: str
    models_loaded: bool
    uptime_seconds: float


@app.post("/classify", response_model=ClassifyResponse)
def classify(request: ClassifyRequest) -> ClassifyResponse:
    """Classify an article for mining relevance."""
    start_time = time.time()

    max_body_chars = 500
    text = f"{request.title} {request.body[:max_body_chars]}"

    if not state.models_loaded:
        processing_time_ms = int((time.time() - start_time) * 1000)
        return ClassifyResponse(
            relevance="not_mining",
            relevance_confidence=0.0,
            mining_stage="unspecified",
            mining_stage_confidence=0.0,
            commodities=[],
            commodity_scores={},
            location="not_specified",
            location_confidence=0.0,
            processing_time_ms=processing_time_ms,
            model_version=MODEL_VERSION,
        )

    # Run classifiers
    relevance_result = state.relevance_classifier.classify(text)
    mining_stage_result = state.mining_stage_classifier.classify(text)
    commodity_result = state.commodity_classifier.classify(text)
    location_result = state.location_classifier.classify(text)

    processing_time_ms = int((time.time() - start_time) * 1000)

    return ClassifyResponse(
        relevance=relevance_result["relevance"],
        relevance_confidence=relevance_result["confidence"],
        mining_stage=mining_stage_result["mining_stage"],
        mining_stage_confidence=mining_stage_result["confidence"],
        commodities=commodity_result["commodities"],
        commodity_scores=commodity_result["scores"],
        location=location_result["location"],
        location_confidence=location_result["confidence"],
        processing_time_ms=processing_time_ms,
        model_version=MODEL_VERSION,
    )


@app.get("/health", response_model=HealthResponse)
def health() -> HealthResponse:
    """Health check endpoint. Returns 200 only if models are loaded."""
    uptime = time.time() - state.startup_time if state.startup_time else 0
    response = HealthResponse(
        status="healthy" if state.models_loaded else "unhealthy",
        model_version=MODEL_VERSION,
        models_loaded=state.models_loaded,
        uptime_seconds=uptime,
    )
    if not state.models_loaded:
        raise HTTPException(status_code=503, detail="Models not loaded")
    return response
