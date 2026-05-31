from __future__ import annotations

import base64
import hashlib
import hmac
import json

import pytest

from app.errors import AppError, ErrorCode
from app.security.jwt import JwtVerifier
from tests._support import AUDIENCE, ISSUER, TEST_PUBLIC_PEM, mint_token


def _verifier(audience: str = AUDIENCE) -> JwtVerifier:
    return JwtVerifier(TEST_PUBLIC_PEM, issuer=ISSUER, audience=audience)


def test_roundtrip_with_mfa_and_org_type() -> None:
    token = mint_token(user_id="u1", roles=[("o1", "admin", "admin")], mfa=True)
    claims = _verifier().verify(token)
    assert claims.user_id == "u1"
    assert claims.mfa is True
    assert "mfa" in claims.amr and "pwd" in claims.amr
    assert claims.roles[0].type == "admin"


def test_no_mfa_defaults_false() -> None:
    claims = _verifier().verify(mint_token(roles=[("o1", "owner", "merchant")], mfa=False))
    assert claims.mfa is False
    assert claims.amr == ["pwd"]
    assert claims.roles[0].type == "merchant"


def test_expired_token_raises() -> None:
    with pytest.raises(AppError) as exc:
        _verifier().verify(mint_token(expired=True))
    assert exc.value.code == ErrorCode.TOKEN_EXPIRED


def test_wrong_audience_rejected() -> None:
    with pytest.raises(AppError) as exc:
        _verifier(audience="WRONG").verify(mint_token())
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
    # An attacker hand-signs HS256 using the RSA public PEM as the shared secret. The RS256-only
    # verifier must reject it (alg-confusion guard).
    forged = _forge_hs256(TEST_PUBLIC_PEM, {"sub": "x", "iss": ISSUER, "aud": AUDIENCE})
    with pytest.raises(AppError):
        _verifier().verify(forged)
