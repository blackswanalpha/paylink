"""Parse the lVM ``PayLinkResponse`` JSON into a typed value."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any


@dataclass(frozen=True)
class ChainPayLink:
    id: str
    creator: str
    receiver: str
    owner: str
    amount: int
    expiry: int
    status: str  # NONE | CREATED | VERIFIED | FAILED | CANCELLED
    metadata_hash: str
    created_at: int
    vote_count: int

    @classmethod
    def from_rpc(cls, d: dict[str, Any]) -> ChainPayLink:
        return cls(
            id=d["id"],
            creator=d["creator"],
            receiver=d["receiver"],
            owner=d["owner"],
            amount=int(d["amount"]),
            expiry=int(d["expiry"]),
            status=d["status"],
            metadata_hash=d.get("metadataHash", ""),
            created_at=int(d.get("createdAt", 0)),
            vote_count=int(d.get("voteCount", 0)),
        )
