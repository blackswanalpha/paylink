from __future__ import annotations

from fastapi import FastAPI
from fastapi.testclient import TestClient

from app.errors import AppError, ErrorCode, envelope, install_error_handlers


def _app() -> FastAPI:
    app = FastAPI()
    install_error_handlers(app)

    @app.get("/boom")
    async def boom() -> None:
        raise AppError(ErrorCode.EMAIL_TAKEN, "taken", details={"x": 1})

    @app.get("/unexpected")
    async def unexpected() -> None:
        raise RuntimeError("nope")

    return app


def _client() -> TestClient:
    return TestClient(_app(), raise_server_exceptions=False)


def test_app_error_envelope() -> None:
    r = _client().get("/boom")
    assert r.status_code == 409
    err = r.json()["error"]
    assert err["code"] == "EMAIL_TAKEN"
    assert err["message"] == "taken"
    assert err["details"] == {"x": 1}
    assert "trace_id" in err


def test_unhandled_is_500_envelope() -> None:
    r = _client().get("/unexpected")
    assert r.status_code == 500
    assert r.json()["error"]["code"] == "INTERNAL_ERROR"


def test_unknown_route_is_envelope() -> None:
    r = _client().get("/does-not-exist")
    assert r.status_code == 404
    assert r.json()["error"]["code"].startswith("HTTP_")


def test_envelope_helper_shape() -> None:
    e = envelope("CODE", "msg", {"k": "v"})
    assert e["error"]["code"] == "CODE"
    assert e["error"]["details"] == {"k": "v"}
    assert "trace_id" in e["error"]
