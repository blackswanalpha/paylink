"""RS256 access-token issue + verify (PyJWT).

The access token is short-lived (60 min) and self-contained: the service verifies its OWN tokens
on every protected route (the ``get_principal`` dependency), so the ``/v1/users/me`` round-trip
needs no gateway. The same public key is published at the JWKS endpoint for the gateway/other
services to verify independently.
"""

from __future__ import annotations

import uuid
from dataclasses import dataclass, field
from datetime import UTC, datetime, timedelta

import jwt

from app.errors import AppError, ErrorCode
from app.security.keys import KeyStore


@dataclass(frozen=True)
class OrgRole:
    org_id: str
    role: str
    type: str | None = None  # org type (merchant|developer|admin), when the issuer includes it


@dataclass(frozen=True)
class AccessClaims:
    """Decoded, verified access-token claims (the authenticated principal)."""

    user_id: str
    roles: list[OrgRole]  # org-scoped memberships
    user_roles: list[str]  # user-level roles, e.g. ["payer"]
    kyc_tier: int
    jti: str
    sid: str  # session id the token was minted for (marks the "current" session)
    mfa: bool = False  # true iff MFA was satisfied at login (amr contains "mfa")
    amr: list[str] = field(default_factory=list)  # auth methods (RFC 8176), e.g. ["pwd","mfa"]


class JwtIssuer:
    def __init__(self, keys: KeyStore, *, issuer: str, audience: str, ttl_seconds: int) -> None:
        self._keys = keys
        self._issuer = issuer
        self._audience = audience
        self._ttl = ttl_seconds

    def issue_access(
        self,
        *,
        user_id: str,
        roles: list[OrgRole],
        user_roles: list[str],
        kyc_tier: int,
        sid: str = "",
        mfa: bool = False,
        now: datetime | None = None,
    ) -> tuple[str, int]:
        """Mint a signed access token. Returns ``(token, expires_in_seconds)``.

        ``mfa`` records whether the session was MFA-elevated at login; it surfaces as the ``mfa``
        claim + an ``amr`` (RFC 8176) entry so downstream services (e.g. admin-backoffice) can gate
        on step-up auth without re-querying identity. Refresh/OAuth flows mint ``mfa=False``.
        """
        now = now or datetime.now(UTC)
        exp = now + timedelta(seconds=self._ttl)
        payload = {
            "sub": user_id,
            "iss": self._issuer,
            "aud": self._audience,
            "iat": int(now.timestamp()),
            "nbf": int(now.timestamp()),
            "exp": int(exp.timestamp()),
            "jti": uuid.uuid4().hex,
            "sid": sid,
            "roles": [
                {"org_id": r.org_id, "role": r.role, **({"type": r.type} if r.type else {})}
                for r in roles
            ],
            "user_roles": user_roles,
            "kyc_tier": kyc_tier,
            "mfa": mfa,
            "amr": ["pwd", "mfa"] if mfa else ["pwd"],
        }
        token = jwt.encode(
            payload, self._keys.private_pem, algorithm="RS256", headers={"kid": self._keys.kid}
        )
        return token, self._ttl


class JwtVerifier:
    def __init__(self, public_pem: str, *, issuer: str, audience: str) -> None:
        self._public_pem = public_pem
        self._issuer = issuer
        self._audience = audience

    def verify(self, token: str) -> AccessClaims:
        try:
            payload = jwt.decode(
                token,
                self._public_pem,
                algorithms=["RS256"],  # RS256 only — never accept HS/none (alg-confusion guard)
                audience=self._audience,
                issuer=self._issuer,
            )
        except jwt.ExpiredSignatureError as exc:
            raise AppError(ErrorCode.TOKEN_EXPIRED, "access token expired") from exc
        except jwt.InvalidTokenError as exc:
            raise AppError(ErrorCode.INVALID_TOKEN, "invalid access token") from exc

        roles = [
            OrgRole(
                org_id=str(r["org_id"]),
                role=str(r["role"]),
                type=str(r["type"]) if r.get("type") else None,
            )
            for r in payload.get("roles", [])
            if isinstance(r, dict) and "org_id" in r and "role" in r
        ]
        return AccessClaims(
            user_id=str(payload["sub"]),
            roles=roles,
            user_roles=[str(x) for x in payload.get("user_roles", [])],
            kyc_tier=int(payload.get("kyc_tier", 0)),
            jti=str(payload.get("jti", "")),
            sid=str(payload.get("sid", "")),
            mfa=bool(payload.get("mfa", False)),
            amr=[str(x) for x in payload.get("amr", [])],
        )
