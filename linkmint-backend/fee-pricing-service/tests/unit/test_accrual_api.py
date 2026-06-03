"""POST /v1/internal/accruals — intake + idempotency + the X-Internal-Token gate."""

from __future__ import annotations

import uuid
from types import SimpleNamespace

import pytest
from fastapi.testclient import TestClient

from app.deps import internal_gate
from app.errors import AppError, ErrorCode
from tests._support import FakePricingRepository, make_settings


def _accrual_body(**over: object) -> dict[str, object]:
    body: dict[str, object] = {
        "merchant_id": str(uuid.uuid4()),
        "amount": 2_500,
        "currency": "KES",
        "source_ref": "pay-001",
        "occurred_at": "2026-05-15T12:00:00+00:00",
    }
    body.update(over)
    return body


def test_accrual_happy(client: TestClient, fake_repo: FakePricingRepository) -> None:
    r = client.post("/v1/internal/accruals", json=_accrual_body())
    assert r.status_code == 202
    assert r.json()["accepted"] is True
    assert len(fake_repo.accruals) == 1
    assert fake_repo.accruals[0].period == "2026-05"


def test_accrual_duplicate_source_ref_idempotent(
    client: TestClient, fake_repo: FakePricingRepository
) -> None:
    body = _accrual_body(source_ref="pay-dup")
    r1 = client.post("/v1/internal/accruals", json=body)
    r2 = client.post("/v1/internal/accruals", json=body)
    assert r1.status_code == 202 and r2.status_code == 202
    assert len(fake_repo.accruals) == 1  # second is a no-op


def _gate_request(secret: str | None) -> SimpleNamespace:
    settings = make_settings(internal_shared_secret=secret) if secret else make_settings()
    return SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(settings=settings)))


def test_internal_gate_open_when_no_secret() -> None:
    # No secret configured → trusted network is the control; any (or no) token passes.
    assert internal_gate(_gate_request(None), x_internal_token=None) is None


def test_internal_gate_requires_token_when_secret_set() -> None:
    req = _gate_request("test-internal-token")
    with pytest.raises(AppError) as exc:
        internal_gate(req, x_internal_token="wrong")
    assert exc.value.code == ErrorCode.UNAUTHORIZED
    with pytest.raises(AppError):
        internal_gate(req, x_internal_token=None)
    assert internal_gate(req, x_internal_token="test-internal-token") is None
