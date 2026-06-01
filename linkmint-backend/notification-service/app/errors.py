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
from starlette.exceptions import HTTPException as StarletteHTTPException

from app.logging import get_logger, trace_id_var

log = get_logger("notify.errors")


class ErrorCode(StrEnum):
    INVALID_PAYLOAD = "INVALID_PAYLOAD"
    INVALID_QUERY = "INVALID_QUERY"
    IDEMPOTENT_CONFLICT = "IDEMPOTENT_CONFLICT"
    DELIVERY_NOT_FOUND = "DELIVERY_NOT_FOUND"
    TEMPLATE_NOT_FOUND = "TEMPLATE_NOT_FOUND"
    UNAUTHORIZED = "UNAUTHORIZED"
    INTERNAL_ERROR = "INTERNAL_ERROR"


_HTTP_STATUS: dict[ErrorCode, int] = {
    ErrorCode.INVALID_PAYLOAD: 400,
    ErrorCode.INVALID_QUERY: 400,
    ErrorCode.IDEMPOTENT_CONFLICT: 409,
    ErrorCode.DELIVERY_NOT_FOUND: 404,
    ErrorCode.TEMPLATE_NOT_FOUND: 422,
    ErrorCode.UNAUTHORIZED: 401,
    ErrorCode.INTERNAL_ERROR: 500,
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
