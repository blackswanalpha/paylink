"""Authorization for the user-facing KYC/status surface — self-or-admin, from the token claims.

compliance-risk owns NO memberships table (one schema per service; cross-schema FKs to
``identity.memberships`` are disallowed — rules.md), so it authorizes from ``AccessClaims``
directly; the RS256 signature on the token (verified in :mod:`app.security.jwt`) is the anchor.

A caller may read/act on a ``user_id`` iff it is THEIR OWN ``sub``, or they are platform staff (an
``admin`` user-role or an ``admin`` org role). Otherwise ``FORBIDDEN`` (403). This is pure and
unit-tested.
"""

from __future__ import annotations

from app.errors import AppError, ErrorCode
from app.security.jwt import AccessClaims


def is_admin(principal: AccessClaims) -> bool:
    """True for platform staff: an ``admin`` user-role or an ``admin`` org role."""
    if "admin" in principal.user_roles:
        return True
    return any(r.role == "admin" for r in principal.roles)


def require_self_or_admin(principal: AccessClaims, user_id: str) -> None:
    """Allow when the principal is ``user_id`` itself or platform staff; else ``FORBIDDEN``."""
    if principal.user_id == user_id or is_admin(principal):
        return
    raise AppError(
        ErrorCode.FORBIDDEN,
        "not permitted to act on behalf of another user",
        details={"required": "self|admin"},
    )
