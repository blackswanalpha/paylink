"""Unit tests for the compliance-risk client (httpx MockTransport — no network, no Docker)."""

from __future__ import annotations

import json
from collections.abc import Callable

import httpx
import pytest

from app.compliance.client import ComplianceClient, ComplianceUnavailable

Handler = Callable[[httpx.Request], httpx.Response]


def _client(handler: Handler) -> ComplianceClient:
    http = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    return ComplianceClient("http://compliance:8093/", http, internal_token="tok", timeout=2.0)


async def test_evaluate_parses_decision_and_sends_token() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.url.path == "/v1/risk/evaluate"
        assert request.headers["X-Internal-Token"] == "tok"
        body = json.loads(request.content)
        assert body == {
            "user_id": "u1",
            "action": "paylink.create",
            "amount": 5000,
            "currency": "KES",
            "context": "paylink.create:PLK1",
        }
        return httpx.Response(
            200,
            json={
                "decision": "block",
                "score": 1.0,
                "reasons": [{"code": "AML_THRESHOLD", "detail": "x"}],
            },
        )

    decision = await _client(handler).evaluate(
        user_id="u1",
        action="paylink.create",
        amount=5000,
        currency="KES",
        context="paylink.create:PLK1",
    )
    assert decision.decision == "block"
    assert decision.score == 1.0
    assert decision.reasons[0]["code"] == "AML_THRESHOLD"


async def test_evaluate_omits_none_fields_and_token() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        assert "amount" not in body and "geo" not in body and "registered_country" not in body
        assert "x-internal-token" not in {k.lower() for k in request.headers}
        return httpx.Response(200, json={"decision": "allow", "score": 0.0, "reasons": []})

    http = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    decision = await ComplianceClient("http://compliance:8093", http).evaluate(
        user_id="u1", action="paylink.create"
    )
    assert decision.decision == "allow"


async def test_http_error_status_raises_unavailable() -> None:
    with pytest.raises(ComplianceUnavailable):
        await _client(lambda _r: httpx.Response(503, text="down")).evaluate(
            user_id="u1", action="paylink.create"
        )


async def test_transport_error_raises_unavailable() -> None:
    def handler(_request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("boom")

    with pytest.raises(ComplianceUnavailable):
        await _client(handler).evaluate(user_id="u1", action="paylink.create")


async def test_bad_json_raises_unavailable() -> None:
    with pytest.raises(ComplianceUnavailable):
        await _client(lambda _r: httpx.Response(200, text="not-json")).evaluate(
            user_id="u1", action="paylink.create"
        )


async def test_missing_decision_key_raises_unavailable() -> None:
    with pytest.raises(ComplianceUnavailable):
        await _client(lambda _r: httpx.Response(200, json={"score": 0.1})).evaluate(
            user_id="u1", action="paylink.create"
        )
