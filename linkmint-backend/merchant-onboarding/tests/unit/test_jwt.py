from __future__ import annotations

import base64
import hashlib
import hmac
import json

import pytest

from app.errors import AppError, ErrorCode
from app.security.jwt import JwtVerifier
from tests._support import AUDIENCE, ISSUER, TEST_PUBLIC_PEM, mint_token


def _verifier(issuer: str = ISSUER, audience: str = AUDIENCE) -> JwtVerifier:
    return JwtVerifier(TEST_PUBLIC_PEM, issuer=issuer, audience=audience)


def test_verify_roundtrip() -> None:
    token = mint_token(user_id="u1", roles=[("o1", "owner")], user_roles=["payer"], kyc_tier=2)
    claims = _verifier().verify(token)
    assert claims.user_id == "u1"
    assert claims.kyc_tier == 2
    assert claims.roles[0].org_id == "o1" and claims.roles[0].role == "owner"
    assert "payer" in claims.user_roles


def test_expired_token_raises() -> None:
    with pytest.raises(AppError) as exc:
        _verifier().verify(mint_token(expired=True))
    assert exc.value.code == ErrorCode.TOKEN_EXPIRED


def test_wrong_audience_rejected() -> None:
    with pytest.raises(AppError) as exc:
        _verifier(audience="WRONG").verify(mint_token())
    assert exc.value.code == ErrorCode.INVALID_TOKEN


def test_wrong_issuer_rejected() -> None:
    with pytest.raises(AppError) as exc:
        _verifier().verify(mint_token(issuer="evil-issuer"))
    assert exc.value.code == ErrorCode.INVALID_TOKEN


def test_garbage_token_rejected() -> None:
    with pytest.raises(AppError) as exc:
        _verifier().verify("not.a.jwt")
    assert exc.value.code == ErrorCode.INVALID_TOKEN


def _forge_hs256(secret: str, payload: dict) -> str:
    def b64(raw: bytes) -> str:
        return base64.urlsafe_b64encode(raw).rstrip(b"=").decode()

    header = b64(json.dumps({"alg": "HS256", "typ": "JWT"}).encode())
    body = b64(json.dumps(payload).encode())
    signing_input = f"{header}.{body}".encode()
    sig = hmac.new(secret.encode(), signing_input, hashlib.sha256).digest()
    return f"{header}.{body}.{b64(sig)}"


def test_alg_confusion_hs256_rejected() -> None:
    # An attacker hand-signs HS256 using the RSA public PEM as the shared secret (PyJWT's own
    # encoder blocks this, so we forge by hand). The RS256-only verifier must reject it.
    forged = _forge_hs256(TEST_PUBLIC_PEM, {"sub": "x", "iss": ISSUER, "aud": AUDIENCE})
    with pytest.raises(AppError):
        _verifier().verify(forged)
