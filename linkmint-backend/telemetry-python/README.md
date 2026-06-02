# telemetry-python

LinkMint's shared OpenTelemetry helper for Python/FastAPI services (work18). The Python twin of
[`telemetry-go`](../telemetry-go); both speak **W3C trace context** so one `trace_id` follows a
request across HTTP calls and the Kafka event bus, in either language.

## What it gives a service

- `init_telemetry(service_name, version)` — installs the global W3C propagator and (when
  `OTEL_EXPORTER_OTLP_ENDPOINT` is set) a batched `TracerProvider` exporting spans over OTLP/gRPC to
  Tempo. Returns a shutdown callable. **No-op until an endpoint is configured.**
- `ObservabilityMiddleware` — one pure-ASGI middleware that: extracts inbound trace context, starts a
  server span, records the Prometheus `http_requests_total` / `http_request_duration_seconds` set
  (the same Go services already expose), and **seeds `X-Request-Id` with the 32-hex trace id** so the
  service's existing `RequestIdMiddleware` adopts it — unifying `trace_id` across logs, the error
  envelope, the response header, and the trace in Tempo.
- `add_otel_ids` — a structlog processor that adds `trace_id`/`span_id` from the active span.
- `traced_async_client(**kwargs)` / `inject_trace(headers)` — outbound httpx trace propagation.
- `inject_trace_headers()` / `worker_span(name, carrier)` — thread the trace across the Celery hop.

## Wiring (FastAPI)

```python
from linkmint_telemetry import ObservabilityMiddleware, init_telemetry

# in the lifespan startup:
shutdown = init_telemetry(settings.service_name, version="0.1.0")
# add LAST so it wraps RequestIdMiddleware (Starlette applies the last-added middleware outermost):
app.add_middleware(ObservabilityMiddleware, service_name=settings.service_name, routes=app.routes)
```

Append `add_otel_ids` to the structlog processor chain in `app/logging.py` for span-level log
correlation.

## Invariant

Spans, metric labels, and the unified correlation id carry only **low-cardinality, PII-free** data:
the route TEMPLATE (never the raw path), the HTTP method, the status code. Never attach request
bodies, query strings, auth headers, or rail identifiers.

## Test

```bash
pip install -e ".[dev]" && pytest   # ruff/black/mypy via `make lint`; 80% coverage gate
```
