"""Structured JSON logging (structlog) with a per-request correlation id.

The correlation id is taken from the inbound ``X-Request-Id`` header (or generated), exposed as
``trace_id`` on every log line and echoed back on the response. ``trace_id`` is also the value
placed in the error envelope, so handlers read it from :data:`trace_id_var`.

The Celery worker has no HTTP request scope, so it logs without a ``trace_id`` (or with one threaded
through the task payload in a future iteration) — the same JSON pipeline is shared via
:func:`configure_logging`.
"""

from __future__ import annotations

import logging
import uuid
from contextvars import ContextVar
from typing import Any

import structlog
from starlette.requests import Request
from starlette.types import ASGIApp
from structlog.typing import EventDict, WrappedLogger

trace_id_var: ContextVar[str] = ContextVar("trace_id", default="")


def _add_trace_id(_logger: WrappedLogger, _method: str, event_dict: EventDict) -> EventDict:
    tid = trace_id_var.get()
    if tid:
        event_dict["trace_id"] = tid
    return event_dict


def configure_logging(level: str, service_name: str) -> None:
    """Install the structlog JSON pipeline. Idempotent."""
    level_no = getattr(logging, level.upper(), logging.INFO)

    def _add_service(_logger: WrappedLogger, _method: str, event_dict: EventDict) -> EventDict:
        event_dict.setdefault("service", service_name)
        return event_dict

    structlog.configure(
        processors=[
            structlog.contextvars.merge_contextvars,
            structlog.processors.add_log_level,
            _add_service,
            _add_trace_id,
            structlog.processors.TimeStamper(fmt="iso", utc=True),
            structlog.processors.StackInfoRenderer(),
            structlog.processors.format_exc_info,
            structlog.processors.JSONRenderer(),
        ],
        wrapper_class=structlog.make_filtering_bound_logger(level_no),
        logger_factory=structlog.PrintLoggerFactory(),
        cache_logger_on_first_use=True,
    )


def get_logger(name: str = "notify") -> Any:
    return structlog.get_logger(name)


class RequestIdMiddleware:
    """Pure-ASGI middleware: bind a correlation id for the request scope."""

    def __init__(self, app: ASGIApp) -> None:
        self.app = app

    async def __call__(self, scope: Any, receive: Any, send: Any) -> None:
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return

        request = Request(scope, receive=receive)
        rid = request.headers.get("X-Request-Id") or uuid.uuid4().hex
        token = trace_id_var.set(rid)

        async def send_with_header(message: Any) -> None:
            if message["type"] == "http.response.start":
                headers = message.setdefault("headers", [])
                headers.append((b"x-request-id", rid.encode()))
            await send(message)

        try:
            await self.app(scope, receive, send_with_header)
        finally:
            trace_id_var.reset(token)
