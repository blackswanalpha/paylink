"""Pure-ASGI observability middleware: server span + HTTP metrics + trace_id unification."""

from __future__ import annotations

import time
from collections.abc import Awaitable, Callable, MutableMapping
from typing import Any

from opentelemetry import context, trace
from opentelemetry.propagate import extract
from opentelemetry.trace import SpanKind

from .metrics import HTTP_REQUEST_DURATION, HTTP_REQUESTS, route_template

Receive = Callable[[], Awaitable[MutableMapping[str, Any]]]
Send = Callable[[MutableMapping[str, Any]], Awaitable[None]]
ASGIApp = Callable[[MutableMapping[str, Any], Receive, Send], Awaitable[None]]


class ObservabilityMiddleware:
    """Extracts W3C trace context, starts a server span, records the Prometheus HTTP metrics, and
    seeds ``X-Request-Id`` with the 32-hex trace id so the service's existing RequestIdMiddleware
    adopts it — unifying ``trace_id`` across logs, the error envelope, the response header, and the
    trace in Tempo.

    Add it LAST in ``main.py`` so it wraps the RequestIdMiddleware (Starlette applies the last-added
    middleware outermost). Pass ``routes=app.routes`` so the span/metric route label is the matched
    template, never the raw path. When tracing is off the tracer is a no-op: the span context is
    invalid, so X-Request-Id is left untouched and behavior is unchanged.
    """

    def __init__(self, app: ASGIApp, service_name: str = "service", routes: Any = None) -> None:
        self.app = app
        self.service_name = service_name
        self.routes = routes

    async def __call__(self, scope: MutableMapping[str, Any], receive: Receive, send: Send) -> None:
        if scope.get("type") != "http":
            await self.app(scope, receive, send)
            return

        carrier = {
            k.decode("latin-1").lower(): v.decode("latin-1") for k, v in scope.get("headers", [])
        }
        ctx = extract(carrier)
        method = scope.get("method", "GET")
        span = trace.get_tracer("telemetry/asgi").start_span(
            method,
            context=ctx,
            kind=SpanKind.SERVER,
            attributes={"service.name": self.service_name},
        )

        sc = span.get_span_context()
        if sc is not None and sc.trace_id != 0:
            _set_header(scope, "x-request-id", format(sc.trace_id, "032x"))

        token = context.attach(trace.set_span_in_context(span))
        status = {"code": 500}
        start = time.perf_counter()

        async def send_wrapper(message: MutableMapping[str, Any]) -> None:
            if message["type"] == "http.response.start":
                status["code"] = int(message["status"])
            await send(message)

        try:
            await self.app(scope, receive, send_wrapper)
        finally:
            duration = time.perf_counter() - start
            route = route_template(self.routes, scope)
            span.update_name(f"{method} {route}")
            span.set_attribute("http.request.method", method)
            span.set_attribute("http.route", route)
            span.set_attribute("http.response.status_code", status["code"])
            span.end()
            context.detach(token)
            HTTP_REQUESTS.labels(method, route, str(status["code"])).inc()
            HTTP_REQUEST_DURATION.labels(route).observe(duration)


def _set_header(scope: MutableMapping[str, Any], name: str, value: str) -> None:
    """Replace (or add) a header in the ASGI scope before the inner app reads it."""
    lname = name.lower().encode("latin-1")
    raw = [(k, v) for (k, v) in scope.get("headers", []) if k.lower() != lname]
    raw.append((name.encode("latin-1"), value.encode("latin-1")))
    scope["headers"] = raw
