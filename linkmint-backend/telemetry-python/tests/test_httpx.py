import httpx
import pytest
from opentelemetry import trace
from starlette.applications import Starlette
from starlette.responses import JSONResponse
from starlette.routing import Route

from linkmint_telemetry.httpx_client import inject_trace, traced_async_client


def test_inject_trace_in_span():
    carrier: dict[str, str] = {}
    with trace.get_tracer("t").start_as_current_span("op"):
        inject_trace(carrier)
    assert "traceparent" in carrier


@pytest.mark.asyncio
async def test_traced_client_injects_traceparent():
    async def echo(request):
        return JSONResponse({"tp": request.headers.get("traceparent")})

    app = Starlette(routes=[Route("/", echo)])
    transport = httpx.ASGITransport(app=app)
    async with traced_async_client(transport=transport, base_url="http://test") as client:
        with trace.get_tracer("t").start_as_current_span("op"):
            resp = await client.get("/")
    assert resp.json()["tp"]
