"""HMAC-SHA256 verification for KYC provider callbacks (constant-time).

The ``/v1/kyc/callbacks/{provider}`` route is NOT JWT-authed — the trust anchor is a per-provider
shared secret. The route reads the raw request body ONCE and verifies the ``X-Signature`` header
(HMAC-SHA256 of the raw bytes, hex) against the provider's secret with a constant-time compare
(``hmac.compare_digest``) so the verification leaks no timing signal. A ``sha256=`` prefix on the
presented signature is tolerated (some vendors prefix it). Concept mirrors the constant-time compare
in ``adapters/mpesa/internal/server/callbacks.go``.
"""

from __future__ import annotations

import hashlib
import hmac


def compute_signature(secret: str, raw: bytes) -> str:
    """HMAC-SHA256 of ``raw`` under ``secret``, lower-case hex."""
    return hmac.new(secret.encode(), raw, hashlib.sha256).hexdigest()


def verify_signature(secret: str, raw: bytes, presented: str | None) -> bool:
    """True iff ``presented`` is a valid HMAC-SHA256 signature of ``raw`` under ``secret``.

    Returns False on an empty secret or a missing/blank presented signature. A leading ``sha256=``
    on ``presented`` is stripped. The compare is constant-time.
    """
    if not secret or not presented:
        return False
    candidate = presented.strip()
    if candidate.lower().startswith("sha256="):
        candidate = candidate[len("sha256=") :]
    expected = compute_signature(secret, raw)
    return hmac.compare_digest(expected, candidate.lower())
