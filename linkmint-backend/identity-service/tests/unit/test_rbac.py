from __future__ import annotations

import pytest

from app.domain import rbac
from app.domain.models import Permission, Scope
from app.errors import AppError, ErrorCode
from app.security.jwt import OrgRole


def test_owner_has_all_permissions() -> None:
    assert all(rbac.can("owner", p) for p in Permission)


def test_admin_lacks_only_org_delete() -> None:
    assert not rbac.can("admin", Permission.ORG_DELETE)
    assert rbac.can("admin", Permission.MEMBER_INVITE)
    assert rbac.can("admin", Permission.APIKEY_ISSUE)


def test_developer_keys_plus_reads() -> None:
    assert rbac.can("developer", Permission.APIKEY_ISSUE)
    assert rbac.can("developer", Permission.ORG_READ)
    assert not rbac.can("developer", Permission.MEMBER_INVITE)


def test_viewer_and_operator_read_only() -> None:
    assert rbac.can("viewer", Permission.ORG_READ)
    assert not rbac.can("viewer", Permission.APIKEY_READ)
    assert rbac.can("operator", Permission.APIKEY_READ)
    assert not rbac.can("operator", Permission.APIKEY_ISSUE)


def test_unknown_role_has_no_permissions() -> None:
    assert rbac.permissions_for("nope") == set()
    assert not rbac.can("nope", Permission.ORG_READ)


def test_require_forbidden_when_member_lacks_permission() -> None:
    with pytest.raises(AppError) as exc:
        rbac.require([OrgRole("o1", "viewer")], "o1", Permission.MEMBER_INVITE)
    assert exc.value.code == ErrorCode.FORBIDDEN


def test_require_org_not_found_when_not_a_member() -> None:
    with pytest.raises(AppError) as exc:
        rbac.require([OrgRole("o1", "owner")], "other-org", Permission.ORG_READ)
    assert exc.value.code == ErrorCode.ORG_NOT_FOUND


def test_require_returns_role_on_success() -> None:
    assert rbac.require([OrgRole("o1", "owner")], "o1", Permission.ORG_DELETE) == "owner"


def test_validate_scopes() -> None:
    assert rbac.validate_scopes("owner", [Scope.PAYMENTS_WRITE.value]) == [
        Scope.PAYMENTS_WRITE.value
    ]
    with pytest.raises(AppError) as exc:
        rbac.validate_scopes("developer", [Scope.PAYMENTS_WRITE.value])
    assert exc.value.code == ErrorCode.SCOPE_NOT_ALLOWED
    with pytest.raises(AppError):
        rbac.validate_scopes("owner", ["bogus:scope"])
