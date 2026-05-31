"""Scoped API keys: generate (shown once), argon2id hash-at-rest, verify.

The full ``lm_live_<secret>`` key is returned to the caller exactly once at issuance and never
persisted or logged. We store its argon2id ``hash`` plus a non-secret ``prefix`` fragment used to
narrow candidate rows during verification (argon2 is salted, so you can't look up by hash).
"""

from __future__ import annotations

import secrets
from dataclasses import dataclass

from app.security.passwords import PasswordHashing

PREFIX = "lm_live_"
_PREFIX_FRAGMENT_LEN = 8


@dataclass(frozen=True)
class GeneratedApiKey:
    full_key: str  # shown once: lm_live_<secret>
    prefix: str  # stored + displayed: lm_live_<first 8 of secret>
    hash: str  # argon2id of full_key


def generate_api_key(hasher: PasswordHashing) -> GeneratedApiKey:
    secret = secrets.token_urlsafe(32)
    full = f"{PREFIX}{secret}"
    prefix = f"{PREFIX}{secret[:_PREFIX_FRAGMENT_LEN]}"
    return GeneratedApiKey(full_key=full, prefix=prefix, hash=hasher.hash(full))


def prefix_of(full_key: str) -> str:
    """Recover the stored prefix fragment from a presented full key (for candidate lookup)."""
    secret = full_key[len(PREFIX) :] if full_key.startswith(PREFIX) else ""
    return f"{PREFIX}{secret[:_PREFIX_FRAGMENT_LEN]}"


def verify_api_key(hasher: PasswordHashing, full_key: str, stored_hash: str) -> bool:
    return hasher.verify(stored_hash, full_key)
