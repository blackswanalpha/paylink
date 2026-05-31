"""Build and sign lVM transactions (TxCreatePayLink=1, TxCancelPayLink=3).

Payload field order mirrors the Go payload structs exactly, and the signable bytes mirror
``Transaction.SignableBytes()`` = ``{type, from, nonce, payload}`` (that order, compact). Only a
``metadataHash`` ever crosses to the chain — never raw metadata (invariant A.1).
"""

from __future__ import annotations

import hashlib
import uuid
from typing import Any

from app.chain import wire
from app.chain.signer import Signer

TX_CREATE_PAYLINK = 1
TX_CANCEL_PAYLINK = 3


def new_pl_id(
    creator: str, receiver: str, amount: int, expiry: int, seed: str | None = None
) -> str:
    """Caller-supplied 32-byte PayLink id. When ``seed`` is the Idempotency-Key, a retried create
    yields the same id, so the chain dedups the re-submission (one proof settles one PayLink)."""
    s = seed or uuid.uuid4().hex
    raw = f"{creator}|{receiver}|{amount}|{expiry}|{s}".encode()
    return "0x" + hashlib.sha256(raw).hexdigest()


def metadata_hash(metadata: dict[str, Any] | None) -> str:
    if not metadata:
        return wire.ZERO_HASH
    return wire.sha256_hex(wire.go_json(metadata))


def _tx_core(tx_type: int, from_addr: str, nonce: int, payload: dict[str, Any]) -> dict[str, Any]:
    # Insertion order is the on-chain SignableBytes order: type, from, nonce, payload.
    return {"type": tx_type, "from": from_addr, "nonce": nonce, "payload": payload}


def build_create(
    *,
    pl_id: str,
    from_addr: str,
    nonce: int,
    receiver: str,
    amount: int,
    expiry: int,
    md_hash: str,
    rules: Any | None = None,
) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "paylinkId": pl_id,
        "receiver": receiver,
        "amount": amount,
        "expiry": expiry,
        "metadataHash": md_hash,
    }
    if rules is not None:  # matches Go `omitempty` on Rules
        payload["rules"] = rules
    return _tx_core(TX_CREATE_PAYLINK, from_addr, nonce, payload)


def build_cancel(*, pl_id: str, from_addr: str, nonce: int) -> dict[str, Any]:
    return _tx_core(TX_CANCEL_PAYLINK, from_addr, nonce, {"paylinkId": pl_id})


def signable_bytes(tx_core: dict[str, Any]) -> bytes:
    ordered = {
        "type": tx_core["type"],
        "from": tx_core["from"],
        "nonce": tx_core["nonce"],
        "payload": tx_core["payload"],
    }
    return wire.go_json(ordered)


def sign_tx(tx_core: dict[str, Any], signer: Signer) -> dict[str, Any]:
    """Return the full wire transaction: ``tx_core`` + base64 signature + ``0x`` hash."""
    sb = signable_bytes(tx_core)
    digest = hashlib.sha256(sb).digest()
    return {
        **tx_core,
        "signature": signer.sign_digest(digest),
        "hash": wire.sha256_hex(sb),
    }
