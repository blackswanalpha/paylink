"""Prometheus HTTP metrics for Python services — the counter set Go services already expose.

Defined on the default registry so they appear automatically on each service's existing ``/metrics``
mount (``prometheus_client.make_asgi_app``), with no per-service wiring. Labels are PII-free and
low-cardinality: the route TEMPLATE (never the raw path), the method, and the status code.
"""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from prometheus_client import Counter, Histogram

HTTP_REQUESTS = Counter(
    "http_requests_total",
    "Total HTTP requests by method, route, and status.",
    ["method", "route", "status"],
)
HTTP_REQUEST_DURATION = Histogram(
    "http_request_duration_seconds",
    "HTTP request duration in seconds by route.",
    ["route"],
)


def route_template(routes: Any, scope: Mapping[str, Any]) -> str:
    """Return the matched route's path TEMPLATE (e.g. ``/v1/things/{id}``) for a request scope, or
    ``"unmatched"`` for an unrouted request (e.g. a 404). Uses the template, never the raw path, so
    an id or phone number in the URL never becomes a metric label or span name."""
    from starlette.routing import Match  # lazy: only web services need starlette

    for route in routes or []:
        try:
            match, _ = route.matches(scope)
        except Exception:  # a malformed route must never block the request
            continue
        if match == Match.FULL:
            path = getattr(route, "path", "")
            return path or "unmatched"
    return "unmatched"
