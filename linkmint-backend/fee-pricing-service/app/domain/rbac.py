"""RBAC for fee-pricing-service — authorized from the **token claims**, pure and unit-tested.

Like merchant-onboarding, this service owns NO memberships table (one schema per service;
cross-schema FKs to ``identity.memberships`` are disallowed — rules.md). So it authorizes from
``AccessClaims.roles`` (org-scoped) and ``AccessClaims.user_roles`` (platform-level) directly; the
RS256 signature on the token (verified in :mod:`app.security.jwt`) is the trust anchor.

``require_org_member`` raises ``ORG_NOT_FOUND`` (404, not 403) when the principal isn't a member, so
org existence isn't leaked. ``require_platform_admin`` gates tier administration on a platform-level
role (e.g. ``admin``), configurable via ``PRICING_ADMIN_USER_ROLES``.
"""

from __future__ import annotations

from collections.abc import Iterable

from app.errors import AppError, ErrorCode
from app.security.jwt import AccessClaims

# Org roles that may perform privileged per-merchant actions.
_ADMIN_ROLES: frozenset[str] = frozenset({"owner", "admin"})


def org_role(principal: AccessClaims, org_id: str) -> str | None:
    """The principal's role in ``org_id`` from the token claims, or None if not a member."""
    for r in principal.roles:
        if r.org_id == org_id:
            return r.role
    return None


def require_org_member(principal: AccessClaims, org_id: str) -> str:
    """Return the caller's role in ``org_id``; raise ``ORG_NOT_FOUND`` if not a member (no leak)."""
    role = org_role(principal, org_id)
    if role is None:
        raise AppError(ErrorCode.ORG_NOT_FOUND, "organization not found")
    return role


def require_admin(principal: AccessClaims, org_id: str) -> str:
    """Require an ``owner``/``admin`` membership in ``org_id``; else raise.

    ``ORG_NOT_FOUND`` when the caller isn't a member (no existence leak); ``FORBIDDEN`` when they
    are a member but lack the elevated role.
    """
    role = require_org_member(principal, org_id)
    if role not in _ADMIN_ROLES:
        raise AppError(
            ErrorCode.FORBIDDEN,
            f"role '{role}' is not permitted to perform this action",
            details={"required": "owner|admin", "role": role},
        )
    return role


def is_platform_admin(principal: AccessClaims, allowed_roles: Iterable[str]) -> bool:
    """True if the principal carries any platform-level role in ``allowed_roles``."""
    allowed = set(allowed_roles)
    return any(r in allowed for r in principal.user_roles)


def require_platform_admin(principal: AccessClaims, allowed_roles: Iterable[str]) -> None:
    """Require a platform-level admin role (e.g. ``admin``); raise ``FORBIDDEN`` otherwise.

    Used for tier administration (``GET /v1/pricing/tiers``) — a cross-merchant, platform-wide view
    that no single org should see. The allowed set comes from ``PRICING_ADMIN_USER_ROLES``.
    """
    if not is_platform_admin(principal, allowed_roles):
        raise AppError(
            ErrorCode.FORBIDDEN,
            "platform-admin role required",
            details={"required": sorted(set(allowed_roles))},
        )
