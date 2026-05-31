from __future__ import annotations

from fastapi import FastAPI
from fastapi.testclient import TestClient
from pydantic import BaseModel

from app.errors import AppError, ErrorCode, install_error_handlers


def _build() -> FastAPI:
    app = FastAPI()
    install_error_handlers(app)

    class Body(BaseModel):
        n: int

    @app.post("/boom")
    async def boom() -> None:
        raise AppError(ErrorCode.PAYLINK_NOT_FOUND, "nope", details={"x": 1})

    @app.post("/validate")
    async def validate(_b: Body) -> dict[str, str]:
        return {"ok": "yes"}

    @app.get("/crash")
    async def crash() -> None:
        raise RuntimeError("unexpected")

    return app


def test_app_error_uses_envelope() -> None:
    r = TestClient(_build()).post("/boom")
    assert r.status_code == 404
    body = r.json()
    assert set(body["error"].keys()) == {"code", "message", "details", "trace_id"}
    assert body["error"]["code"] == "PAYLINK_NOT_FOUND"
    assert body["error"]["details"] == {"x": 1}


def test_validation_maps_to_invalid_payload() -> None:
    r = TestClient(_build()).post("/validate", json={"n": "not-an-int"})
    assert r.status_code == 400
    assert r.json()["error"]["code"] == "INVALID_PAYLOAD"


def test_unhandled_exception_is_internal_error() -> None:
    client = TestClient(_build(), raise_server_exceptions=False)
    r = client.get("/crash")
    assert r.status_code == 500
    assert r.json()["error"]["code"] == "INTERNAL_ERROR"
