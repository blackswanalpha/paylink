"""Admin authorization: the admin-role gate, the MFA gate, and the default-deny scope gate.

Every ``/v1/admin`` route declares ``Depends(require_admin("<scope>"))``. The returned dependency
verifies the principal is platform staff (admin-org membership / admin role), that the token is
MFA-elevated, and that the caller holds the required scope — then returns the principal + granted
scopes (an :class:`AdminContext`) so the route can audit the access. Phase 1 is read-only, so every
route requires ``support.read``; the other scopes exist for the Phase-2 mutations.
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from dataclasses import dataclass

from app.deps import AdminRepoDep, PrincipalDep, SettingsDep
from app.domain.scopes import ScopeResolver
from app.errors import AppError, ErrorCode
from app.security.jwt import AccessClaims


def is_admin(principal: AccessClaims) -> bool:
    """True for platform staff: admin-org membership, an ``admin`` role, or an admin user-role."""
    if "admin" in principal.user_roles:
        return True
    return any(r.type == "admin" or r.role == "admin" for r in principal.roles)


@dataclass(frozen=True)
class AdminContext:
    principal: AccessClaims
    scopes: frozenset[str]


def require_admin(scope: str) -> Callable[..., Awaitable[AdminContext]]:
    """Build the admin → MFA → scope gate dependency for ``scope`` (default-deny)."""

    async def _dep(
        principal: PrincipalDep,
        repo: AdminRepoDep,
        settings: SettingsDep,
    ) -> AdminContext:
        if not is_admin(principal):
            raise AppError(ErrorCode.FORBIDDEN, "admin role required")
        if not principal.mfa:
            raise AppError(ErrorCode.MFA_REQUIRED, "MFA required for admin access")
        granted = await ScopeResolver(repo, settings).resolve(principal.user_id)
        if scope not in granted:
            raise AppError(
                ErrorCode.SCOPE_DENIED,
                f"scope '{scope}' is required",
                details={"required": scope},
            )
        return AdminContext(principal=principal, scopes=granted)

    return _dep
