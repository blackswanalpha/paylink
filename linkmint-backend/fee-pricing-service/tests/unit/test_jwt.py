"""RS256 verifier seam — only valid identity tokens are accepted; HS256/none rejected."""

from __future__ import annotations

import jwt as pyjwt
import pytest
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from fastapi.testclient import TestClient

from app.errors import AppError, ErrorCode
from app.security.jwt import JwtVerifier
from tests._support import (
    AUDIENCE,
    ISSUER,
    TEST_PUBLIC_PEM,
    auth_headers,
    mint_token,
)

PROTECTED = "/v1/fx/rates"


def test_missing_authorization(client: TestClient) -> None:
    assert client.get(PROTECTED).status_code == 401


def test_malformed_authorization(client: TestClient) -> None:
    assert client.get(PROTECTED, headers={"Authorization": "Token abc"}).status_code == 401


def test_expired_token(client: TestClient) -> None:
    r = client.get(PROTECTED, headers={"Authorization": f"Bearer {mint_token(expired=True)}"})
    assert r.status_code == 401
    assert r.json()["error"]["code"] == "TOKEN_EXPIRED"


def test_bad_signature(client: TestClient) -> None:
    other = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    pem = other.private_bytes(
        serialization.Encoding.PEM,
        serialization.PrivateFormat.PKCS8,
        serialization.NoEncryption(),
    ).decode()
    r = client.get(PROTECTED, headers={"Authorization": f"Bearer {mint_token(private_pem=pem)}"})
    assert r.status_code == 401
    assert r.json()["error"]["code"] == "INVALID_TOKEN"


def test_valid_token_passes(client: TestClient) -> None:
    assert client.get(PROTECTED, headers=auth_headers()).status_code == 200


def test_verifier_rejects_hs256_alg_confusion() -> None:
    verifier = JwtVerifier(TEST_PUBLIC_PEM, issuer=ISSUER, audience=AUDIENCE)
    forged = pyjwt.encode(
        {"sub": "x", "iss": ISSUER, "aud": AUDIENCE}, "shared-secret", algorithm="HS256"
    )
    with pytest.raises(AppError) as exc:
        verifier.verify(forged)
    assert exc.value.code == ErrorCode.INVALID_TOKEN


def test_verifier_parses_roles_and_user_roles() -> None:
    verifier = JwtVerifier(TEST_PUBLIC_PEM, issuer=ISSUER, audience=AUDIENCE)
    tok = mint_token(roles=[{"org_id": "o1", "role": "admin"}], user_roles=["admin"])
    claims = verifier.verify(tok)
    assert claims.roles[0].org_id == "o1"
    assert claims.roles[0].role == "admin"
    assert claims.user_roles == ["admin"]
