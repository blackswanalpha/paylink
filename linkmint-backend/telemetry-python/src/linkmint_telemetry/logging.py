"""structlog processor that correlates a service's logs with its traces."""

from __future__ import annotations

from typing import Any

from opentelemetry import trace


def add_otel_ids(_logger: Any, _method: str, event_dict: dict[str, Any]) -> dict[str, Any]:
    """Stamp ``trace_id``/``span_id`` from the active span onto a structlog event. Append it to a
    service's existing processor chain (after its ``_add_trace_id``). ``setdefault`` on trace_id so
    it never clobbers the id the service's RequestIdMiddleware already set — which, once the
    ObservabilityMiddleware has seeded X-Request-Id, is the same OTel trace id. Adds the span_id for
    span-level log correlation."""
    sc = trace.get_current_span().get_span_context()
    if sc is not None and sc.trace_id != 0:
        event_dict.setdefault("trace_id", format(sc.trace_id, "032x"))
        event_dict["span_id"] = format(sc.span_id, "016x")
    return event_dict
