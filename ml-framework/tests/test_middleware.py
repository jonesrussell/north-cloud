"""Tests for nc_ml.middleware."""

from fastapi import FastAPI, Request
from httpx import ASGITransport, AsyncClient

from nc_ml.middleware import add_middleware


def _make_app() -> FastAPI:
    app = FastAPI()
    add_middleware(app, "test-module")

    @app.get("/ok")
    async def ok():
        return {"status": "ok"}

    @app.get("/fail")
    async def fail():
        raise RuntimeError("boom")

    return app


async def test_request_id_injected():
    app = _make_app()
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/ok")
    assert resp.status_code == 200
    assert "x-request-id" in resp.headers
    assert "x-processing-time-ms" in resp.headers


async def test_request_id_propagated():
    app = _make_app()
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/ok", headers={"x-request-id": "my-custom-id"})
    assert resp.headers["x-request-id"] == "my-custom-id"


async def test_unhandled_exception_wrapped():
    app = _make_app()
    transport = ASGITransport(app=app, raise_app_exceptions=False)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/fail")
    assert resp.status_code == 500
    data = resp.json()
    assert data["error"] == "internal_error"
    assert data["module"] == "test-module"
    assert "x-request-id" in resp.headers
