"""Shared fixtures for the gateway acceptance matrix.

Targets a running gateway (booted by `make test` via docker-compose.test.yml, or any stack at
GATEWAY_BASE_URL). Constants default to the test-compose values; override via env to point at
another stack. Tokens are minted with PyJWT (HS256) to match the dev JWT seam.
"""

from __future__ import annotations

import os
import time
from collections.abc import Callable, Iterator
from typing import Any

import httpx
import jwt as pyjwt
import pytest

BASE_URL = os.environ.get("GATEWAY_BASE_URL", "http://localhost:8088")
JWT_SECRET = os.environ.get("GATEWAY_JWT_DEV_SECRET", "test-secret")
JWT_ISSUER = os.environ.get("GATEWAY_JWT_ISSUER", "linkmint-dev")
CREATOR_CLAIM = os.environ.get("GATEWAY_JWT_CREATOR_ADDR_CLAIM", "creator_addr")
PARTNER_KEY = os.environ.get("GATEWAY_PARTNER_API_KEY", "test-partner-key")
PARTNER_ADDR = os.environ.get(
    "GATEWAY_PARTNER_CREATOR_ADDR", "0x00000000000000000000000000000000000000bb"
)
RATE_LIMIT = int(os.environ.get("GATEWAY_RATE_LIMIT_PER_MINUTE", "100"))


@pytest.fixture(scope="session", autouse=True)
def _ready() -> None:
    """Wait until the gateway answers (a 401 on a protected route is the readiness signal)."""
    deadline = time.time() + 60
    last: Any = None
    while time.time() < deadline:
        try:
            r = httpx.get(f"{BASE_URL}/v1/paylinks", timeout=3)
            if r.status_code in (401, 200, 429):
                return
            last = r.status_code
        except Exception as exc:  # noqa: BLE001
            last = repr(exc)
        time.sleep(1)
    raise RuntimeError(f"gateway not ready at {BASE_URL} (last={last})")


@pytest.fixture(scope="session")
def base_url() -> str:
    return BASE_URL


@pytest.fixture
def client() -> Iterator[httpx.Client]:
    with httpx.Client(base_url=BASE_URL, timeout=10) as c:
        yield c


@pytest.fixture
def mint() -> Callable[..., str]:
    def _mint(claims: dict[str, Any] | None = None, exp_delta: int = 3600) -> str:
        payload: dict[str, Any] = {
            "iss": JWT_ISSUER,
            "sub": "user-1",
            "exp": int(time.time()) + exp_delta,
        }
        if claims:
            payload.update(claims)
        return pyjwt.encode(payload, JWT_SECRET, algorithm="HS256")

    return _mint


@pytest.fixture
def valid_token(mint: Callable[..., str]) -> str:
    return mint({CREATOR_CLAIM: "0xAaAaAa0000000000000000000000000000000001"})


@pytest.fixture
def partner_key() -> str:
    return PARTNER_KEY


@pytest.fixture
def partner_addr() -> str:
    return PARTNER_ADDR


@pytest.fixture
def rate_limit() -> int:
    return RATE_LIMIT
