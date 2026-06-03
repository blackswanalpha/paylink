"""RBAC matrix — org membership + platform-admin gates (pure)."""

from __future__ import annotations

import pytest

from app.domain.rbac import (
    is_platform_admin,
    org_role,
    require_admin,
    require_org_member,
    require_platform_admin,
)
from app.errors import AppError, ErrorCode
from app.security.jwt import AccessClaims, OrgRole


def _claims(
    *, roles: list[OrgRole] | None = None, user_roles: list[str] | None = None
) -> AccessClaims:
    return AccessClaims(
        user_id="u1",
        roles=roles or [],
        user_roles=user_roles or [],
        kyc_tier=1,
        jti="j",
        sid="s",
    )


def test_org_role_lookup() -> None:
    c = _claims(roles=[OrgRole("o1", "admin")])
    assert org_role(c, "o1") == "admin"
    assert org_role(c, "o2") is None


def test_require_org_member_not_found_for_non_member() -> None:
    with pytest.raises(AppError) as exc:
        require_org_member(_claims(), "o1")
    assert exc.value.code == ErrorCode.ORG_NOT_FOUND


def test_require_admin_matrix() -> None:
    assert require_admin(_claims(roles=[OrgRole("o1", "owner")]), "o1") == "owner"
    # member but not elevated → FORBIDDEN
    with pytest.raises(AppError) as exc:
        require_admin(_claims(roles=[OrgRole("o1", "member")]), "o1")
    assert exc.value.code == ErrorCode.FORBIDDEN
    # not a member → ORG_NOT_FOUND (no leak)
    with pytest.raises(AppError) as exc2:
        require_admin(_claims(), "o1")
    assert exc2.value.code == ErrorCode.ORG_NOT_FOUND


def test_platform_admin() -> None:
    allowed = {"admin"}
    assert is_platform_admin(_claims(user_roles=["admin"]), allowed) is True
    assert is_platform_admin(_claims(user_roles=["merchant"]), allowed) is False
    require_platform_admin(_claims(user_roles=["admin"]), allowed)  # no raise
    with pytest.raises(AppError) as exc:
        require_platform_admin(_claims(user_roles=["merchant"]), allowed)
    assert exc.value.code == ErrorCode.FORBIDDEN
