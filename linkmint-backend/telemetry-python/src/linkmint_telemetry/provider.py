"""Global OpenTelemetry tracer init for Python services (the Go twin is telemetry-go's Init)."""

from __future__ import annotations

import os
from collections.abc import Callable

from opentelemetry import trace
from opentelemetry.baggage.propagation import W3CBaggagePropagator
from opentelemetry.propagate import set_global_textmap
from opentelemetry.propagators.composite import CompositePropagator
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.trace.sampling import ParentBased, TraceIdRatioBased
from opentelemetry.trace.propagation.tracecontext import TraceContextTextMapPropagator

ShutdownFunc = Callable[[], None]


def _noop() -> None:
    return None


def init_telemetry(service_name: str, version: str = "0.0.0") -> ShutdownFunc:
    """Install the global W3C propagator and, when an OTLP endpoint is configured, a batched
    TracerProvider exporting spans over gRPC to Tempo. Returns a shutdown callable (a no-op when
    tracing is off), so callers can unconditionally register it.

    Honored environment (the standard OTel vars):

        OTEL_SDK_DISABLED            truthy -> force a no-op (propagator still installed)
        OTEL_EXPORTER_OTLP_ENDPOINT  OTLP gRPC collector, e.g. http://tempo:4317; empty -> no-op
        OTEL_SERVICE_NAME            overrides the service_name argument
        OTEL_TRACES_SAMPLER_ARG      parent-based ratio in [0,1] (default 1.0 for local dev)
        DEPLOY_ENV                   deployment.environment resource attr (default "local")

    The propagator is installed even when export is off, so a request arriving with a traceparent
    still threads one trace_id through this service's logs; turning on the endpoint later lights up
    export with no code change.
    """
    set_global_textmap(
        CompositePropagator([TraceContextTextMapPropagator(), W3CBaggagePropagator()])
    )

    if _truthy(os.getenv("OTEL_SDK_DISABLED")):
        return _noop
    endpoint = (os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT") or "").strip()
    if not endpoint:
        return _noop

    name = (os.getenv("OTEL_SERVICE_NAME") or service_name).strip()
    resource = Resource.create(
        {
            "service.name": name,
            "service.version": version,
            "deployment.environment": os.getenv("DEPLOY_ENV", "local"),
        }
    )
    provider = TracerProvider(resource=resource, sampler=ParentBased(TraceIdRatioBased(_ratio())))

    # Imported lazily so a process that only needs propagation (e.g. unit tests) does not pull the
    # gRPC exporter wheel at import time.
    from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter

    target, insecure = _grpc_target(endpoint)
    provider.add_span_processor(
        BatchSpanProcessor(OTLPSpanExporter(endpoint=target, insecure=insecure))
    )
    trace.set_tracer_provider(provider)
    return provider.shutdown


def _grpc_target(endpoint: str) -> tuple[str, bool]:
    """Split an endpoint URL into (host:port, insecure), inferring TLS from the scheme: http:// ->
    insecure (the local Tempo default), https:// -> TLS, bare host:port -> insecure."""
    insecure = True
    ep = endpoint
    if ep.startswith("http://"):
        ep, insecure = ep[len("http://") :], True
    elif ep.startswith("https://"):
        ep, insecure = ep[len("https://") :], False
    return ep.rstrip("/"), insecure


def _ratio() -> float:
    raw = (os.getenv("OTEL_TRACES_SAMPLER_ARG") or "").strip()
    if raw:
        try:
            v = float(raw)
        except ValueError:
            return 1.0
        if 0.0 <= v <= 1.0:
            return v
    return 1.0


def _truthy(v: str | None) -> bool:
    return (v or "").strip().lower() in {"1", "true", "yes", "on"}
