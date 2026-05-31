"""End-to-end over real Postgres + Redis: staff grant from the DB, search + views, audit, readyz."""

from __future__ import annotations

import uuid

import pytest
import sqlalchemy as sa
from fastapi.testclient import TestClient

from tests._support import SAMPLE_USER_ID, CapturingAuditSink, mint_token

pytestmark = pytest.mark.integration


def _seed_staff(pg_url: str, sub: str, scopes: list[str]) -> None:
    engine = sa.create_engine(pg_url)
    with engine.begin() as conn:
        conn.execute(
            sa.text("INSERT INTO admin.staff (sub, scopes) VALUES (:sub, :scopes)"),
            {"sub": uuid.UUID(sub), "scopes": scopes},
        )
    engine.dispose()


def test_admin_flow(live_client: tuple[TestClient, CapturingAuditSink], pg_url: str) -> None:
    client, audit = live_client
    sub = str(uuid.uuid4())
    _seed_staff(pg_url, sub, ["support.read"])
    headers = {
        "Authorization": f"Bearer {mint_token(user_id=sub, roles=[('o', 'admin', 'admin')], mfa=True)}"
    }

    # readiness over the real db + redis
    assert client.get("/internal/readyz").status_code == 200

    # unified search (scope granted from the real admin.staff row)
    resp = client.get("/v1/admin/search", params={"q": "alice"}, headers=headers)
    assert resp.status_code == 200, resp.text
    assert any(h["label"] == "alice@example.com" for h in resp.json()["groups"]["user"])

    # a drill-down view
    view = client.get(f"/v1/admin/users/{SAMPLE_USER_ID}", headers=headers)
    assert view.status_code == 200 and view.json()["id"] == SAMPLE_USER_ID

    # every privileged access was audited
    actions = [r.action for r in audit.records]
    assert "admin.search" in actions and "admin.view.user" in actions


def test_admin_flow_unseeded_sub_is_scope_denied(
    live_client: tuple[TestClient, CapturingAuditSink], pg_url: str
) -> None:
    client, _ = live_client
    # An admin+MFA token whose sub has no admin.staff row → default-deny.
    headers = {"Authorization": f"Bearer {mint_token(roles=[('o', 'admin', 'admin')], mfa=True)}"}
    resp = client.get("/v1/admin/search", params={"q": "alice"}, headers=headers)
    assert resp.status_code == 403 and resp.json()["error"]["code"] == "SCOPE_DENIED"
