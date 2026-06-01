"""RBAC for merchant-onboarding — authorized from the **token claims**, pure and unit-tested.

DELIBERATE DIVERGENCE FROM identity-service: identity is the system of record for memberships, so
it reloads fresh org roles from its ``memberships`` table for every RBAC check. merchant-onboarding
owns NO memberships table — one schema per service, and cross-schema FKs to ``identity.memberships``
are disallowed (rules.md). So it authorizes from ``AccessClaims.roles`` directly; the RS256
signature on the token (verified in :mod:`app.security.jwt`) is the trust anchor.

``require_org_member`` raises ``ORG_NOT_FOUND`` (404, not 403) when the principal isn't a member,
so org existence isn't leaked to non-members. ``require_admin`` (roles ``owner``/``admin``) raises
``FORBIDDEN`` when the member lacks the elevated role.
"""

from __future__ import annotations

from app.errors import AppError, ErrorCode
from app.security.jwt import AccessClaims

# Org roles that may perform privileged merchant actions (fee-tier changes).
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
