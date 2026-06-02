"""Outbound HTTP trace propagation for httpx."""

from __future__ import annotations

from collections.abc import MutableMapping
from typing import Any

from opentelemetry.propagate import inject


def inject_trace(headers: MutableMapping[str, str]) -> None:
    """Inject W3C trace context (traceparent/baggage) from the active span into a header mapping for
    an outbound call. Use when you build requests by hand."""
    inject(headers)


async def _httpx_request_hook(request: Any) -> None:
    inject(request.headers)


def traced_async_client(**kwargs: Any) -> Any:
    """Return an ``httpx.AsyncClient`` that injects the active trace context into every request, so
    a downstream service continues the same trace. Composes with any caller-supplied event_hooks;
    pass the same kwargs you would to ``httpx.AsyncClient`` (base_url, timeout, transport, ...)."""
    import httpx  # lazy: only services making outbound calls need httpx

    hooks = dict(kwargs.pop("event_hooks", {}) or {})
    request_hooks = list(hooks.get("request", []))
    request_hooks.append(_httpx_request_hook)
    hooks["request"] = request_hooks
    return httpx.AsyncClient(event_hooks=hooks, **kwargs)
