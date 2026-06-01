"""PII redaction at the KYC-callback boundary.

INVARIANT (rules.md Part A — non-custodial / data-minimization): raw PII (legal names, national-id
numbers, document images, dates of birth, addresses) is NEVER persisted, logged, returned in a
response, or placed in an event payload. A KYC provider's callback metadata is passed through
:func:`redact` BEFORE any write/log/emit; only an allowlist of safe scalar keys survives, and only
when the value is a scalar (str/int/float/bool/None) — nested structures are dropped wholesale so a
PII blob can't smuggle through under an allowlisted key.

``compliance.kyc_records.documents`` stores ONLY the output of this function.
"""

from __future__ import annotations

from typing import Any

# Safe, non-PII metadata keys a KYC vendor may return that are useful to retain for audit/debug.
# Everything NOT in this set (names, id numbers, image refs, DOB, address, ...) is dropped.
_ALLOWED_KEYS: frozenset[str] = frozenset(
    {
        "session_id",
        "status",
        "reason_code",
        "decision",
        "document_type",
        "verification_level",
        "completed_at",
        "country",
        "tier",
        "provider",
        "check_id",
    }
)

_SCALAR_TYPES = (str, int, float, bool, type(None))


def redact(meta: dict[str, Any] | None) -> dict[str, Any]:
    """Return a copy of ``meta`` keeping only allowlisted scalar keys; all else is dropped."""
    if not meta:
        return {}
    return {
        key: value
        for key, value in meta.items()
        if key in _ALLOWED_KEYS and isinstance(value, _SCALAR_TYPES)
    }
