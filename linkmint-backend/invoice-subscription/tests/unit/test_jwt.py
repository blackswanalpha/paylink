"""RS256 verifier seam — only valid identity tokens are accepted."""

from __future__ import annotations

import uuid

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from fastapi.testclient import TestClient

from tests._support import merchant_headers, mint_token


def _path() -> str:
    return f"/v1/invoices/{uuid.uuid4()}"


def test_missing_authorization(client: TestClient) -> None:
    assert client.get(_path()).status_code == 401


def test_malformed_authorization(client: TestClient) -> None:
    r = client.get(_path(), headers={"Authorization": "Token abc"})
    assert r.status_code == 401


def test_expired_token(client: TestClient) -> None:
    tok = mint_token(user_id=str(uuid.uuid4()), expired=True)
    r = client.get(_path(), headers={"Authorization": f"Bearer {tok}"})
    assert r.status_code == 401
    assert r.json()["error"]["code"] == "TOKEN_EXPIRED"


def test_bad_signature(client: TestClient) -> None:
    other = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    pem = other.private_bytes(
        serialization.Encoding.PEM,
        serialization.PrivateFormat.PKCS8,
        serialization.NoEncryption(),
    ).decode()
    tok = mint_token(user_id=str(uuid.uuid4()), private_pem=pem)
    r = client.get(_path(), headers={"Authorization": f"Bearer {tok}"})
    assert r.status_code == 401
    assert r.json()["error"]["code"] == "INVALID_TOKEN"


def test_valid_token_passes_auth(client: TestClient) -> None:
    # A valid token reaches the handler (then 404 because the invoice doesn't exist) — proving auth ok.
    uid = str(uuid.uuid4())
    r = client.get(_path(), headers=merchant_headers(uid))
    assert r.status_code == 404
