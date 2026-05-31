"""Gateway acceptance matrix — routing, auth, error envelopes, X-Creator-Addr inject/strip,
correlation-id propagation, credential hygiene, and rate limiting.

Runs against a live gateway (see conftest / `make test`). Tests are ordered so the partner-keyed
rate-limit test (which exhausts the partner bucket) runs after the partner-auth tests; JWT tests use
the separate dev-user bucket and stay well under the limit.
"""

from __future__ import annotations

import os
import subprocess
import time
from typing import Any

import httpx
import pytest


def auth(token: str) -> dict[str, str]:
    return {"Authorization": f"Bearer {token}"}


def assert_envelope(body: dict[str, Any], code: str | None = None) -> None:
    assert "error" in body, body
    err = body["error"]
    assert {"code", "message", "details", "trace_id"} <= set(err.keys()), err
    assert isinstance(err["details"], dict)
    assert isinstance(err["message"], str)
    if code is not None:
        assert err["code"] == code, err


# ── Routing ──────────────────────────────────────────────────────────────────────────────────
def test_jwt_routes_to_paylink_service(client: httpx.Client, valid_token: str) -> None:
    r = client.get("/v1/paylinks", headers=auth(valid_token))
    assert r.status_code == 200, r.text
    body = r.json()
    assert body["service"] == "paylink"
    assert body["path"] == "/v1/paylinks"  # strip_path=false → full path forwarded


def test_jwt_routes_to_payment_orchestrator(client: httpx.Client, valid_token: str) -> None:
    r = client.get("/v1/payments/abc123", headers=auth(valid_token))
    assert r.status_code == 200, r.text
    body = r.json()
    assert body["service"] == "payments"
    assert body["path"] == "/v1/payments/abc123"


def test_unknown_route_404_envelope(client: httpx.Client) -> None:
    r = client.get("/nope")  # no auth needed; unknown path → 404
    assert r.status_code == 404
    assert_envelope(r.json(), "NOT_FOUND")


def test_unknown_v1_subpath_404_envelope(client: httpx.Client, valid_token: str) -> None:
    r = client.get("/v1/unknown", headers=auth(valid_token))
    assert r.status_code == 404
    assert_envelope(r.json(), "NOT_FOUND")


# ── Auth: JWT ────────────────────────────────────────────────────────────────────────────────
def test_missing_credentials_401_envelope(client: httpx.Client) -> None:
    r = client.get("/v1/paylinks")
    assert r.status_code == 401
    assert_envelope(r.json(), "UNAUTHORIZED")


def test_malformed_token_401(client: httpx.Client) -> None:
    r = client.get("/v1/paylinks", headers={"Authorization": "Bearer not.a.jwt"})
    assert r.status_code == 401
    assert_envelope(r.json(), "UNAUTHORIZED")


def test_bad_signature_401(client: httpx.Client, mint: Any) -> None:
    good = mint({"creator_addr": "0xabc"})
    tampered = good[:-3] + ("aaa" if not good.endswith("aaa") else "bbb")
    r = client.get("/v1/paylinks", headers=auth(tampered))
    assert r.status_code == 401


def test_expired_token_401(client: httpx.Client, mint: Any) -> None:
    tok = mint({"creator_addr": "0xabc"}, exp_delta=-3600)
    r = client.get("/v1/paylinks", headers=auth(tok))
    assert r.status_code == 401


# ── X-Creator-Addr injection + anti-spoofing ────────────────────────────────────────────────
def test_creator_addr_injected_from_claim(client: httpx.Client, valid_token: str) -> None:
    r = client.get("/v1/paylinks", headers=auth(valid_token))
    assert r.status_code == 200
    headers = r.json()["headers"]
    assert headers.get("x-creator-addr") == "0xaaaaaa0000000000000000000000000000000001"


def test_client_supplied_creator_addr_is_stripped(client: httpx.Client, valid_token: str) -> None:
    spoof = "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
    r = client.get(
        "/v1/paylinks",
        headers={**auth(valid_token), "X-Creator-Addr": spoof, "X-Partner-Id": "evil"},
    )
    assert r.status_code == 200
    headers = r.json()["headers"]
    # the gateway-authoritative value wins; the spoofed one never reaches upstream
    assert headers.get("x-creator-addr") == "0xaaaaaa0000000000000000000000000000000001"
    assert spoof not in (headers.get("x-creator-addr") or "")


def test_credentials_not_forwarded_upstream(client: httpx.Client, valid_token: str) -> None:
    r = client.get("/v1/paylinks", headers=auth(valid_token))
    headers = r.json()["headers"]
    assert "authorization" not in headers
    assert "x-api-key" not in headers


# ── Correlation id ──────────────────────────────────────────────────────────────────────────
def test_correlation_id_echoed_and_propagated(client: httpx.Client, valid_token: str) -> None:
    r = client.get("/v1/paylinks", headers={**auth(valid_token), "X-Request-Id": "corr-abc-123"})
    assert r.status_code == 200
    assert r.headers.get("X-Request-Id") == "corr-abc-123"
    assert r.json()["headers"].get("x-request-id") == "corr-abc-123"


def test_correlation_id_generated_when_absent(client: httpx.Client, valid_token: str) -> None:
    r = client.get("/v1/paylinks", headers=auth(valid_token))
    assert r.headers.get("X-Request-Id")  # gateway generated one
    assert r.json()["headers"].get("x-request-id")  # and forwarded it


# ── Auth: API key (partner). Keep these BEFORE the rate-limit test (shared partner bucket). ──
def test_api_key_passes_and_injects_partner_addr(
    client: httpx.Client, partner_key: str, partner_addr: str
) -> None:
    r = client.get("/v1/paylinks", headers={"X-API-Key": partner_key})
    assert r.status_code == 200, r.text
    assert r.json()["headers"].get("x-creator-addr") == partner_addr.lower()


def test_invalid_api_key_401(client: httpx.Client) -> None:
    r = client.get("/v1/paylinks", headers={"X-API-Key": "definitely-wrong"})
    assert r.status_code == 401
    assert_envelope(r.json(), "UNAUTHORIZED")


# ── Credentials are header-only (no query-string leak into URLs/logs/upstream) ───────────────
def test_jwt_in_query_string_rejected(client: httpx.Client, mint: Any) -> None:
    tok = mint({"creator_addr": "0xabc"})
    r = client.get(f"/v1/paylinks?jwt={tok}")  # no Authorization header
    assert r.status_code == 401
    assert_envelope(r.json(), "UNAUTHORIZED")


def test_api_key_in_query_string_rejected(client: httpx.Client, partner_key: str) -> None:
    r = client.get(f"/v1/paylinks?X-API-Key={partner_key}")  # no X-API-Key header
    assert r.status_code == 401
    assert_envelope(r.json(), "UNAUTHORIZED")


# ── Rate limiting (partner bucket; runs last so it doesn't starve the partner-auth tests) ────
def test_rate_limit_429_with_retry_after(
    base_url: str, partner_key: str, rate_limit: int
) -> None:
    seen_429 = False
    retry_after: str | None = None
    with httpx.Client(base_url=base_url, timeout=10) as c:
        for _ in range(rate_limit + 10):
            r = c.get("/v1/paylinks", headers={"X-API-Key": partner_key})
            if r.status_code == 429:
                seen_429 = True
                retry_after = r.headers.get("Retry-After")
                assert_envelope(r.json(), "RATE_LIMITED")
                break
    assert seen_429, "expected a 429 within limit+10 requests"
    assert retry_after is not None and int(retry_after) >= 0


# ── Upstream down → 502 (opt-in: needs docker control of the test stack). ────────────────────
@pytest.mark.skipif(
    not os.environ.get("GATEWAY_TEST_COMPOSE"),
    reason="set GATEWAY_TEST_COMPOSE=<path to docker-compose.test.yml> to run the 502 test",
)
def test_upstream_down_502(client: httpx.Client, valid_token: str) -> None:
    compose = os.environ["GATEWAY_TEST_COMPOSE"]
    base = ["docker", "compose", "-f", compose]
    subprocess.run([*base, "stop", "echo-paylink"], check=True)
    try:
        time.sleep(1)
        r = client.get("/v1/paylinks", headers=auth(valid_token))
        # connection refused → 502; DNS/connect timeout on a stopped container → 504.
        assert r.status_code in (502, 503, 504)
        assert_envelope(r.json())
        assert r.json()["error"]["code"] in (
            "BAD_GATEWAY",
            "SERVICE_UNAVAILABLE",
            "UPSTREAM_TIMEOUT",
        )
    finally:
        subprocess.run([*base, "start", "echo-paylink"], check=True)
        time.sleep(3)
