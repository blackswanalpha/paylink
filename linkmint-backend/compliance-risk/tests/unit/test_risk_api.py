from __future__ import annotations

import uuid
from collections.abc import AsyncIterator, Iterator

import fakeredis.aioredis
import pytest
from fastapi.testclient import TestClient

from app.deps import get_idempotency, get_services
from app.domain.services import ServiceDeps, Services, build_services
from app.events.stub import NoopPublisher
from app.idempotency import IdempotencyStore
from app.main import create_app
from app.security.provider_crypto import ProviderCipher
from tests._support import FakeRepository, make_settings, noop_commit

EVAL = "/v1/risk/evaluate"


def test_evaluate_allow_clean_tier2(client: TestClient, fake_repo: FakeRepository) -> None:
    uid = str(uuid.uuid4())
    fake_repo.seed_kyc(uuid.UUID(uid), tier=2)
    resp = client.post(
        EVAL,
        json={"user_id": uid, "action": "paylink.create", "amount": 1000, "currency": "KES"},
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["decision"] == "allow"
    assert body["score"] == 0.0
    assert body["reasons"] == []


def test_evaluate_block_tier0(client: TestClient, fake_repo: FakeRepository) -> None:
    uid = str(uuid.uuid4())  # no kyc record → tier 0
    resp = client.post(
        EVAL, json={"user_id": uid, "action": "paylink.create", "amount": 500, "currency": "KES"}
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["decision"] == "block"
    assert any(r["code"] == "LOW_KYC" for r in body["reasons"])
    # a block writes a risk_score + a block flag + check.failed/flag.raised events
    assert len(fake_repo.risk_scores) == 1
    assert len(fake_repo.flags) == 1 and fake_repo.flags[0].severity == "block"


def test_evaluate_review_amount_over_ceiling(client: TestClient, fake_repo: FakeRepository) -> None:
    uid = str(uuid.uuid4())
    fake_repo.seed_kyc(uuid.UUID(uid), tier=1)
    resp = client.post(
        EVAL,
        json={"user_id": uid, "action": "paylink.create", "amount": 60000, "currency": "KES"},
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["decision"] == "review"
    assert any(r["code"] == "AMOUNT_OVER_TIER_CEILING" for r in body["reasons"])
    assert fake_repo.flags[0].severity == "warn"


def test_evaluate_response_shape_is_fixed_contract(
    client: TestClient, fake_repo: FakeRepository
) -> None:
    uid = str(uuid.uuid4())
    fake_repo.seed_kyc(uuid.UUID(uid), tier=2)
    body = client.post(
        EVAL, json={"user_id": uid, "action": "paylink.create", "amount": 1, "currency": "KES"}
    ).json()
    assert set(body.keys()) == {"decision", "score", "reasons"}


def test_evaluate_bad_user_id_400(client: TestClient) -> None:
    resp = client.post(EVAL, json={"user_id": "not-a-uuid", "action": "paylink.create"})
    assert resp.status_code == 400
    assert resp.json()["error"]["code"] == "INVALID_PAYLOAD"


def test_evaluate_missing_action_422_envelope(client: TestClient) -> None:
    resp = client.post(EVAL, json={"user_id": str(uuid.uuid4())})
    assert resp.status_code == 400  # RequestValidationError → INVALID_PAYLOAD envelope
    assert resp.json()["error"]["code"] == "INVALID_PAYLOAD"


def test_evaluate_negative_amount_rejected(client: TestClient) -> None:
    # Hardening: a negative amount must not be able to defeat the LOW_KYC / AML hard rules — the
    # schema (amount ge=0) rejects it before it reaches the engine.
    resp = client.post(
        EVAL, json={"user_id": str(uuid.uuid4()), "action": "paylink.create", "amount": -5}
    )
    assert resp.status_code == 400
    assert resp.json()["error"]["code"] == "INVALID_PAYLOAD"


def test_evaluate_no_jwt_required(client: TestClient, fake_repo: FakeRepository) -> None:
    # The internal surface takes NO Authorization header (trusted network).
    uid = str(uuid.uuid4())
    fake_repo.seed_kyc(uuid.UUID(uid), tier=2)
    resp = client.post(EVAL, json={"user_id": uid, "action": "paylink.create", "amount": 1})
    assert resp.status_code == 200


# ── internal gate (shared-secret) ──
@pytest.fixture
def gated_client() -> Iterator[tuple[TestClient, FakeRepository]]:
    settings = make_settings(internal_shared_secret="s3cr3t-internal")
    repo = FakeRepository()
    app = create_app(settings)
    cipher = ProviderCipher.from_settings(settings)
    idem = IdempotencyStore(fakeredis.aioredis.FakeRedis(decode_responses=True), 3600)
    with TestClient(app) as test_client:

        async def _services_override() -> AsyncIterator[Services]:
            deps = ServiceDeps(
                repo=repo,  # type: ignore[arg-type]
                commit=noop_commit,
                settings=settings,
                publisher=NoopPublisher(),
                cipher=cipher,
            )
            yield build_services(deps)

        app.dependency_overrides[get_services] = _services_override
        app.dependency_overrides[get_idempotency] = lambda: idem
        yield test_client, repo
    app.dependency_overrides.clear()


def test_gate_rejects_without_token(gated_client: tuple[TestClient, FakeRepository]) -> None:
    test_client, _ = gated_client
    resp = test_client.post(EVAL, json={"user_id": str(uuid.uuid4()), "action": "paylink.create"})
    assert resp.status_code == 401
    assert resp.json()["error"]["code"] == "UNAUTHORIZED"


def test_gate_rejects_wrong_token(gated_client: tuple[TestClient, FakeRepository]) -> None:
    test_client, _ = gated_client
    resp = test_client.post(
        EVAL,
        json={"user_id": str(uuid.uuid4()), "action": "paylink.create"},
        headers={"X-Internal-Token": "wrong"},
    )
    assert resp.status_code == 401


def test_gate_allows_with_correct_token(gated_client: tuple[TestClient, FakeRepository]) -> None:
    test_client, repo = gated_client
    uid = str(uuid.uuid4())
    repo.seed_kyc(uuid.UUID(uid), tier=2)
    resp = test_client.post(
        EVAL,
        json={"user_id": uid, "action": "paylink.create", "amount": 10, "currency": "KES"},
        headers={"X-Internal-Token": "s3cr3t-internal"},
    )
    assert resp.status_code == 200
    assert resp.json()["decision"] == "allow"
