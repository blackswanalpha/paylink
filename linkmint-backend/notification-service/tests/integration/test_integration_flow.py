"""End-to-end against real Postgres + Redis: intake → eager deliver → SENT; failures persist."""

from __future__ import annotations

import uuid

import httpx
import pytest
from fastapi.testclient import TestClient
from sqlalchemy import text

from app.channels.registry import build_channel_registry
from app.db.models import DeliveryRow
from app.db.repository import SyncDeliveryStore
from app.db.session import make_sync_engine, make_sync_sessionmaker
from app.delivery.runner import DeliveryRunner
from tests._support import make_settings

pytestmark = pytest.mark.integration

CONTACT = {"phone": "+254712345678", "email": "jane@example.com"}
DATA = {"amount": "1500", "currency": "KES", "paylink_id": "pl_e2e", "dedupe_id": "pl_e2e"}


def test_intake_to_sent(live_client: TestClient) -> None:
    resp = live_client.post(
        "/v1/notifications",
        json={
            "event_kind": "paylink.verified",
            "user_id": str(uuid.uuid4()),
            "data": DATA,
            "contact": CONTACT,
        },
    )
    assert resp.status_code == 201
    ids = resp.json()["delivery_ids"]
    assert len(ids) == 2  # sms + email

    # Eager Celery already ran the console sends → both deliveries SENT, recipient masked.
    for delivery_id in ids:
        view = live_client.get(f"/internal/deliveries/{delivery_id}").json()
        assert view["status"] == "SENT"
        assert view["delivered_at"] is not None
        assert "*" in view["recipient"]


def test_readyz_ok(live_client: TestClient) -> None:
    body = live_client.get("/internal/readyz").json()
    assert body["status"] == "ready"
    assert body["checks"]["db"] == "ok"
    assert body["checks"]["redis"] == "ok"


def test_failed_delivery_persists_retry_state(pg_url: str) -> None:
    """The runner + SyncDeliveryStore against the real DB: a forced failure persists FAILED state."""
    engine = make_sync_engine(pg_url)
    delivery_id = uuid.uuid4()
    fail_phone = "+254700000000"
    with make_sync_sessionmaker(engine)() as session:
        session.add(
            DeliveryRow(
                delivery_id=delivery_id,
                channel="sms",
                recipient=fail_phone,
                event_kind="paylink.verified",
                payload={"body": "hi", "dedupe_key": f"k-{delivery_id}"},
                status="QUEUED",
                attempts=0,
            )
        )
        session.commit()

    store = SyncDeliveryStore(engine)
    with httpx.Client() as client:
        channels = build_channel_registry(make_settings(console_fail_recipients=fail_phone), client)
        outcome = DeliveryRunner(store, channels).run_once(delivery_id)

    assert outcome.status == "FAILED"
    assert outcome.countdown == 30

    with engine.connect() as conn:
        row = conn.execute(
            text(
                "SELECT status, attempts, last_error, next_retry_at "
                "FROM notify.deliveries WHERE delivery_id = :id"
            ),
            {"id": str(delivery_id)},
        ).one()
    assert row.status == "FAILED"
    assert row.attempts == 1
    assert row.last_error
    assert row.next_retry_at is not None
    engine.dispose()
