from __future__ import annotations

from datetime import UTC, datetime

import pytest

from app.chain.mapping import ChainPayLink
from app.domain.models import OffChainStatus
from app.domain.reconcile import reconcile_status

NOW = datetime(2026, 5, 29, tzinfo=UTC)
FAR_FUTURE = int(datetime(2100, 1, 1, tzinfo=UTC).timestamp())
PAST = int(datetime(2020, 1, 1, tzinfo=UTC).timestamp())


def _chain(status: str, expiry: int = FAR_FUTURE) -> ChainPayLink:
    return ChainPayLink(
        id="0x1",
        creator="0xc",
        receiver="0xr",
        owner="0xo",
        amount=1,
        expiry=expiry,
        status=status,
        metadata_hash="0x0",
        created_at=0,
        vote_count=0,
    )


@pytest.mark.parametrize(
    "terminal", [OffChainStatus.VERIFIED, OffChainStatus.FAILED, OffChainStatus.CANCELLED]
)
def test_terminal_local_is_authoritative(terminal: OffChainStatus) -> None:
    assert reconcile_status(terminal, _chain("CREATED"), now=NOW) == terminal


def test_no_chain_record_keeps_local() -> None:
    assert reconcile_status(OffChainStatus.PENDING, None, now=NOW) == OffChainStatus.PENDING


def test_chain_created_maps_to_pending() -> None:
    assert (
        reconcile_status(OffChainStatus.PENDING, _chain("CREATED"), now=NOW)
        == OffChainStatus.PENDING
    )


@pytest.mark.parametrize(
    ("chain_status", "expected"),
    [
        ("VERIFIED", OffChainStatus.VERIFIED),
        ("FAILED", OffChainStatus.FAILED),
        ("CANCELLED", OffChainStatus.CANCELLED),
    ],
)
def test_chain_settlement_is_reflected(chain_status: str, expected: OffChainStatus) -> None:
    assert reconcile_status(OffChainStatus.PENDING, _chain(chain_status), now=NOW) == expected


def test_pending_past_expiry_becomes_expired() -> None:
    assert (
        reconcile_status(OffChainStatus.PENDING, _chain("CREATED", expiry=PAST), now=NOW)
        == OffChainStatus.EXPIRED
    )
