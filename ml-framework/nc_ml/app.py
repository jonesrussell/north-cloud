"""FastAPI application factory for ML sidecar modules."""

import time
from contextlib import asynccontextmanager

from fastapi import FastAPI, Request
from prometheus_client import CONTENT_TYPE_LATEST, generate_latest
from starlette.responses import Response

from nc_ml.health import aggregate_health_status
from nc_ml.logging import configure_logging, get_logger
from nc_ml.metrics import (
    errors_total,
    request_duration_seconds,
    requests_total,
    set_health,
    set_model_info,
)
from nc_ml.middleware import add_middleware
from nc_ml.module import BaseModule, ClassifierModule, ExtractorModule
from nc_ml.schemas import (
    ClassifyRequest,
    HealthResponse,
    StandardResponse,
)


def create_app(module: BaseModule) -> FastAPI:
    """Create a FastAPI app wired to the given module."""
    module_name = module.name()
    configure_logging(module_name)
    log = get_logger(module_name)

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        app.state.start_time = time.monotonic()
        try:
            await module.initialize()
            app.state.initialized = True
            set_model_info(module_name, module.version(), module.schema_version())
            set_health(module_name, "healthy")
            log.info("module_initialized", module=module_name)
        except Exception:
            app.state.initialized = False
            set_health(module_name, "unhealthy")
            log.exception("module_init_failed", module=module_name)
        yield
        await module.shutdown()
        log.info("module_shutdown", module=module_name)

    app = FastAPI(title=f"NC ML - {module_name}", lifespan=lifespan)
    # Set defaults before lifespan runs
    app.state.initialized = False
    app.state.start_time = time.monotonic()
    add_middleware(app, module_name)

    @app.post("/v1/classify")
    async def classify_endpoint(request: Request, body: ClassifyRequest) -> StandardResponse:
        request_id = getattr(request.state, "request_id", "unknown")
        start = time.perf_counter()

        try:
            if isinstance(module, ClassifierModule):
                result = await module.classify(body)
                relevance: float | None = result.relevance
                confidence: float | None = result.confidence
            elif isinstance(module, ExtractorModule):
                result = await module.extract(body)
                relevance = None
                confidence = None
            else:
                msg = f"Module {module_name} does not support classify or extract"
                raise TypeError(msg)
        except Exception:
            errors_total.labels(module=module_name, error_code="prediction_error").inc()
            requests_total.labels(module=module_name, status="error").inc()
            raise

        elapsed_ms = (time.perf_counter() - start) * 1000
        request_duration_seconds.labels(module=module_name).observe(elapsed_ms / 1000)
        requests_total.labels(module=module_name, status="ok").inc()

        return StandardResponse(
            module=module_name,
            version=module.version(),
            schema_version=module.schema_version(),
            result=result,
            relevance=relevance,
            confidence=confidence,
            processing_time_ms=elapsed_ms,
            request_id=request_id,
        )

    @app.get("/v1/health")
    async def health_endpoint(request: Request) -> HealthResponse:
        initialized = request.app.state.initialized
        start_time = request.app.state.start_time

        if not initialized:
            status = "unhealthy"
            checks: dict[str, bool] = {"initialized": False}
        else:
            checks = await module.health_checks()
            status = aggregate_health_status(checks)

        set_health(module_name, status)

        return HealthResponse(
            status=status,
            module=module_name,
            version=module.version(),
            schema_version=module.schema_version(),
            models_loaded=initialized,
            uptime_seconds=time.monotonic() - start_time,
            checks=checks,
        )

    @app.get("/metrics")
    async def metrics_endpoint() -> Response:
        return Response(
            content=generate_latest(),
            media_type=CONTENT_TYPE_LATEST,
        )

    return app
