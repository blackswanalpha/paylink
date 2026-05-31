"""RBAC: role→permission matrix + role→scope grants. Pure and exhaustively unit-tested.

Enforcement entry points raise the standard ``AppError`` so routers stay thin. ``ORG_NOT_FOUND`` is
returned (not ``FORBIDDEN``) when the principal has no membership in the org, so org existence isn't
leaked to non-members.
"""

from __future__ import annotations

from app.domain.models import Permission, Role, Scope
from app.errors import AppError, ErrorCode
from app.security.jwt import OrgRole

_ALL_PERMISSIONS: set[Permission] = set(Permission)

_MATRIX: dict[Role, set[Permission]] = {
    Role.OWNER: set(_ALL_PERMISSIONS),
    Role.ADMIN: _ALL_PERMISSIONS - {Permission.ORG_DELETE},
    Role.DEVELOPER: {
        Permission.ORG_READ,
        Permission.MEMBER_READ,
        Permission.APIKEY_READ,
        Permission.APIKEY_ISSUE,
        Permission.APIKEY_REVOKE,
    },
    Role.OPERATOR: {Permission.ORG_READ, Permission.MEMBER_READ, Permission.APIKEY_READ},
    Role.VIEWER: {Permission.ORG_READ, Permission.MEMBER_READ},
}

# Maximal API-key scopes a role may grant (cannot escalate beyond this set).
_SCOPE_GRANTS: dict[Role, set[Scope]] = {
    Role.OWNER: set(Scope),
    Role.ADMIN: set(Scope),
    Role.DEVELOPER: {Scope.PAYLINKS_READ, Scope.PAYLINKS_WRITE, Scope.PAYMENTS_READ},
    Role.OPERATOR: set(),
    Role.VIEWER: set(),
}


def permissions_for(role: str) -> set[Permission]:
    try:
        return _MATRIX[Role(role)]
    except ValueError:
        return set()


def can(role: str, permission: Permission) -> bool:
    return permission in permissions_for(role)


def role_in_org(roles: list[OrgRole], org_id: str) -> str | None:
    for r in roles:
        if r.org_id == org_id:
            return r.role
    return None


def require(roles: list[OrgRole], org_id: str, permission: Permission) -> str:
    """Return the caller's role in ``org_id`` if it grants ``permission``; else raise.

    Raises ``ORG_NOT_FOUND`` when the caller isn't a member (no existence leak) and ``FORBIDDEN``
    when they are a member but lack the permission.
    """
    role = role_in_org(roles, org_id)
    if role is None:
        raise AppError(ErrorCode.ORG_NOT_FOUND, "organization not found")
    if not can(role, permission):
        raise AppError(
            ErrorCode.FORBIDDEN,
            f"role '{role}' lacks permission '{permission.value}'",
            details={"required": permission.value, "role": role},
        )
    return role


def allowed_scopes_for_role(role: str) -> set[Scope]:
    try:
        return _SCOPE_GRANTS[Role(role)]
    except ValueError:
        return set()


def validate_scopes(role: str, requested: list[str]) -> list[str]:
    """Validate every requested scope is known AND grantable by ``role``; else SCOPE_NOT_ALLOWED."""
    allowed = {s.value for s in allowed_scopes_for_role(role)}
    known = {s.value for s in Scope}
    for scope in requested:
        if scope not in known:
            raise AppError(ErrorCode.SCOPE_NOT_ALLOWED, f"unknown scope '{scope}'")
        if scope not in allowed:
            raise AppError(
                ErrorCode.SCOPE_NOT_ALLOWED,
                f"role '{role}' cannot grant scope '{scope}'",
                details={"scope": scope, "role": role},
            )
    return requested
