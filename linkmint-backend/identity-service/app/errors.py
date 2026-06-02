"""Domain error codes + the standard LinkMint error envelope.

Every error response is exactly::

    {"error": {"code": "...", "message": "...", "details": {}, "trace_id": "..."}}
"""

from __future__ import annotations

from enum import StrEnum
from typing import Any

from fastapi import FastAPI, Request
from fastapi.encoders import jsonable_encoder
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from linkmint_idempotency import IdempotencyConflict
from starlette.exceptions import HTTPException as StarletteHTTPException

from app.logging import get_logger, trace_id_var

log = get_logger("identity.errors")


class ErrorCode(StrEnum):
    # Generic / framework
    INVALID_PAYLOAD = "INVALID_PAYLOAD"
    INVALID_QUERY = "INVALID_QUERY"
    IDEMPOTENT_CONFLICT = "IDEMPOTENT_CONFLICT"
    UNAUTHORIZED = "UNAUTHORIZED"
    FORBIDDEN = "FORBIDDEN"
    INTERNAL_ERROR = "INTERNAL_ERROR"

    # Registration / users
    EMAIL_TAKEN = "EMAIL_TAKEN"
    PHONE_TAKEN = "PHONE_TAKEN"
    USER_NOT_FOUND = "USER_NOT_FOUND"
    USER_SUSPENDED = "USER_SUSPENDED"

    # Auth / tokens
    INVALID_CREDENTIALS = "INVALID_CREDENTIALS"
    INVALID_TOKEN = "INVALID_TOKEN"
    TOKEN_EXPIRED = "TOKEN_EXPIRED"

    # MFA
    MFA_REQUIRED = "MFA_REQUIRED"
    MFA_INVALID = "MFA_INVALID"
    MFA_NOT_ENROLLED = "MFA_NOT_ENROLLED"
    MFA_ALREADY_ENROLLED = "MFA_ALREADY_ENROLLED"

    # Organizations / membership
    ORG_NOT_FOUND = "ORG_NOT_FOUND"
    MEMBER_NOT_FOUND = "MEMBER_NOT_FOUND"
    MEMBER_EXISTS = "MEMBER_EXISTS"
    CANNOT_REMOVE_LAST_OWNER = "CANNOT_REMOVE_LAST_OWNER"

    # API keys
    API_KEY_NOT_FOUND = "API_KEY_NOT_FOUND"
    SCOPE_NOT_ALLOWED = "SCOPE_NOT_ALLOWED"

    # Sessions
    SESSION_NOT_FOUND = "SESSION_NOT_FOUND"

    # OAuth
    OAUTH_PROVIDER_UNKNOWN = "OAUTH_PROVIDER_UNKNOWN"
    OAUTH_EXCHANGE_FAILED = "OAUTH_EXCHANGE_FAILED"

    # Reserved KYC seam (consumed once compliance/work12 lands)
    KYC_GATE = "KYC_GATE"


_HTTP_STATUS: dict[ErrorCode, int] = {
    ErrorCode.INVALID_PAYLOAD: 400,
    ErrorCode.INVALID_QUERY: 400,
    ErrorCode.IDEMPOTENT_CONFLICT: 409,
    ErrorCode.UNAUTHORIZED: 401,
    ErrorCode.FORBIDDEN: 403,
    ErrorCode.INTERNAL_ERROR: 500,
    ErrorCode.EMAIL_TAKEN: 409,
    ErrorCode.PHONE_TAKEN: 409,
    ErrorCode.USER_NOT_FOUND: 404,
    ErrorCode.USER_SUSPENDED: 403,
    ErrorCode.INVALID_CREDENTIALS: 401,
    ErrorCode.INVALID_TOKEN: 401,
    ErrorCode.TOKEN_EXPIRED: 401,
    ErrorCode.MFA_REQUIRED: 401,
    ErrorCode.MFA_INVALID: 401,
    ErrorCode.MFA_NOT_ENROLLED: 404,
    ErrorCode.MFA_ALREADY_ENROLLED: 409,
    ErrorCode.ORG_NOT_FOUND: 404,
    ErrorCode.MEMBER_NOT_FOUND: 404,
    ErrorCode.MEMBER_EXISTS: 409,
    ErrorCode.CANNOT_REMOVE_LAST_OWNER: 409,
    ErrorCode.API_KEY_NOT_FOUND: 404,
    ErrorCode.SCOPE_NOT_ALLOWED: 403,
    ErrorCode.SESSION_NOT_FOUND: 404,
    ErrorCode.OAUTH_PROVIDER_UNKNOWN: 400,
    ErrorCode.OAUTH_EXCHANGE_FAILED: 401,
    ErrorCode.KYC_GATE: 402,
}


class AppError(Exception):
    """A domain error that serializes to the standard envelope."""

    def __init__(
        self,
        code: ErrorCode,
        message: str,
        *,
        details: dict[str, Any] | None = None,
        http_status: int | None = None,
    ) -> None:
        self.code = code
        self.message = message
        self.details = details or {}
        self.http_status = http_status if http_status is not None else _HTTP_STATUS.get(code, 400)
        super().__init__(message)


def envelope(code: str, message: str, details: dict[str, Any] | None = None) -> dict[str, Any]:
    return {
        "error": {
            "code": code,
            "message": message,
            "details": details or {},
            "trace_id": trace_id_var.get(),
        }
    }


def install_error_handlers(app: FastAPI) -> None:
    @app.exception_handler(AppError)
    async def _handle_app_error(_request: Request, exc: AppError) -> JSONResponse:
        if exc.http_status >= 500:
            log.error("app_error", code=exc.code.value, message=exc.message, details=exc.details)
        return JSONResponse(
            status_code=exc.http_status,
            content=envelope(exc.code.value, exc.message, exc.details),
        )

    @app.exception_handler(IdempotencyConflict)
    async def _handle_idempotency_conflict(
        _request: Request, exc: IdempotencyConflict
    ) -> JSONResponse:
        # The shared idempotency lib (work17) is transport-free; map its conflict to our envelope.
        return JSONResponse(
            status_code=_HTTP_STATUS[ErrorCode.IDEMPOTENT_CONFLICT],
            content=envelope(ErrorCode.IDEMPOTENT_CONFLICT.value, str(exc)),
        )

    @app.exception_handler(RequestValidationError)
    async def _handle_validation(_request: Request, exc: RequestValidationError) -> JSONResponse:
        return JSONResponse(
            status_code=_HTTP_STATUS[ErrorCode.INVALID_PAYLOAD],
            content=envelope(
                ErrorCode.INVALID_PAYLOAD.value,
                "request validation failed",
                {"errors": jsonable_encoder(exc.errors())},
            ),
        )

    @app.exception_handler(StarletteHTTPException)
    async def _handle_http(_request: Request, exc: StarletteHTTPException) -> JSONResponse:
        # Framework-level HTTP errors (404 route, 405, ...) — keep the envelope shape.
        code = f"HTTP_{exc.status_code}"
        message = exc.detail if isinstance(exc.detail, str) else "http error"
        return JSONResponse(status_code=exc.status_code, content=envelope(code, message))

    @app.exception_handler(Exception)
    async def _handle_unexpected(_request: Request, exc: Exception) -> JSONResponse:
        log.error("unhandled_exception", error=str(exc), error_type=type(exc).__name__)
        return JSONResponse(
            status_code=500,
            content=envelope(ErrorCode.INTERNAL_ERROR.value, "internal error"),
        )
