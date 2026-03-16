"""Request middleware for ML sidecars."""

import time
import uuid
from datetime import datetime, timezone

from fastapi import FastAPI, Request, Response
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.responses import JSONResponse

from nc_ml.schemas import StandardError


class RequestMiddleware(BaseHTTPMiddleware):
    """Injects request IDs, tracks timing, and catches unhandled exceptions."""

    def __init__(self, app: FastAPI, module_name: str = "unknown") -> None:
        super().__init__(app)
        self.module_name = module_name

    async def dispatch(self, request: Request, call_next) -> Response:
        request_id = request.headers.get("x-request-id", str(uuid.uuid4()))
        request.state.request_id = request_id

        start = time.perf_counter()
        try:
            response = await call_next(request)
        except Exception:
            elapsed_ms = (time.perf_counter() - start) * 1000
            error = StandardError(
                error="internal_error",
                message="An unexpected error occurred",
                module=self.module_name,
                request_id=request_id,
                timestamp=datetime.now(tz=timezone.utc),
            )
            response = JSONResponse(
                status_code=500,
                content=error.model_dump(mode="json"),
            )
            response.headers["x-request-id"] = request_id
            response.headers["x-processing-time-ms"] = f"{elapsed_ms:.2f}"
            return response

        elapsed_ms = (time.perf_counter() - start) * 1000
        response.headers["x-request-id"] = request_id
        response.headers["x-processing-time-ms"] = f"{elapsed_ms:.2f}"
        return response


def add_middleware(app: FastAPI, module_name: str) -> None:
    """Add RequestMiddleware to the app."""
    app.add_middleware(RequestMiddleware, module_name=module_name)
