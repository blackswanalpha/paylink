"""Thread the trace across the Celery hop (notification-service web -> worker)."""

from __future__ import annotations

from collections.abc import Iterator, Mapping
from contextlib import contextmanager

from opentelemetry import context, trace
from opentelemetry.propagate import extract, inject
from opentelemetry.trace import Span, SpanKind


def inject_trace_headers() -> dict[str, str]:
    """Capture the active trace context as a carrier dict (e.g. ``{"traceparent": "..."}``) to pass
    into a Celery task's kwargs so the worker can continue the originating request's trace."""
    carrier: dict[str, str] = {}
    inject(carrier)
    return carrier


@contextmanager
def worker_span(name: str, carrier: Mapping[str, str] | None) -> Iterator[Span]:
    """In a Celery worker, continue the trace carried in ``carrier`` (from
    ``inject_trace_headers``) under a new CONSUMER span, making it the active context for the task
    body — so the worker's logs and spans share the originating request's trace_id."""
    ctx = extract(dict(carrier or {}))
    span = trace.get_tracer("telemetry/celery").start_span(
        name, context=ctx, kind=SpanKind.CONSUMER
    )
    token = context.attach(trace.set_span_in_context(span))
    try:
        yield span
    finally:
        context.detach(token)
        span.end()
