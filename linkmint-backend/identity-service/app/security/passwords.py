"""argon2id password hashing (spec: ``password_hash`` is argon2id; null for OAuth-only users)."""

from __future__ import annotations

from argon2 import PasswordHasher
from argon2.exceptions import InvalidHashError, VerificationError, VerifyMismatchError

from app.config import Settings


class PasswordHashing:
    def __init__(self, hasher: PasswordHasher) -> None:
        self._hasher = hasher

    @classmethod
    def from_settings(cls, settings: Settings) -> PasswordHashing:
        return cls(
            PasswordHasher(
                time_cost=settings.argon2_time_cost,
                memory_cost=settings.argon2_memory_cost_kib,
                parallelism=settings.argon2_parallelism,
            )
        )

    def hash(self, password: str) -> str:
        return self._hasher.hash(password)

    def verify(self, hashed: str, password: str) -> bool:
        try:
            self._hasher.verify(hashed, password)
            return True
        except (VerifyMismatchError, VerificationError, InvalidHashError):
            return False

    def needs_rehash(self, hashed: str) -> bool:
        return self._hasher.check_needs_rehash(hashed)
