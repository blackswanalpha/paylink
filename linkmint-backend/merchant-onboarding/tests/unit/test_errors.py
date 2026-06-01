from __future__ import annotations

from fastapi import FastAPI
from fastapi.testclient import TestClient

from app.errors import AppError, ErrorCode, envelope, install_error_handlers


def _app() -> FastAPI:
    app = FastAPI()
    install_error_handlers(app)

    @app.get("/boom")
    async def boom() -> None:
        raise AppError(ErrorCode.ALREADY_ONBOARDED, "dup", details={"x": 1})

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
    assert err["code"] == "ALREADY_ONBOARDED"
    assert err["message"] == "dup"
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


def test_status_map_for_domain_codes() -> None:
    assert AppError(ErrorCode.PAYLOAD_TOO_LARGE, "x").http_status == 413
    assert AppError(ErrorCode.INVALID_ACCOUNT, "x").http_status == 422
    assert AppError(ErrorCode.MERCHANT_NOT_FOUND, "x").http_status == 404
    assert AppError(ErrorCode.UNSUPPORTED_COUNTRY, "x").http_status == 400
    assert AppError(ErrorCode.INVALID_TRANSITION, "x").http_status == 409
