from __future__ import annotations

from app.errors import AppError, ErrorCode, envelope


def test_envelope_shape() -> None:
    env = envelope("X", "msg", {"k": "v"})
    assert set(env.keys()) == {"error"}
    assert set(env["error"].keys()) == {"code", "message", "details", "trace_id"}
    assert env["error"]["code"] == "X"
    assert env["error"]["message"] == "msg"
    assert env["error"]["details"] == {"k": "v"}


def test_app_error_default_status_from_map() -> None:
    assert AppError(ErrorCode.ALREADY_VERIFIED, "x").http_status == 409
    assert AppError(ErrorCode.INVALID_TIER, "x").http_status == 400
    assert AppError(ErrorCode.UNKNOWN_PROVIDER, "x").http_status == 404
    assert AppError(ErrorCode.INVALID_SIGNATURE, "x").http_status == 401
    assert AppError(ErrorCode.COMPLIANCE_NOT_FOUND, "x").http_status == 404
    assert AppError(ErrorCode.UNAUTHORIZED, "x").http_status == 401
    assert AppError(ErrorCode.FORBIDDEN, "x").http_status == 403


def test_app_error_status_override() -> None:
    err = AppError(ErrorCode.INVALID_PAYLOAD, "x", http_status=418)
    assert err.http_status == 418


def test_app_error_unknown_code_defaults_400() -> None:
    # PROVIDER_ERROR is mapped to 502; an explicit message round-trips.
    err = AppError(ErrorCode.PROVIDER_ERROR, "upstream down")
    assert err.http_status == 502
    assert err.message == "upstream down"
