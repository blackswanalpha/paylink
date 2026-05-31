from __future__ import annotations

import base64
import hashlib
import hmac
import json
from datetime import UTC, datetime, timedelta

import jwt as pyjwt
import pytest

from app.errors import AppError, ErrorCode
from app.security.jwt import JwtIssuer, JwtVerifier, OrgRole
from app.security.keys import KeyStore
from tests._support import make_settings


def _issuer_verifier() -> tuple[KeyStore, JwtIssuer, JwtVerifier]:
    keys = KeyStore.from_settings(make_settings())
    issuer = JwtIssuer(keys, issuer="linkmint-identity", audience="linkmint", ttl_seconds=3600)
    verifier = JwtVerifier(keys.public_pem, issuer="linkmint-identity", audience="linkmint")
    return keys, issuer, verifier


def test_issue_verify_roundtrip() -> None:
    _, issuer, verifier = _issuer_verifier()
    token, expires_in = issuer.issue_access(
        user_id="u1", roles=[OrgRole("o1", "owner")], user_roles=["payer"], kyc_tier=2, sid="s1"
    )
    assert expires_in == 3600
    claims = verifier.verify(token)
    assert claims.user_id == "u1"
    assert claims.kyc_tier == 2
    assert claims.sid == "s1"
    assert claims.roles[0].org_id == "o1" and claims.roles[0].role == "owner"
    assert "payer" in claims.user_roles


def test_expired_token_raises() -> None:
    _, issuer, verifier = _issuer_verifier()
    token, _ = issuer.issue_access(
        user_id="u",
        roles=[],
        user_roles=[],
        kyc_tier=0,
        now=datetime.now(UTC) - timedelta(hours=2),
    )
    with pytest.raises(AppError) as exc:
        verifier.verify(token)
    assert exc.value.code == ErrorCode.TOKEN_EXPIRED


def test_bad_signature_rejected() -> None:
    _, issuer, _ = _issuer_verifier()
    token, _ = issuer.issue_access(user_id="u", roles=[], user_roles=[], kyc_tier=0)
    other = KeyStore.from_settings(make_settings(jwt_private_key_pem=None))  # different key
    verifier = JwtVerifier(other.public_pem, issuer="linkmint-identity", audience="linkmint")
    with pytest.raises(AppError) as exc:
        verifier.verify(token)
    assert exc.value.code == ErrorCode.INVALID_TOKEN


def test_wrong_audience_rejected() -> None:
    keys, issuer, _ = _issuer_verifier()
    token, _ = issuer.issue_access(user_id="u", roles=[], user_roles=[], kyc_tier=0)
    verifier = JwtVerifier(keys.public_pem, issuer="linkmint-identity", audience="WRONG")
    with pytest.raises(AppError) as exc:
        verifier.verify(token)
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
    keys, _, verifier = _issuer_verifier()
    # An attacker hand-signs HS256 using the RSA public PEM as the shared secret (PyJWT's own
    # encoder blocks this, so we forge by hand). The RS256-only verifier must reject it.
    forged = _forge_hs256(
        keys.public_pem, {"sub": "x", "iss": "linkmint-identity", "aud": "linkmint"}
    )
    with pytest.raises(AppError):
        verifier.verify(forged)


def test_jwks_and_kid() -> None:
    keys, issuer, _ = _issuer_verifier()
    jwks = keys.jwks()
    entry = jwks["keys"][0]
    assert entry["kty"] == "RSA" and entry["alg"] == "RS256" and entry["use"] == "sig"
    assert entry["kid"] == keys.kid and entry["n"] and entry["e"]
    token, _ = issuer.issue_access(user_id="u", roles=[], user_roles=[], kyc_tier=0)
    header = pyjwt.get_unverified_header(token)
    assert header["kid"] == keys.kid and header["alg"] == "RS256"
