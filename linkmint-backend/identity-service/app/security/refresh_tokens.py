"""Opaque refresh tokens: high-entropy mint + SHA-256 hash-at-rest.

Refresh tokens are 256-bit random opaque strings (not JWTs) so they can be revoked server-side.
They're stored SHA-256-hashed in ``identity.sessions.refresh_token`` — a fast constant-entropy hash
is sufficient here (unlike low-entropy passwords/keys, which use argon2id) and keeps ``/refresh``
on the cheap hot path.
"""

from __future__ import annotations

import hashlib
import hmac
import secrets


def mint_refresh_token() -> str:
    return secrets.token_urlsafe(32)


def hash_refresh_token(token: str) -> str:
    return hashlib.sha256(token.encode()).hexdigest()


def refresh_token_matches(token: str, stored_hash: str) -> bool:
    return hmac.compare_digest(hash_refresh_token(token), stored_hash)
