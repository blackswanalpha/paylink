"""End-to-end invoice flow against real Postgres + Redis.

create (DRAFT) → finalize (mints a PayLink, OPEN) → simulate the chain settlement
(``chain.paylink.verified`` via the consumer over a real session) → the invoice is PAID, with the
``invoice.*`` outbox rows persisted. The PayLink mint is faked (paylink-service isn't in this stack).
"""

from __future__ import annotations

import asyncio
import uuid

import pytest
import sqlalchemy as sa
from fastapi.testclient import TestClient

from app.db.repositories import InvoiceRepository
from app.db.session import make_engine, make_sessionmaker
from app.domain.services import ServiceDeps, build_services
from app.events.consumer import InvoiceEventConsumer
from app.events.stub import NoopPublisher
from tests._support import FakePaylink, create_body, make_settings, merchant_headers

pytestmark = pytest.mark.integration


async def _settle(pg_url: str, pl_id: str) -> None:
    """Drive the chain.paylink.verified consumer over a real session (the settlement path)."""
    engine = make_engine(pg_url)
    sessionmaker = make_sessionmaker(engine)
    try:
        async with sessionmaker() as session:
            deps = ServiceDeps(
                repo=InvoiceRepository(session),
                commit=session.commit,
                settings=make_settings(database_url=pg_url),
                publisher=NoopPublisher(),
                paylink=FakePaylink(),
            )
            consumer = InvoiceEventConsumer(build_services(deps).invoices)
            await consumer.handle("chain.paylink.verified", {"entity_id": pl_id})
    finally:
        await engine.dispose()


def test_create_finalize_settle(live_client: TestClient, pg_url: str) -> None:
    uid = str(uuid.uuid4())
    pl_id = f"PLK_{uuid.uuid4().hex[:10]}"
    live_client.app.state.paylink_client = FakePaylink(pl_id=pl_id)

    # 1. create → DRAFT
    r = live_client.post("/v1/invoices", json=create_body(), headers=merchant_headers(uid))
    assert r.status_code == 201
    iid = r.json()["invoice_id"]
    assert r.json()["status"] == "DRAFT"

    # 2. finalize → OPEN + pl_id
    rf = live_client.post(f"/v1/invoices/{iid}/finalize", headers=merchant_headers(uid))
    assert rf.status_code == 200
    assert rf.json()["status"] == "OPEN"
    assert rf.json()["pl_id"] == pl_id

    # 3. the invoice + its lines persisted with the aggregated total
    rg = live_client.get(f"/v1/invoices/{iid}", headers=merchant_headers(uid))
    assert rg.status_code == 200
    assert rg.json()["total"] == 3480
    assert len(rg.json()["lines"]) == 1

    # 4. settle via the chain event consumer → PAID
    asyncio.run(_settle(pg_url, pl_id))
    rp = live_client.get(f"/v1/invoices/{iid}", headers=merchant_headers(uid))
    assert rp.json()["status"] == "PAID"
    assert rp.json()["paid_at"] is not None

    # 5. the outbox carries the lifecycle events
    engine = sa.create_engine(pg_url)  # psycopg3 drives sync engines too
    with engine.connect() as conn:
        kinds = set(
            conn.execute(
                sa.text("SELECT kind FROM invoice.invoice_events WHERE invoice_id = :i"),
                {"i": iid},
            ).scalars()
        )
    engine.dispose()
    assert {"invoice.created", "invoice.finalized", "invoice.paid"} <= kinds


def test_void_blocks_after_paid(live_client: TestClient, pg_url: str) -> None:
    uid = str(uuid.uuid4())
    pl_id = f"PLK_{uuid.uuid4().hex[:10]}"
    live_client.app.state.paylink_client = FakePaylink(pl_id=pl_id)
    iid = live_client.post(
        "/v1/invoices", json=create_body(), headers=merchant_headers(uid)
    ).json()["invoice_id"]
    live_client.post(f"/v1/invoices/{iid}/finalize", headers=merchant_headers(uid))
    asyncio.run(_settle(pg_url, pl_id))
    rv = live_client.post(f"/v1/invoices/{iid}/void", headers=merchant_headers(uid))
    assert rv.status_code == 409
    assert rv.json()["error"]["code"] == "ALREADY_PAID"
