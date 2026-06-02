"""LinkMint shared OpenTelemetry helper for Python services (work18).

The Python half of the telemetry-go / telemetry-python pair. Typical wiring in a FastAPI service::

    from linkmint_telemetry import ObservabilityMiddleware, init_telemetry, add_otel_ids

    shutdown = init_telemetry(settings.service_name, version)   # lifespan startup
    app.add_middleware(
        ObservabilityMiddleware, service_name=settings.service_name, routes=app.routes
    )
    # append add_otel_ids to the structlog processor chain in app/logging.py

Tracing is a no-op until OTEL_EXPORTER_OTLP_ENDPOINT is set, so importing this never changes
behavior. Invariant: spans, metric labels, and the unified correlation id carry only low-cardinality
PII-free data (the route TEMPLATE, method, status) — never bodies, query strings, or rail ids.
"""

from .asgi import ObservabilityMiddleware
from .celery import inject_trace_headers, worker_span
from .httpx_client import inject_trace, traced_async_client
from .logging import add_otel_ids
from .metrics import HTTP_REQUEST_DURATION, HTTP_REQUESTS
from .provider import init_telemetry

__all__ = [
    "ObservabilityMiddleware",
    "init_telemetry",
    "add_otel_ids",
    "inject_trace",
    "traced_async_client",
    "inject_trace_headers",
    "worker_span",
    "HTTP_REQUESTS",
    "HTTP_REQUEST_DURATION",
]
