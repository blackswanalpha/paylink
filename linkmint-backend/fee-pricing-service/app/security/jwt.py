"""RS256 access-token verification (PyJWT) — fee-pricing-service is a CONSUMER, not an issuer.

Unlike identity-service (which signs and verifies its own tokens), fee-pricing-service only
*verifies* identity-service's RS256 access tokens with identity's public key. There is intentionally
NO ``JwtIssuer``/``KeyStore`` here. The verifier is RS256-only — it never accepts HS256 or ``none``
(alg-confusion guard): the RS256 signature is the trust anchor for the RBAC claims this service
authorizes from (see :mod:`app.domain.rbac`).
"""

from __future__ import annotations

from dataclasses import dataclass

import jwt

from app.errors import AppError, ErrorCode


@dataclass(frozen=True)
class OrgRole:
    org_id: str
    role: str


@dataclass(frozen=True)
class AccessClaims:
    """Decoded, verified access-token claims (the authenticated principal)."""

    user_id: str
    roles: list[OrgRole]  # org-scoped memberships (the RBAC source of truth here)
    user_roles: list[str]  # user-level roles, e.g. ["payer"], ["admin"]
    kyc_tier: int
    jti: str
    sid: str  # session id the token was minted for


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
            OrgRole(org_id=str(r["org_id"]), role=str(r["role"]))
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
        )
