from fastapi import FastAPI, Request
from opentelemetry import trace
from prometheus_client import REGISTRY
from starlette.testclient import TestClient

from linkmint_telemetry.asgi import ObservabilityMiddleware


def make_app() -> FastAPI:
    app = FastAPI()

    @app.get("/v1/things/{tid}")
    async def thing(tid: str, request: Request) -> dict[str, str]:
        span = trace.get_current_span()
        return {
            "xrid": request.headers.get("x-request-id", ""),
            "trace": format(span.get_span_context().trace_id, "032x"),
        }

    app.add_middleware(ObservabilityMiddleware, service_name="t", routes=app.routes)
    return app


def test_seeds_request_id_with_trace_id():
    client = TestClient(make_app())
    body = client.get("/v1/things/abc").json()
    assert body["xrid"] == body["trace"]
    assert body["trace"] != "0" * 32


def test_metrics_increment_with_route_template():
    client = TestClient(make_app())
    labels = {"method": "GET", "route": "/v1/things/{tid}", "status": "200"}
    before = REGISTRY.get_sample_value("http_requests_total", labels) or 0.0
    client.get("/v1/things/xyz")
    after = REGISTRY.get_sample_value("http_requests_total", labels) or 0.0
    assert after == before + 1


def test_non_http_scope_passthrough():
    # lifespan scope must pass straight through the middleware (no span/metrics).
    client = TestClient(make_app())
    with client:  # triggers the lifespan scope
        assert client.get("/v1/things/a").status_code == 200
