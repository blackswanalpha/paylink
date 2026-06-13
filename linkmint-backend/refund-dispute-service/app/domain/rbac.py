"""RBAC for refund-dispute-service — authorized from the **token claims**, pure and unit-tested.

Like fee-pricing-service / merchant-onboarding, this service owns NO memberships table (one schema
per service; cross-schema FKs to ``identity.memberships`` are disallowed — rules.md). So it
authorizes from ``AccessClaims.roles`` (org-scoped) and ``AccessClaims.user_roles`` (platform-level)
directly; the RS256 signature on the token (verified in :mod:`app.security.jwt`) is the trust
anchor.

``require_org_member`` raises ``ORG_NOT_FOUND`` (404, not 403) when the principal isn't a member, so
org existence isn't leaked. Platform admins (``REFUND_ADMIN_USER_ROLES``) may act across orgs.
"""

from __future__ import annotations

from collections.abc import Iterable

from app.errors import AppError, ErrorCode
from app.security.jwt import AccessClaims

# Org roles that may perform privileged refund/dispute actions (approve, reject, submit).
_ADMIN_ROLES: frozenset[str] = frozenset({"owner", "admin"})


def org_role(principal: AccessClaims, org_id: str) -> str | None:
    """The principal's role in ``org_id`` from the token claims, or None if not a member."""
    for r in principal.roles:
        if r.org_id == org_id:
            return r.role
    return None


def is_platform_admin(principal: AccessClaims, allowed_roles: Iterable[str]) -> bool:
    """True if the principal carries any platform-level role in ``allowed_roles``."""
    allowed = set(allowed_roles)
    return any(r in allowed for r in principal.user_roles)


def require_org_member(
    principal: AccessClaims, org_id: str | None, *, platform_roles: Iterable[str] = ()
) -> str:
    """Return the caller's role in ``org_id``; raise ``ORG_NOT_FOUND`` if not a member.

    A platform admin (carrying one of ``platform_roles``) is allowed cross-org and returns the
    synthetic role ``"platform_admin"``. When ``org_id`` is None the record carries no org scope, so
    only a platform admin may act (org members cannot be matched) — this keeps unscoped records
    admin-only rather than world-readable.
    """
    if is_platform_admin(principal, platform_roles):
        return "platform_admin"
    if org_id is not None:
        role = org_role(principal, org_id)
        if role is not None:
            return role
    raise AppError(ErrorCode.ORG_NOT_FOUND, "resource not found")


def require_org_admin(
    principal: AccessClaims, org_id: str | None, *, platform_roles: Iterable[str] = ()
) -> str:
    """Require an ``owner``/``admin`` membership in ``org_id`` (or a platform admin); else raise.

    ``ORG_NOT_FOUND`` when the caller isn't a member (no existence leak); ``FORBIDDEN`` when they
    are
    a member but lack the elevated role.
    """
    role = require_org_member(principal, org_id, platform_roles=platform_roles)
    if role == "platform_admin":
        return role
    if role not in _ADMIN_ROLES:
        raise AppError(
            ErrorCode.FORBIDDEN,
            f"role '{role}' is not permitted to perform this action",
            details={"required": "owner|admin", "role": role},
        )
    return role
