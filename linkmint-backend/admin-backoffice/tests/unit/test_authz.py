from __future__ import annotations

from app.security.authz import is_admin
from app.security.jwt import AccessClaims, OrgRole


def _claims(
    *, roles: list[tuple[str, ...]] | None = None, user_roles: list[str] | None = None
) -> AccessClaims:
    return AccessClaims(
        user_id="u",
        roles=[OrgRole(*r) for r in (roles or [])],
        user_roles=user_roles or [],
        kyc_tier=0,
        jti="j",
        sid="s",
        mfa=True,
        amr=["pwd", "mfa"],
    )


def test_is_admin_via_admin_org_type() -> None:
    assert is_admin(_claims(roles=[("o", "viewer", "admin")]))


def test_is_admin_via_admin_role() -> None:
    assert is_admin(_claims(roles=[("o", "admin")]))


def test_is_admin_via_user_role() -> None:
    assert is_admin(_claims(user_roles=["admin"]))


def test_not_admin_for_merchant_membership() -> None:
    assert not is_admin(_claims(roles=[("o", "owner", "merchant")], user_roles=["payer"]))


def test_not_admin_with_no_roles() -> None:
    assert not is_admin(_claims())
