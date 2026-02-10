# streetcode-ml/main.py
"""StreetCode ML Classifier - FastAPI Server."""

import time
from contextlib import asynccontextmanager
from typing import Optional

from fastapi import FastAPI
from pydantic import BaseModel

from classifier.relevance import RelevanceClassifier
from classifier.crime_type import CrimeTypeClassifier
from classifier.location import LocationClassifier


# Application state
class AppState:
    """Application state holding loaded models."""
    relevance_classifier: Optional[RelevanceClassifier] = None
    crime_type_classifier: Optional[CrimeTypeClassifier] = None
    location_classifier: Optional[LocationClassifier] = None
    startup_time: Optional[float] = None


state = AppState()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Load ML models on startup, cleanup on shutdown."""
    # Startup
    state.relevance_classifier = RelevanceClassifier("models/relevance.joblib")
    state.crime_type_classifier = CrimeTypeClassifier("models/crime_type.joblib")
    state.location_classifier = LocationClassifier("models/location.joblib")
    state.startup_time = time.time()
    yield
    # Shutdown (nothing to clean up)


app = FastAPI(
    title="StreetCode ML Classifier",
    description="ML-based street crime classification service",
    version="1.0.0",
    lifespan=lifespan,
)


class ClassifyRequest(BaseModel):
    """Request body for classification."""
    title: str
    body: str = ""


class ClassifyResponse(BaseModel):
    """Response body for classification."""
    relevance: str
    relevance_confidence: float
    crime_types: list[str]
    crime_type_scores: dict[str, float]
    location: str
    location_confidence: float
    processing_time_ms: int


class HealthResponse(BaseModel):
    """Response body for health check."""
    status: str
    model_version: str
    uptime_seconds: float


@app.post("/classify", response_model=ClassifyResponse)
def classify(request: ClassifyRequest) -> ClassifyResponse:
    """Classify an article for street crime relevance."""
    start_time = time.time()

    # Combine title and body (use first 500 chars of body)
    max_body_chars = 500
    text = f"{request.title} {request.body[:max_body_chars]}"

    # Run classifiers
    relevance_result = state.relevance_classifier.classify(text)
    crime_type_result = state.crime_type_classifier.classify(text)
    location_result = state.location_classifier.classify(text)

    ms_per_second = 1000
    processing_time_ms = int((time.time() - start_time) * ms_per_second)

    return ClassifyResponse(
        relevance=relevance_result["relevance"],
        relevance_confidence=relevance_result["confidence"],
        crime_types=crime_type_result["crime_types"],
        crime_type_scores=crime_type_result["scores"],
        location=location_result["location"],
        location_confidence=location_result["confidence"],
        processing_time_ms=processing_time_ms,
    )


@app.get("/health", response_model=HealthResponse)
def health() -> HealthResponse:
    """Health check endpoint."""
    uptime = time.time() - state.startup_time if state.startup_time else 0
    return HealthResponse(
        status="healthy",
        model_version="1.0.0",
        uptime_seconds=uptime,
    )
