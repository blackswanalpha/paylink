from opentelemetry import trace

from linkmint_telemetry.celery import inject_trace_headers, worker_span


def test_celery_trace_roundtrip():
    with trace.get_tracer("t").start_as_current_span("producer") as span:
        producer_trace = format(span.get_span_context().trace_id, "032x")
        carrier = inject_trace_headers()
    assert "traceparent" in carrier

    with worker_span("consume", carrier) as wspan:
        worker_trace = format(wspan.get_span_context().trace_id, "032x")
    assert worker_trace == producer_trace


def test_worker_span_without_carrier_starts_fresh():
    with worker_span("consume", None) as wspan:
        assert wspan.get_span_context().trace_id != 0
