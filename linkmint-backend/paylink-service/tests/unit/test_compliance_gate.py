"""work12 / Flow E — the compliance gate on PayLink create (service- and API-level)."""

from __future__ import annotations

from datetime import UTC, datetime
from typing import Any

import fakeredis.aioredis
import pytest
from fastapi.testclient import TestClient

from app.chain.nonce import NonceManager
from app.deps import caller_address, caller_user_id, get_idempotency, get_service
from app.domain.models import OffChainStatus
from app.domain.service import CreateCommand, PayLinkService
from app.errors import AppError, ErrorCode
from app.events.stub import NoopPublisher
from app.idempotency import IdempotencyStore
from app.main import create_app
from tests._support import (
    FakeChainClient,
    FakeComplianceClient,
    FakeRepository,
    make_settings,
    noop_commit,
)

FUTURE = datetime(2030, 1, 1, tzinfo=UTC)
RECEIVER = "0x0000000000000000000000000000000000000004"


def _svc(
    repo: FakeRepository,
    chain: FakeChainClient,
    signer: Any,
    compliance: FakeComplianceClient,
    **settings_over: Any,
) -> PayLinkService:
    over: dict[str, Any] = {"compliance_check_enabled": True, "amount_kyc_threshold": 1000}
    over.update(settings_over)
    return PayLinkService(
        repo=repo,
        commit=noop_commit,
        chain=chain,  # type: ignore[arg-type]
        signer=signer,
        nonces=NonceManager(chain),  # type: ignore[arg-type]
        publisher=NoopPublisher(),
        settings=make_settings(**over),
        compliance=compliance,  # type: ignore[arg-type]
    )


def _cmd(caller: str, **over: Any) -> CreateCommand:
    base: dict[str, Any] = {
        "receiver": RECEIVER,
        "amount": 5000,  # default: above the 1000 test threshold
        "currency": "KES",
        "expiry": FUTURE,
        "usage": "single",
        "metadata": None,
        "rules": None,
        "idem_key": "i1",
        "caller_addr": caller,
        "user_id": "user-1",
    }
    base.update(over)
    return CreateCommand(**base)


async def test_below_threshold_skips_gate(fake_repo: Any, fake_chain: Any, signer: Any) -> None:
    comp = FakeComplianceClient(decision="block")  # would block if it were called
    svc = _svc(fake_repo, fake_chain, signer, comp)
    row = await svc.create(_cmd(signer.address, amount=500))
    assert row.status == OffChainStatus.PENDING.value
    assert comp.calls == []  # not consulted below threshold


async def test_above_threshold_allow_creates_and_forwards_fields(
    fake_repo: Any, fake_chain: Any, signer: Any
) -> None:
    comp = FakeComplianceClient(decision="allow")
    svc = _svc(fake_repo, fake_chain, signer, comp)
    row = await svc.create(_cmd(signer.address, amount=5000))
    assert row.status == OffChainStatus.PENDING.value
    assert len(comp.calls) == 1
    call = comp.calls[0]
    assert call["user_id"] == "user-1"
    assert call["action"] == "paylink.create"
    assert call["amount"] == 5000
    assert call["currency"] == "KES"
    assert call["context"].startswith("paylink.create:")


async def test_above_threshold_block_raises_402_no_row_no_submit(
    fake_repo: Any, fake_chain: Any, signer: Any
) -> None:
    comp = FakeComplianceClient(
        decision="block", score=1.0, reasons=[{"code": "AML_THRESHOLD", "detail": "x"}]
    )
    svc = _svc(fake_repo, fake_chain, signer, comp)
    with pytest.raises(AppError) as exc:
        await svc.create(_cmd(signer.address, amount=5000))
    assert exc.value.code is ErrorCode.KYC_REQUIRED
    assert exc.value.http_status == 402
    # Flow E: nothing persisted, nothing submitted, no events on a block.
    assert fake_repo.rows == {}
    assert fake_repo.events == []
    assert fake_chain.sent == []


async def test_above_threshold_review_creates(fake_repo: Any, fake_chain: Any, signer: Any) -> None:
    comp = FakeComplianceClient(decision="review")
    svc = _svc(fake_repo, fake_chain, signer, comp)
    row = await svc.create(_cmd(signer.address, amount=5000))
    assert row.status == OffChainStatus.PENDING.value  # review is a soft signal — creation proceeds


async def test_unavailable_fail_closed_refuses(
    fake_repo: Any, fake_chain: Any, signer: Any
) -> None:
    comp = FakeComplianceClient(raise_unavailable=True)
    svc = _svc(fake_repo, fake_chain, signer, comp, compliance_fail_open=False)
    with pytest.raises(AppError) as exc:
        await svc.create(_cmd(signer.address, amount=5000))
    assert exc.value.code is ErrorCode.KYC_REQUIRED
    assert fake_repo.rows == {}


async def test_unavailable_fail_open_creates(fake_repo: Any, fake_chain: Any, signer: Any) -> None:
    comp = FakeComplianceClient(raise_unavailable=True)
    svc = _svc(fake_repo, fake_chain, signer, comp, compliance_fail_open=True)
    row = await svc.create(_cmd(signer.address, amount=5000))
    assert row.status == OffChainStatus.PENDING.value


async def test_missing_user_id_skips_gate(fake_repo: Any, fake_chain: Any, signer: Any) -> None:
    comp = FakeComplianceClient(decision="block")
    svc = _svc(fake_repo, fake_chain, signer, comp)
    row = await svc.create(_cmd(signer.address, amount=5000, user_id=None))
    assert row.status == OffChainStatus.PENDING.value
    assert comp.calls == []  # no user id → cannot evaluate → skip (dev/no-gateway)


async def test_gate_disabled_skips(fake_repo: Any, fake_chain: Any, signer: Any) -> None:
    comp = FakeComplianceClient(decision="block")
    svc = _svc(fake_repo, fake_chain, signer, comp, compliance_check_enabled=False)
    row = await svc.create(_cmd(signer.address, amount=5000))
    assert row.status == OffChainStatus.PENDING.value
    assert comp.calls == []


def test_create_api_returns_402_envelope_on_block(
    fake_repo: Any, fake_chain: Any, signer: Any
) -> None:
    """End-to-end through the route: a block surfaces as 402 KYC_REQUIRED in the error envelope."""
    settings = make_settings(compliance_check_enabled=True, amount_kyc_threshold=1000)
    app = create_app(settings)
    comp = FakeComplianceClient(
        decision="block", reasons=[{"code": "AML_THRESHOLD", "detail": "x"}]
    )
    nonces = NonceManager(fake_chain)
    idem = IdempotencyStore(fakeredis.aioredis.FakeRedis(decode_responses=True), 3600)

    async def _service_override() -> Any:
        yield PayLinkService(
            repo=fake_repo,
            commit=noop_commit,
            chain=fake_chain,
            signer=signer,
            nonces=nonces,
            publisher=NoopPublisher(),
            settings=settings,
            compliance=comp,
        )

    app.dependency_overrides[get_service] = _service_override
    app.dependency_overrides[get_idempotency] = lambda: idem
    app.dependency_overrides[caller_address] = lambda: signer.address
    app.dependency_overrides[caller_user_id] = lambda: "user-1"
    with TestClient(app) as tc:
        resp = tc.post(
            "/v1/paylinks",
            json={
                "receiver": RECEIVER,
                "amount": 5000,
                "currency": "KES",
                "expiry": "2030-01-01T00:00:00Z",
                "usage": "single",
            },
        )
    app.dependency_overrides.clear()
    assert resp.status_code == 402
    err = resp.json()["error"]
    assert err["code"] == "KYC_REQUIRED"
    assert set(err.keys()) == {"code", "message", "details", "trace_id"}
    assert fake_repo.rows == {}  # nothing persisted
