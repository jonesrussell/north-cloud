"""Entertainment ML Classifier - FastAPI Server (rule-based; ML later)."""

import time

from fastapi import FastAPI
from pydantic import BaseModel

from classifier.relevance import classify_entertainment_relevance

MODEL_VERSION = "2026-02-08-entertainment-v1"


app = FastAPI(
    title="Entertainment ML Classifier",
    description="Entertainment classification service (rule-based; ML model later)",
    version="1.0.0",
)


class ClassifyRequest(BaseModel):
    """Request body for classification."""

    title: str
    body: str = ""


class ClassifyResponse(BaseModel):
    """Response body for classification."""

    relevance: str
    relevance_confidence: float
    categories: list[str]
    processing_time_ms: int = 0
    model_version: str = ""


class HealthResponse(BaseModel):
    """Response body for health check. model_version for classifier ml-health."""

    status: str
    model_version: str


@app.post("/classify", response_model=ClassifyResponse)
def classify(request: ClassifyRequest) -> ClassifyResponse:
    """Classify an article for entertainment relevance."""
    start_time = time.time()
    max_body_chars = 500
    body = (request.body or "")[:max_body_chars]
    text = f"{request.title} {body}".strip()

    result = classify_entertainment_relevance(text)
    processing_time_ms = int((time.time() - start_time) * 1000)

    return ClassifyResponse(
        relevance=result["relevance"],
        relevance_confidence=result["confidence"],
        categories=result["categories"],
        processing_time_ms=processing_time_ms,
        model_version=MODEL_VERSION,
    )


@app.get("/health", response_model=HealthResponse)
def health() -> HealthResponse:
    """Health check. Returns 200 with model_version for classifier ml-health."""
    return HealthResponse(status="healthy", model_version=MODEL_VERSION)
