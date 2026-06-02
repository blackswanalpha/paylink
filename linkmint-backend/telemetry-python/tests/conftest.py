"""Install a real (always-sampling) TracerProvider + the W3C propagator once per session so spans
have valid trace ids in-process (the global no-op provider yields zero ids)."""

import pytest
from opentelemetry import trace
from opentelemetry.baggage.propagation import W3CBaggagePropagator
from opentelemetry.propagate import set_global_textmap
from opentelemetry.propagators.composite import CompositePropagator
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.sampling import ALWAYS_ON
from opentelemetry.trace.propagation.tracecontext import TraceContextTextMapPropagator


@pytest.fixture(scope="session", autouse=True)
def _otel():
    set_global_textmap(
        CompositePropagator([TraceContextTextMapPropagator(), W3CBaggagePropagator()])
    )
    trace.set_tracer_provider(TracerProvider(sampler=ALWAYS_ON))
    yield
