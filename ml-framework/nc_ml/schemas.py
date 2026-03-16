"""Standard schemas for NC ML sidecar modules."""

from datetime import datetime
from typing import Any, Literal

from pydantic import BaseModel, ConfigDict


class ClassifyRequest(BaseModel):
    """Standard request for classification or extraction."""

    title: str
    body: str
    metadata: dict[str, Any] | None = None


class ModuleResult(BaseModel):
    """Base result model. Subclasses must not add extra fields."""

    model_config = ConfigDict(extra="forbid")


class ClassifierResult(ModuleResult):
    """Result from a classifier module."""

    relevance: float
    confidence: float


class ExtractorResult(ModuleResult):
    """Result from an extractor module."""


class StandardResponse(BaseModel):
    """Envelope returned by every ML sidecar endpoint."""

    module: str
    version: str
    schema_version: str
    result: ModuleResult
    relevance: float | None
    confidence: float | None
    processing_time_ms: float
    request_id: str


class StandardError(BaseModel):
    """Error envelope for failed requests."""

    error: str
    message: str
    module: str
    request_id: str
    timestamp: datetime


class HealthResponse(BaseModel):
    """Response from the health endpoint."""

    status: Literal["healthy", "degraded", "unhealthy"]
    module: str
    version: str
    schema_version: str
    models_loaded: bool
    uptime_seconds: float
    checks: dict[str, bool] | None = None
