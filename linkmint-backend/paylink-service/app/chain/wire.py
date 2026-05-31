"""Byte-exact wire encoding to match the lVM chain's Go ``encoding/json`` output.

Go's ``json.Marshal`` (used by ``Transaction.SignableBytes`` and the RPC layer) emits compact
JSON and, with the default HTML escaping, encodes ``&``, ``<`` and ``>`` as ``\\u0026`` /
``\\u003c`` / ``\\u003e``. We replicate that so the bytes we sign/hash equal what the chain
recomputes server-side (see ``paylink-chain/internal/types/transaction.go``).
"""

from __future__ import annotations

import hashlib
import json
from typing import Any

ZERO_HASH = "0x" + "00" * 32


def go_json(obj: Any) -> bytes:
    """Serialize like Go's default ``json.Marshal``: compact + HTML-escaped ``& < >``."""
    text = json.dumps(obj, separators=(",", ":"), ensure_ascii=False)
    text = text.replace("&", "\\u0026").replace("<", "\\u003c").replace(">", "\\u003e")
    return text.encode("utf-8")


def sha256_hex(data: bytes) -> str:
    """SHA-256 as a ``0x``-prefixed lowercase hex string (matches ``types.Hash.Hex()``)."""
    return "0x" + hashlib.sha256(data).hexdigest()
