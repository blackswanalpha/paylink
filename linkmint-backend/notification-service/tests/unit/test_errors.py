"""The standard error envelope + handlers."""

from __future__ import annotations

from fastapi.testclient import TestClient

from app.errors import envelope


def test_envelope_shape() -> None:
    env = envelope("SOME_CODE", "a message", {"k": "v"})
    assert env == {
        "error": {
            "code": "SOME_CODE",
            "message": "a message",
            "details": {"k": "v"},
            "trace_id": "",
        }
    }


def test_unknown_route_is_enveloped_404(client: TestClient) -> None:
    resp = client.get("/no/such/route")
    assert resp.status_code == 404
    body = resp.json()
    assert body["error"]["code"] == "HTTP_404"
    assert "trace_id" in body["error"]
    assert resp.headers.get("x-request-id")  # correlation id echoed


def test_validation_error_is_enveloped_400(client: TestClient) -> None:
    resp = client.post("/v1/notifications", json={"user_id": "u"})  # missing event_kind
    assert resp.status_code == 400
    assert resp.json()["error"]["code"] == "INVALID_PAYLOAD"
