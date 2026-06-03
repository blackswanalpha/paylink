"""Opaque password-reset tokens: high-entropy mint + SHA-256 hash-at-rest.

Reset tokens are 256-bit random opaque strings (not JWTs) so they can be invalidated server-side and
single-used. They're stored SHA-256-hashed in ``identity.password_reset_tokens.token_hash`` — a fast
constant-entropy hash is sufficient (unlike low-entropy passwords/keys, which use argon2id), and
lookup-by-hash is itself the constant-time match (exact equality on an indexed column). Mirrors the
refresh-token primitive in :mod:`app.security.refresh_tokens`.
"""

from __future__ import annotations

import hashlib
import hmac
import secrets


def mint_reset_token() -> str:
    return secrets.token_urlsafe(32)


def hash_reset_token(token: str) -> str:
    return hashlib.sha256(token.encode()).hexdigest()


def reset_token_matches(token: str, stored_hash: str) -> bool:
    return hmac.compare_digest(hash_reset_token(token), stored_hash)
