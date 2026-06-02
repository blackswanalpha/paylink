"""PII redaction for logs + responses.

INVARIANT (rules.md Part A — non-custodial / data-minimization): a destination contact (phone /
email) is the one piece of PII this service handles. It is stored in ``notify.deliveries.recipient``
(authorized by §2.7) but is NEVER logged raw and NEVER returned unmasked. :func:`mask_recipient`
masks it in every log line and the GET delivery response; :func:`safe_data_keys`
keeps only scalar event-``data`` keys out of logs so a nested PII blob can't smuggle through.
"""

from __future__ import annotations

from typing import Any

_SCALAR_TYPES = (str, int, float, bool, type(None))


def mask_recipient(channel: str, value: str) -> str:
    """Mask a phone/email so it is safe to log or return.

    Email → first char + ``***@`` + domain (``j***@x.io``). Phone/other → keep a short prefix +
    last 2 chars (``+2547*****78``). Empty/short values are fully masked.
    """
    if not value:
        return ""
    if channel == "email" or "@" in value:
        local, _, domain = value.partition("@")
        head = local[0] if local else ""
        return f"{head}***@{domain}" if domain else f"{head}***"
    # phone / opaque handle
    if len(value) <= 4:
        return "*" * len(value)
    keep_prefix = value[:4]
    keep_suffix = value[-2:]
    return f"{keep_prefix}{'*' * max(1, len(value) - 6)}{keep_suffix}"


def safe_data_keys(data: dict[str, Any] | None) -> dict[str, Any]:
    """Keep only scalar ``data`` keys (drop nested structures) for safe logging."""
    if not data:
        return {}
    return {k: v for k, v in data.items() if isinstance(v, _SCALAR_TYPES)}
