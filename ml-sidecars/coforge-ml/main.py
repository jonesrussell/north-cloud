"""Coforge ML Classifier - FastAPI Server."""

import time
from contextlib import asynccontextmanager
from typing import Optional

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from classifier.relevance import RelevanceClassifier
from classifier.audience import AudienceClassifier
from classifier.topic import TopicClassifier
from classifier.industry import IndustryClassifier


MODEL_VERSION = "2026-02-08-coforge-v1"


class AppState:
    """Application state holding loaded models."""

    relevance_classifier: Optional[RelevanceClassifier] = None
    audience_classifier: Optional[AudienceClassifier] = None
    topic_classifier: Optional[TopicClassifier] = None
    industry_classifier: Optional[IndustryClassifier] = None
    startup_time: Optional[float] = None
    models_loaded: bool = False


state = AppState()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Load ML models on startup, cleanup on shutdown."""
    try:
        state.relevance_classifier = RelevanceClassifier("models/relevance.joblib")
        state.audience_classifier = AudienceClassifier("models/audience.joblib")
        state.topic_classifier = TopicClassifier("models/topic.joblib")
        state.industry_classifier = IndustryClassifier("models/industry.joblib")
        state.models_loaded = True
    except Exception:
        state.models_loaded = False
    state.startup_time = time.time()
    yield


app = FastAPI(
    title="Coforge ML Classifier",
    description="ML-based content classification for developers and entrepreneurs",
    version="1.0.0",
    lifespan=lifespan,
)

max_body_chars = 500
ms_per_second = 1000


class ClassifyRequest(BaseModel):
    """Request body for classification."""

    title: str
    body: str = ""


class ClassifyResponse(BaseModel):
    """Response body for classification."""

    relevance: str
    relevance_confidence: float
    audience: str
    audience_confidence: float
    topics: list[str]
    topic_scores: dict[str, float]
    industries: list[str]
    industry_scores: dict[str, float]
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
    """Classify an article for coforge relevance, audience, topics, and industry."""
    start_time = time.time()

    text = f"{request.title} {request.body[:max_body_chars]}"

    if not state.models_loaded:
        processing_time_ms = int((time.time() - start_time) * ms_per_second)
        return ClassifyResponse(
            relevance="not_relevant",
            relevance_confidence=0.0,
            audience="hybrid",
            audience_confidence=0.0,
            topics=[],
            topic_scores={},
            industries=[],
            industry_scores={},
            processing_time_ms=processing_time_ms,
            model_version=MODEL_VERSION,
        )

    relevance_result = state.relevance_classifier.classify(text)
    audience_result = state.audience_classifier.classify(text)
    topic_result = state.topic_classifier.classify(text)
    industry_result = state.industry_classifier.classify(text)

    processing_time_ms = int((time.time() - start_time) * ms_per_second)

    return ClassifyResponse(
        relevance=relevance_result["relevance"],
        relevance_confidence=relevance_result["confidence"],
        audience=audience_result["audience"],
        audience_confidence=audience_result["confidence"],
        topics=topic_result["topics"],
        topic_scores=topic_result["scores"],
        industries=industry_result["industries"],
        industry_scores=industry_result["scores"],
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
