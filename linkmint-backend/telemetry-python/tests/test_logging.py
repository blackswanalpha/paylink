from opentelemetry import trace

from linkmint_telemetry.logging import add_otel_ids


def test_adds_ids_in_span():
    with trace.get_tracer("t").start_as_current_span("op"):
        out = add_otel_ids(None, "info", {"event": "x"})
    assert len(out["trace_id"]) == 32
    assert len(out["span_id"]) == 16


def test_no_ids_without_span():
    out = add_otel_ids(None, "info", {"event": "x"})
    assert "span_id" not in out
    assert "trace_id" not in out


def test_does_not_clobber_existing_trace_id():
    with trace.get_tracer("t").start_as_current_span("op"):
        out = add_otel_ids(None, "info", {"event": "x", "trace_id": "preset"})
    assert out["trace_id"] == "preset"
