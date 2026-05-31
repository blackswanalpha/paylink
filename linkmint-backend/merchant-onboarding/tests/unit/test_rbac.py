from __future__ import annotations

import pytest

from app.domain import rbac
from app.errors import AppError, ErrorCode
from app.security.jwt import AccessClaims, OrgRole


def _principal(*roles: tuple[str, str]) -> AccessClaims:
    return AccessClaims(
        user_id="u1",
        roles=[OrgRole(org_id=o, role=r) for o, r in roles],
        user_roles=["payer"],
        kyc_tier=1,
        jti="j",
        sid="s",
    )


def test_org_role_lookup() -> None:
    p = _principal(("o1", "admin"), ("o2", "viewer"))
    assert rbac.org_role(p, "o1") == "admin"
    assert rbac.org_role(p, "o2") == "viewer"
    assert rbac.org_role(p, "o3") is None


def test_require_org_member_ok() -> None:
    assert rbac.require_org_member(_principal(("o1", "viewer")), "o1") == "viewer"


def test_require_org_member_not_a_member_is_org_not_found() -> None:
    with pytest.raises(AppError) as exc:
        rbac.require_org_member(_principal(("o1", "owner")), "other")
    assert exc.value.code == ErrorCode.ORG_NOT_FOUND  # no existence leak (404, not 403)


def test_require_admin_owner_and_admin_ok() -> None:
    assert rbac.require_admin(_principal(("o1", "owner")), "o1") == "owner"
    assert rbac.require_admin(_principal(("o1", "admin")), "o1") == "admin"


def test_require_admin_member_without_role_is_forbidden() -> None:
    with pytest.raises(AppError) as exc:
        rbac.require_admin(_principal(("o1", "developer")), "o1")
    assert exc.value.code == ErrorCode.FORBIDDEN


def test_require_admin_non_member_is_org_not_found() -> None:
    with pytest.raises(AppError) as exc:
        rbac.require_admin(_principal(("o1", "owner")), "other")
    assert exc.value.code == ErrorCode.ORG_NOT_FOUND
