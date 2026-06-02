"""The canonical event envelope — byte-identical to eventbus-go's Envelope.

The wire format is the shared contract between every LinkMint service, regardless of language. The
JSON encoding rules (fixed top-level field order; recursively key-sorted payload; compact; no HTML
escaping; ``occurred_at`` as an RFC3339 UTC ``Z`` string) MUST match ``eventbus-go`` exactly so a
Go-published event is consumable by Python and vice-versa. See ``workload/catalog.md``.
"""

from __future__ import annotations

import json
import uuid
from datetime import UTC, datetime
from typing import Any

from pydantic import BaseModel, ConfigDict, Field

# RFC3339, UTC, seconds precision, literal Z — never a +00:00 offset or fractional seconds.
_OCCURRED_AT_FMT = "%Y-%m-%dT%H:%M:%SZ"


def _now_z() -> str:
    return datetime.now(UTC).strftime(_OCCURRED_AT_FMT)


def _canonical(value: Any) -> Any:
    """Recursively sort mapping keys so a payload's bytes never depend on insertion order."""
    if isinstance(value, dict):
        return {k: _canonical(value[k]) for k in sorted(value)}
    if isinstance(value, (list, tuple)):
        return [_canonical(v) for v in value]
    return value


class Envelope(BaseModel):
    """One domain event on the bus. ``payload`` carries ids/metadata only — never secrets."""

    model_config = ConfigDict(extra="ignore")  # ignore unknown fields on decode (forward-compat)

    id: str
    name: str
    key: str = ""
    correlation_id: str = ""
    occurred_at: str
    source: str
    payload: dict[str, Any] = Field(default_factory=dict)

    @classmethod
    def new(
        cls,
        name: str,
        key: str,
        correlation_id: str,
        source: str,
        payload: dict[str, Any] | None,
    ) -> Envelope:
        """Build an envelope, stamping a fresh UUID id and the current UTC time."""
        return cls(
            id=str(uuid.uuid4()),
            name=name,
            key=key,
            correlation_id=correlation_id,
            occurred_at=_now_z(),
            source=source,
            payload=payload or {},
        )

    def to_canonical_bytes(self) -> bytes:
        """Encode to canonical JSON bytes — byte-identical to eventbus-go's Envelope.Marshal."""
        ordered = {
            "id": self.id,
            "name": self.name,
            "key": self.key,
            "correlation_id": self.correlation_id,
            "occurred_at": self.occurred_at,
            "source": self.source,
            "payload": _canonical(self.payload),
        }
        return json.dumps(ordered, ensure_ascii=False, separators=(",", ":")).encode("utf-8")

    @classmethod
    def from_bytes(cls, raw: bytes) -> Envelope:
        """Decode envelope bytes; unknown fields are ignored."""
        return cls.model_validate_json(raw)
